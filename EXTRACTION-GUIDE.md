# 3D Viewer Asset Extraction Guide

This guide covers exactly which files the 3D viewer tries to load, where they
must live on disk, and how to extract / copy them from your Borderlands 2
install with UModel. If a path is referenced by a `const` in `static/viewer3d.js`,
it appears here.

The viewer only loads real assets when they exist. Anything missing silently
falls back to a procedural placeholder — there are no errors for absent files.

## Fast path: one script

There is a one-shot shell script that runs UModel and lays every file out in
exactly the tree `viewer3d.js` expects:

```bash
bash extract-assets.sh
```

It handles UMODEL detection (`UMODEL` env, `umodel` next to the script, or on PATH), prompts for your
BL2 `WillowGame/Content/` path (or `BL2_CONTENT`), and produces a pass/fail
report for each viewer asset class. It does **not** automate everything — heads
are too many to script sensibly, `body_mask.png` is not in any `.upk`, and the
JSON sidecars are Python-parser output — but it does the extraction + movement
for bodies and the Startup texture copies, and it checks the rest.

The rest of this document is the manual reference: what each step does, where
each file comes from, and what to do when the script reports something missing.


## What the viewer actually loads

`viewer3d.js` references these static assets. Each one is a real HTTP GET the
browser performs at runtime.

### Character bodies (6 glTF meshes)

From `CHARACTER_MODELS`:

| Character | Path the viewer requests | Source mesh (inside a `GD_*_Streaming_SF` package) |
|---|---|---|
| Axton | `/static/models/characters/Axton/Skel_SoldierBody.gltf` (+ `.bin`) | `Skel_SoldierBody` in `GD_Soldier_Streaming_SF` |
| Zer0 | `/static/models/characters/Zer0/Skel_AssassinBody.gltf` (+ `.bin`) | `Skel_AssassinBody` in `GD_Assassin_Streaming_SF` |
| Maya | `/static/models/characters/Maya/Skel_SirenBody.gltf` (+ `.bin`) | `Skel_SirenBody` in `GD_Siren_Streaming_SF` |
| Salvador | `/static/models/characters/Salvador/Char_MercBody.gltf` (+ `.bin`) | `Char_MercBody` in `GD_Mercenary_Streaming_SF` |
| Gaige | `/static/models/characters/Gaige/Skel_MechromancerBody.gltf` (+ `.bin`) | `Skel_MechromancerBody` in `GD_Mechromancer_Streaming_SF` |
| Krieg | `/static/models/characters/Krieg/Skel_PsychoBody.gltf` (+ `.bin`) | `Skel_PsychoBody` in `GD_Psycho_Streaming_SF` |

UModel: open each `GD_*_Streaming_SF.upk`, go to `SkeletalMesh3`, export the
named mesh as glTF (UModel's glTF exporter, not PSK/FBX). Copy the resulting
`.gltf` + `.bin` into the matching `static/models/characters/<Name>/` folder.

The `.bin` is required — it holds the geometry buffer. The `.gltf` is a small
JSON that points at it. Do not drop the `.bin`.

UModel may also export a `materials/` and `textures/` folder alongside each
glTF when material/texture export is enabled. The body viewer ignores these
(the body is recolored from `skin_colors.json` — see below). They are optional
and harmless to leave in place.

### Body zone masks (6 PNG images)

From `BODY_MASK_PATHS`:

| Character | Path the viewer requests |
|---|---|
| Axton | `/static/models/characters/Axton/body_mask.png` |
| Zer0 | `/static/models/characters/Zer0/body_mask.png` |
| Maya | `/static/models/characters/Maya/body_mask.png` |
| Salvador | `/static/models/characters/Salvador/body_mask.png` |
| Gaige | `/static/models/characters/Gaige/body_mask.png` |
| Krieg | `/static/models/characters/Krieg/body_mask.png` |

These are grayscale alpha masks where R = zone A, G = zone B, B = zone C — the
three skin zones the body recolor blends. They are **not stored in any `.upk`**.
They are standalone images you supply.

If a mask is missing the viewer falls back to a luminance-based recolor of the
body's existing glTF texture. The body still shows a color, just with less
accurate zone placement. The mask is optional.

Where to get them: extract from the game's existing body texture composites, or
paint your own. The mask must be the same pixel dimensions as the body mesh's
UV space (UModel exports the body glTF with its UVs intact, so match that).
When in doubt, a solid white `body_mask.png` makes every vertex zone A.

### Character heads (per-save, from `head_models.json`)

The save's `head_asset` field names an in-game asset path like
`GD_Assassin_Items_MainGame.Assassin.Head_Zero002`. At runtime the viewer:

1. Looks it up in `/static/head_models.json` → gets a glTF URL.
2. Loads that glTF.
3. Applies the MIC material from `/static/head_materials.json` for that same
   asset path.

So two JSON files plus the head glTFs and textures are needed. The head glTFs
and textures live under `static/models/heads/<UPK_name>/<UPK_name>_SF/` — see
the "Head packages" section below for the full extraction list.

### Weapon meshes (6 glTFs)

From `WEAPON_MODELS`:

| Weapon type | Path the viewer requests |
|---|---|
| Pistol | `/static/models/weapons/Pistol/model.gltf` (+ `.bin`) |
| Assault Rifle | `/static/models/weapons/Assault_Rifle/model.gltf` (+ `.bin`) |
| SMG | `/static/models/weapons/SMG/model.gltf` (+ `.bin`) |
| Shotgun | `/static/models/weapons/Shotgun/model.gltf` (+ `.bin`) |
| Sniper Rifle | `/static/models/weapons/Sniper_Rifle/model.gltf` (+ `.bin`) |
| Rocket Launcher | `/static/models/weapons/Rocket_Launcher/model.gltf` (+ `.bin`) |

These come from `split_gestalt.py` (which reads PSKs from Startup.upk and
writes per-manufacturer glTFs under `static/models/weapons/<Type>/model.gltf`),
or you can hand-place a pre-split glTF at that exact path. If a weapon type's
folder is missing the viewer shows a placeholder.

### Weapon textures (per-weapon-type PNGs)

From `WEAPON_TEXTURES`. These are the base color + normal + composite mask the
weapon material uses **before** the per-rarity MIC overrides are applied. They
live under `static/textures/weapons/`:

| Weapon type | comp (mask) | nrm (normal) | dif (base composite) |
|---|---|---|---|
| Pistol | `Weap_Pistols_Comp.png` | `Weap_Pistols_Nrm.png` | `Weap_LauncherShotgunPistol_Comp.png` |
| Assault Rifle | `Weap_AssaultRifle_Comp.png` | `Weap_AssaultRifle_Nrm.png` | `Weap_AssaultSubSniper_Comp.png` |
| SMG | `Weap_SMG_Comp.png` | `Weap_SMG_Nrm.png` | `Weap_AssaultSubSniper_Comp.png` |
| Shotgun | `Weap_Shotgun_Comp.png` | `Weap_Shotgun_Nor.png` | `Weap_LauncherShotgunPistol_Comp.png` |
| Sniper Rifle | `Weap_SniperRifle_Comp.png` | `Weap_SniperRifle_Nrm.png` | `Weap_AssaultSubSniper_Comp.png` |
| Rocket Launcher | `Weap_Launchers_Comp.png` | `Weap_Launchers_Nrm.png` | `Weap_LauncherShotgunPistol_Comp.png` |

These come from the Startup.upk `Texture2D/` dump (which you already extract
for `parse_weapon_mics.py`), but they must be copied / symlinked into
`static/textures/weapons/`. The MIC parser reads the TGAs from
`static/textures/Startup/Texture2D/`; the viewer reads the PNGs from
`static/textures/weapons/`. Same source files, two destination folders.

If weapon textures are missing the weapon still renders with the MIC's per-rarity
color vectors and PBR metalness/roughness — it just lacks the base fabric/pattern
detail.

### Item meshes (3 glTFs)

From `ITEM_MODELS`:

| Item type | Path the viewer requests |
|---|---|
| Shield | `/static/models/items/Shield/model.gltf` (+ `.bin`) |
| Grenade Mod | `/static/models/items/Grenade_Mod/model.gltf` (+ `.bin`) |
| Relic | `/static/models/items/Relic/model.gltf` (+ `.bin`) |
| Class Mod | `null` — not loaded (no class mod mesh viewer) |

These come from the `GestaltDef_Shields_GestaltSkeletalMesh`, `GestaltDef_Grenades_GestaltSkeletalMesh`,
and `GestaltDef_Artifact_GestaltSkeletalMesh` PSKs in Startup.upk's
`SkeletalMesh3/` — same PSK source as the weapons. Either run
`split_gestalt.py` (which handles shields/grenades/artifacts too if their PSKs
are present) or place glTFs manually at those paths.

### Item textures (3 PNG sets)

From `ITEM_TEXTURES`:

| Item type | dif | nrm |
|---|---|---|
| Shield | `static/textures/items/Shield_Dif.png` | `static/textures/items/Shield_Nrm.png` |
| Grenade Mod | `static/textures/items/Grenades_Dif.png` | `static/textures/items/Grenades_Nrm.png` |
| Relic | `static/textures/items/ItemArtifacts_Comp.png` | (none) |

Source: Startup.upk `Texture2D/`. Copy/symlink into `static/textures/items/`.
The source TGAs/PNGs for these are already in your
`static/textures/Startup/Texture2D/` dump (e.g. `Shield_Dif.tga`,
`Grenades_Dif.tga`, `ItemArtifacts_Comp.tga`).

### Class mod meshes (6 glTFs)

From `CLASSMOD_MODELS`:

| Character | Path the viewer requests | Source PSK in Startup.upk |
|---|---|---|
| Axton | `/static/models/items/classmods/ClassMod_Soldier/model.gltf` | `SoldierClassMod01` |
| Zer0 | `/static/models/items/classmods/ClassMod_Assassin/model.gltf` | `ZeroClassMod01` |
| Maya | `/static/models/items/classmods/ClassMod_Siren/model.gltf` | `SirenClassmod01` |
| Salvador | `/static/models/items/classmods/ClassMod_Merc/model.gltf` | `MercClassmod01` |
| Gaige | `/static/models/items/classmods/ClassMod_Merc/model.gltf` | `MercClassmod01` (shared with Salvador) |
| Krieg | `/static/models/items/classmods/ClassMod_Merc/model.gltf` | `MercClassmod01` (shared with Salvador) |

These come from the `SoldierClassMod01`, `ZeroClassMod01`, `SirenClassmod01`,
`MercClassmod01` PSKs in Startup.upk's `SkeletalMesh3/`. Note Gaige and Krieg
share Salvador's class mod mesh in-game — that's why the table points all three
at `ClassMod_Merc`.

### Class mod textures (6 PNG sets)

From `CLASSMOD_TEXTURES`:

| Character | dif | nrm |
|---|---|---|
| Axton | `static/textures/items/SoldierClassMod01_Diff.png` | `static/textures/items/SoldierClassMod01_Norm.png` |
| Zer0 | `static/textures/items/AssassinClassMod01_Dif.png` | `static/textures/items/AssassinClassMod01_Nrm.png` |
| Maya | `static/textures/items/SirenClassMod01_Dif.png` | `static/textures/items/SirenClassMod01_Nrm.png` |
| Salvador | `static/textures/items/MercClassMod01_Dif.png` | `static/textures/items/MercClassMod01_Nrm.png` |
| Gaige | `static/textures/items/MercClassMod01_Dif.png` | `static/textures/items/MercClassMod01_Nrm.png` |
| Krieg | `static/textures/items/MercClassMod01_Dif.png` | `static/textures/items/MercClassMod01_Nrm.png` |

Source: Startup.upk `Texture2D/`. Copy/symlink into `static/textures/items/`.

### JSON sidecars (3 + 1)

These are not extracted from `.upk`s — they're produced by the Python parsers
or shipped in the repo:

| File | Produced by / source | What it must contain |
|---|---|---|
| `/static/head_models.json` | UModel export layout (manual mapping, or a discovery script) | In-game head asset path → glTF URL, e.g. `"GD_Assassin_Items_MainGame.Assassin.Head_Zero002": "/static/models/heads/CD_Assassin_Head_Zero002/.../Skel_Zero002.gltf"` |
| `/static/head_materials.json` | `python tools/parse_head_mics.py` | Same key → MIC material (textures + scalars + vectors) |
| `/static/weapon_materials.json` | `python tools/parse_weapon_mics.py` | `[manufacturer][rarity]` → MIC material |
| `/static/skin_colors.json` | Shipped static file — add entries here for new skins | Skin asset path → `{ name, primary, secondary, tertiary, zones: { a, b, c } }` |

`skin_colors.json` is the one file you edit by hand when you want the viewer to
know about a skin the repo doesn't ship yet. No `.upk` extraction needed — just
the three zone RGB vectors (`a`, `b`, `c`) for that skin.

### The `CD_*_Skin_*` packages — not needed

The `CD_*_Skin_*_SF` packages (one per skin per class, already extracted into
`static/models/`) are **not loaded by the viewer**. The skin system works
entirely from `skin_colors.json` zone vectors + the body glTF's own texture +
the optional `body_mask.png`. The `CD_*_Skin_*` `.upk`s hold the in-game skin's
diffuse/mask/emissive textures, but the viewer doesn't read them.

Do not extract `CD_*_Skin_*` packages for the 3D viewer. They are only useful
if you are doing skin texture modding outside this viewer.


## Complete extraction checklist

### Prerequisitess

- [ ] UModel installed (https://www.gildor.org/en/projects/umodel)
- [ ] Borderlands 2 PC install located (the `WillowGame/Content/*.upk` files)
- [ ] `Pillow` installed in the editor's Python env (`pip install Pillow`) —
    needed by the MIC parsers for TGA→PNG conversion
- [ ] Working directory = repo root (the `static/` tree is relative to it)

### Step 1: Extract Startup.upk (textures + weapon/items PSKs)

UModel → open `Startup.upk` from `<BL2 install>/WillowGame/Content/`.

Export these folders (UModel's "Export" / tree-select):

- `MaterialInstanceConstant/` → `static/textures/Startup/MaterialInstanceConstant/`
  - This gets you `MasterMati_*.props.txt` (one per manufacturer × rarity) —
    the source the weapon MIC parser reads.
- `Texture2D/` → `static/textures/Startup/Texture2D/`
  - This gets you all the `.tga` (and possibly `.png`) texture files. The MIC
    parser converts the ones it needs to PNG; the viewer loads PNGs from
    `static/textures/weapons/` and `static/textures/items/` (see copy steps
    below).
- `SkeletalMesh3/` → optionally `static/models/Startup/SkeletalMesh3/` (or any
    temp dir) **if** you want to run `split_gestalt.py`.
  - Needed PSKs for the currently wired-up viewer: the 6 weapon gestalts, plus
    `GestaltDef_Shields_GestaltSkeletalMesh.psk`, `GestaltDef_Grenades_GestaltSkeletalMesh.psk`,
    `GestaltDef_Artifact_GestaltSkeletalMesh.psk`, `SoldierClassMod01.psk`,
    `ZeroClassMod01.psk`, `SirenClassmod01.psk`, `MercClassmod01.psk`.
  - If you skip this, the viewer can't show weapon / item / class mod meshes
    (bodies and heads still work).

### Step 2: Extract the six `GD_*_Streaming_SF.upk` body packages

UModel → open each of these (from `<BL2 install>/WillowGame/Content/`):

- `GD_Soldier_Streaming_SF.upk` → Axton body
- `GD_Assassin_Streaming_SF.upk` → Zer0 body
- `GD_Siren_Streaming_SF.upk` → Maya body
- `GD_Mercenary_Streaming_SF.upk` → Salvador body
- `GD_Mechromancer_Streaming_SF.upk` (or the actual Gaige package name) → Gaige body
- `GD_Psycho_Streaming_SF.upk` (or the actual Krieg package name) → Krieg body

For each, export `SkeletalMesh3/`. Inside you'll find the body mesh (see the
"Character bodies" table above for the exact mesh names). Copy each
`.gltf` + `.bin` into the matching `static/models/characters/<Name>/` folder.

Example for Axton:

```bash
mkdir -p static/models/characters/Axton
cp GD_Soldier_Streaming_SF/SkeletalMesh3/Skel_SoldierBody.gltf \
   GD_Soldier_Streaming_SF/SkeletalMesh3/Skel_SoldierBody.bin \
   static/models/characters/Axton/
```

Repeat for Zer0, Maya, Salvador, Gaige, Krieg.

(If you prefer not to copy, you can instead edit `CHARACTER_MODELS` in
`viewer3d.js` to point at the `GD_*_Streaming_SF/SkeletalMesh3/` paths
directly. Option A (copy) is what the README assumes; Option B is a one-time
code edit.)

### Step 3: Add `body_mask.png` per character (optional)

For each character folder you created in Step 2, optionally add a
`body_mask.png`. This is a grayscale image, same pixel size as the body mesh's
UV space. R channel = zone A, G = zone B, B = zone C.

If you don't have a real mask, create a 256×256 (or matching-size) solid white
PNG — that puts the whole body in zone A and the recolor still works.

```bash
# Example: solid white mask, replaces with real one when available
convert -size 512x512 xc:white static/models/characters/Axton/body_mask.png
```

(TMask dimensions should match the body glTF's UV space — check the glTF's
texture bounds if you need an exact match. A white fallback is always safe.)

### Step 4: Extract every head `.upk` listed in `head_mapping.json`

The file `static/head_mapping.json` maps each in-game head asset path to its
UModel package name (e.g. `CD_Assassin_Head_Zero002`). These are the `.upk`s
you need to extract for heads:

UModel → open each `CD_*_Head_*_SF.upk` named in `head_mapping.json` (from
`<BL2 install>/WillowGame/Content/`).

For each, export the full package tree into
`static/models/heads/<UPK_name>/` — UModel should create the `<UPK_name>_SF/`
subfolder with `SkeletalMesh3/`, `MaterialInstanceConstant/`, and
`Texture2D/` inside.

Example for one head:

```bash
# UModel exports CD_Assassin_Head_Zero002 into a folder it creates; move it:
mv UModelExport/CD_Assassin_Head_Zero002 \
   static/models/heads/CD_Assassin_Head_Zero002
```

After this step you should have, for each head:

```
static/models/heads/CD_Assassin_Head_Zero002/
    CD_Assassin_Head_Zero002_SF/
        SkeletalMesh3/Skel_Zero002.gltf  (+ .bin)
        MaterialInstanceConstant/Mati_*.props.txt
        Texture2D/*.tga   (and/or .png)
```

(Note: the `_SF` suffix and the exact inner structure come from UModel's export
layout for `CD_*_Head_*_SF.upk` packages. Match what UModel produces.)

### Step 5: Build or place weapon / item / class mod glTFs (model.gltf)

The viewer reads one `model.gltf` (+ `.bin`) per weapon type, item type, and
class mod type. Those paths are the ones in `WEAPON_MODELS`, `ITEM_MODELS`,
and `CLASSMOD_MODELS`.

**Option A — run `split_gestalt.py` (recommended if you have the PSKs):**

`split_gestalt.py` reads gestalt PSKs from `BL2_PSK_DIR` (default:
`static/models/Startup/SkeletalMesh3/`). Your Startup extract may already have
that folder under `static/textures/Startup/SkeletalMesh3/` instead — pass the
right path explicitly.

```bash
export BL2_PSK_DIR="$PWD/static/textures/Startup/SkeletalMesh3"  # adjust to where Startup put SkeletalMesh3
export BL2_WEAPON_MODELS_OUT="$PWD/static/models/weapons"
python tools/split_gestalt.py
```

(You can also run the same commands through `extract-assets.sh`'s dry-run companion,
`docs/assets_for_viewer_dryrun.sh`, but the live `split_gestalt.py` is what actually
produces the glTFs.)

`split_gestalt.py` writes per-manufacturer glTFs under
`static/models/weapons/<Type>/<Manufacturer>/model.gltf`, not the flat
`static/models/weapons/<Type>/model.gltf` the viewer loads. To get the viewer's
expected flat path you either:
- place a symlink/copy at `static/models/weapons/<Type>/model.gltf` pointing at
  one manufacturer's glTF (e.g. `Bandit/model.gltf` if you want the Bandit
  variant as the default), or
- drop the per-manufacturer feature and hand-place a single pre-split
  `model.gltf` at `static/models/weapons/<Type>/model.gltf`.

Either way the viewer only ever requests `WEAPON_MODELS[category]`, which is
exactly `static/models/weapons/<Type>/model.gltf`.

(If you don't have the PSKs, or skip this step, the viewer shows placeholders
for weapons / items / class mods. Bodies and heads still work.)

**Option B — hand-place pre-split glTFs:**

For each viewer path, drop a `model.gltf` + `model.bin` at exactly that path:

```
static/models/weapons/Pistol/model.gltf    (+ model.bin)
static/models/weapons/Assault_Rifle/model.gltf
static/models/weapons/SMG/model.gltf
static/models/weapons/Shotgun/model.gltf
static/models/weapons/Sniper_Rifle/model.gltf
static/models/weapons/Rocket_Launcher/model.gltf
static/models/items/Shield/model.gltf
static/models/items/Grenade_Mod/model.gltf
static/models/items/Relic/model.gltf
static/models/items/classmods/ClassMod_Soldier/model.gltf
static/models/items/classmods/ClassMod_Assassin/model.gltf
static/models/items/classmods/ClassMod_Siren/model.gltf
static/models/items/classmods/ClassMod_Merc/model.gltf   (Salvador/Gaige/Krieg share)
```

### Step 6: Copy weapon + item + class mod textures into the viewer's folders

The viewer reads these PNGs by path. They all originate from
`static/textures/Startup/Texture2D/` (your Step 1 extraction), but must be
reachable at the viewer's expected paths.

**Weapon textures** → `static/textures/weapons/`:

```bash
mkdir -p static/textures/weapons
cd static/textures/Startup/Texture2D

for f in \
    Weap_Pistols_Comp Weap_Pistols_Nrm Weap_LauncherShotgunPistol_Comp \
    Weap_AssaultRifle_Comp Weap_AssaultRifle_Nrm Weap_AssaultSubSniper_Comp \
    Weap_SMG_Comp Weap_SMG_Nrm \
    Weap_Shotgun_Comp Weap_Shotgun_Nor \
    Weap_SniperRifle_Comp Weap_SniperRifle_Nrm \
    Weap_Launchers_Comp Weap_Launchers_Nrm; do
    cp "${f}.png" ../weapons/   # or .tga → convert to png first
done
cd ../../..
```

(If your Startup extract only has `.tga`, convert: `convert ${f}.tga
${f}.png` with ImageMagick, or let the MIC parser do it on first run — but the
viewer needs PNG at the `static/textures/weapons/` path, so do the conversion
here.)

**Item textures** → `static/textures/items/`:

```bash
mkdir -p static/textures/items
cd static/textures/Startup/Texture2D

for f in \
    Shield_Dif Shield_Nrm \
    Grenades_Dif Grenades_Nrm \
    ItemArtifacts_Comp \
    SoldierClassMod01_Diff SoldierClassMod01_Norm \
    AssassinClassMod01_Dif AssassinClassMod01_Nrm \
    SirenClassMod01_Dif SirenClassMod01_Nrm \
    MercClassMod01_Dif MercClassMod01_Nrm; do
    cp "${f}.png" ../items/   # or .tga → convert first
done
cd ../../..
```

**Glossy/reflect/PDF textures** — the head MIC parser may reference shared
textures like `GlossyA.png` from the Startup `Texture2D/`. These are already in
your `static/textures/Startup/Texture2D/` dump (both `.tga` and `.png` variants
exist there). The parser resolves them automatically; no extra copy needed.

### Step 7: Run the MIC parsers

```bash
python tools/parse_head_mics.py
python tools/parse_weapon_mics.py
```

This produces `static/head_materials.json` and `static/weapon_materials.json`.
Both are idempotent — re-run only when source files change.

`parse_head_mics.py` walks `static/models/heads/` and parses every
`MaterialInstanceConstant/*.props.txt` it finds. If a head folder is missing its
MIC (no `.props.txt`), that head renders without MIC material (falls back to the
body tint).

`parse_weapon_mics.py` reads `MasterMati_*.props.txt` from
`static/textures/Startup/MaterialInstanceConstant/` and resolves textures from
`static/textures/Startup/Texture2D/`. If a manufacturer × rarity MIC is missing
that rarity still renders (next-higher-rarity fallback in the viewer).

### Step 8: Verify the layout

Before launching the Flask app, spot-check the paths the viewer actually
requests:

```bash
# Character bodies
ls static/models/characters/Axton/Skel_SoldierBody.gltf
ls static/models/characters/Zer0/Skel_AssassinBody.gltf
ls static/models/characters/Maya/Skel_SirenBody.gltf
ls static/models/characters/Salvador/Char_MercBody.gltf
ls static/models/characters/Gaige/Skel_MechromancerBody.gltf
ls static/models/characters/Krieg/Skel_PsychoBody.gltf

# Body masks (optional)
ls static/models/characters/Axton/body_mask.png

# Head glTFs — at least the ones in head_models.json should exist
ls static/models/heads/CD_Assassin_Head_Zero002/CD_Assassin_Head_Zero002_SF/SkeletalMesh3/Skel_Zero002.gltf

# Weapon glTFs (the flat model.gltf the viewer loads, not per-manufacturer)
ls static/models/weapons/Pistol/model.gltf

# Weapon textures
ls static/textures/weapons/Weap_Pistols_Comp.png

# Item textures
ls static/textures/items/Shield_Dif.png

# JSON sidecars
ls static/head_models.json static/head_materials.json static/weapon_materials.json static/skin_colors.json
```

If any of these are missing, the viewer degrades silently for that asset class.

### Step 9: Launch

```bash
python app.py
```

Open `http://localhost:5000`. Open the browser dev console — the viewer logs
every JSON it loads (`Loaded N head model mappings`, `Loaded N skin color
entries`, `weapon_materials.json parse error` on failure). Missing meshes silence
themselves (placeholder fallback); missing JSON files log a warning.

Use the dev server's network tab to confirm the GETs the browser actually fires
for the paths in this guide — that's the definitive check.

Before launching the Flask app, spot-check the paths the viewer actually
requests:

```bash
# Character bodies
ls static/models/characters/Axton/Skel_SoldierBody.gltf
ls static/models/characters/Zer0/Skel_AssassinBody.gltf
ls static/models/characters/Maya/Skel_SirenBody.gltf
ls static/models/characters/Salvador/Char_MercBody.gltf
ls static/models/characters/Gaige/Skel_MechromancerBody.gltf
ls static/models/characters/Krieg/Skel_PsychoBody.gltf

# Body masks (optional)
ls static/models/characters/Axton/body_mask.png

# Head glTFs — at least the ones in head_models.json should exist
ls static/models/heads/CD_Assassin_Head_Zero002/CD_Assassin_Head_Zero002_SF/SkeletalMesh3/Skel_Zero002.gltf

# Weapon glTFs (the flat model.gltf the viewer loads, not per-manufacturer)
ls static/models/weapons/Pistol/model.gltf

# Weapon textures
ls static/textures/weapons/Weap_Pistols_Comp.png

# Item textures
ls static/textures/items/Shield_Dif.png

# JSON sidecars
ls static/head_models.json static/head_materials.json static/weapon_materials.json static/skin_colors.json
```

### Step 9: Launch

```bash
python app.py
```

Open `http://localhost:5000`. Open the browser dev console — the viewer logs
every JSON it loads (`Loaded N head model mappings`, `Loaded N skin color
entries`, `weapon_materials.json parse error` on failure). Missing meshes silence
themselves (placeholder fallback); missing JSON files log a warning.

Use the dev server's network tab to confirm the GETs the browser actually fires
for the paths in this guide — that's the definitive check.


## What you can skip

These are extracted by some UModel guides but are **not** loaded by this viewer:

- `GD_*_Streaming_SF` packages other than the six body ones (no body MIC parsing
  wired up; bodies use `skin_colors.json` only).
- `CD_*_Skin_*_SF` packages (skin colors come from `skin_colors.json`).
- `WillowGame.upk`, `Engine.upk`, `Core.upk`, `GameFramework.upk`,
  `GearboxFramework.upk`, `GFxUI.upk`, `OnlineSubsystemSteamworks.upk`, etc.
- World / lighting / particle / audio / skybox / grass / level packages.
- Item meshes other than Shield / Grenade Mod / Relic / Class Mod (no viewer
  for grenades, artifacts, artifacts, keys, currency, etc. — they show as cards
  in the inventory, not 3D).
- Weapons other than the six base types (pistol / AR / SMG / shotgun / sniper /
  RL). Add-ons / two-hand variants / elemental variants share the same base mesh.
- `body_mask.png` (optional everywhere — see above).


## Re-extraction when assets change

If you re-run UModel with different export settings (e.g. enable material export,
change glTF version), re-run the parsers:

```bash
python tools/parse_head_mics.py
python tools/parse_weapon_mics.py
```

Then re-check the paths in Step 8. The parsers are idempotent; UModel exports
are not — re-exporting can change file names or folder structure, in which case
update the corresponding `const` in `viewer3d.js` or the JSON sidecars.


## Quick reference card

```
Startup.upk (textures + weapon/item PSKs):
  MaterialInstanceConstant/ → static/textures/Startup/MaterialInstanceConstant/
  Texture2D/               → static/textures/Startup/Texture2D/
  SkeletalMesh3/           → (temp, for split_gestalt.py)

Body packages (6 × GD_*_Streaming_SF.upk):
  SkeletalMesh3/ → copy .gltf+.bin → static/models/characters/<Name>/
  + optional body_mask.png        → static/models/characters/<Name>/

Head packages (every CD_*_Head_*_SF.upk in head_mapping.json):
  full package tree → static/models/heads/<UPK_name>/

Weapon textures (from Startup/Texture2D/):
  → static/textures/weapons/<TexName>.png

Item + class mod textures (from Startup/Texture2D/):
  → static/textures/items/<TexName>.png

Parsers:
  python tools/parse_head_mics.py     → static/head_materials.json
  python tools/parse_weapon_mics.py   → static/weapon_materials.json
  python tools/split_gestalt.py       → static/models/weapons/ + items/ (needs PSKs; writes per-manufacturer glTFs — see Step 5)

Static JSON (shipped or hand-edited):
  static/head_models.json   — head asset path → glTF URL
  static/skin_colors.json   — skin asset path → zone colors (edit for new skins)
```

