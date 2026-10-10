# BL2 Save Editor

A full-stack Borderlands 2 save editor built as a local Flask web application
with a Three.js 3D viewer and a holographic UI themed after the in-game
aesthetic. End-to-end: parse Gearbox's encrypted protobuf save format, edit
characters / inventory / weapons / bank, render real glTF character and weapon
models in-browser, import/export industry-standard Gibbed save codes, and write
back atomically with rotating backups and post-write verification.

![Python](https://img.shields.io/badge/Python-3.8+-blue) ![Flask](https://img.shields.io/badge/Flask-2.0+-green) ![Three.js](https://img.shields.io/badge/Three.js-r128-orange) ![License](https://img.shields.io/badge/license-personal_use-lightgrey)

---

## Quick start

```bash
git clone https://github.com/KillianM00/bl2-save-editor.git
cd bl2-save-editor
python setup_wizard.py
```

The wizard verifies your Python version, sets up a virtual environment,
installs dependencies, auto-detects your Borderlands 2 install + save folder,
clones the `borderlands2-tool` dependency, runs a smoke test, and offers to
launch the editor — all interactively, all idempotent.

**Want zero prompts?** `python setup_wizard.py --yes --launch`

> **Note**: on Windows use `python`, not `python3` — the `python3` command
> on Windows is a Microsoft Store stub that won't run your real interpreter.
> On Debian / Ubuntu and a few other Linux distros where only `python3` is on
> PATH, use `python3 setup_wizard.py` instead.

## Architecture

```
                ┌──────────────────────┐
   Browser ──→  │  Flask app (app.py)  │   ← single-page UI; JSON API
                └──────────┬───────────┘
                           │
       ┌───────────────────┼─────────────────────┐
       ▼                   ▼                     ▼
┌─────────────┐    ┌───────────────┐    ┌──────────────────┐
│  save_io    │    │   asset_db    │    │ steam_achievements│
│  protobuf   │    │   Gibbed-data │    │  Steam API ctypes │
│  parse +    │    │   resolver +  │    │  shim             │
│  write      │    │   stat calcs  │    └──────────────────┘
└──────┬──────┘    └───────────────┘
       │
       ▼
┌────────────────────────────────────────┐
│  apocalyptech/borderlands2 library     │   external pinned dep
│  (encrypted-LZO save format)           │
└────────────────────────────────────────┘
```

- **`app.py`** — Flask routes for save list / character get-set / inventory
  manipulation / Gibbed import-export / exe patch / Steam-API queries.
  Handles browser auto-launch and free-port selection on startup.
- **`save_io.py`** — the load-edit-save pipeline. Decrypts and parses the
  protobuf save, mutates fields, re-serialises, writes a rotating backup, then
  atomically renames the temp file to the real save path. Verifies the write
  by re-reading and comparing SHA-1 + item count.
- **`asset_db.py`** — resolves Gibbed JSON exports into typed objects: weapon
  balance definitions, item-part trees, head/skin asset paths, rarity colors,
  per-part stat modifiers. Includes a live stat estimator that mirrors the
  in-game UI's damage / accuracy / fire-rate calculations.
- **`steam_achievements.py`** — `ctypes` shim over `steam_api64.dll` to query
  unlocked achievements and challenge progress without a separate Steam client
  binding.
- **`patch_bl2.py`** — exe patcher that raises the grade-index cap from 80 to
  127 via a single targeted byte rewrite (with verification + unpatch
  support).
- **`templates/index.html` + `static/app.js` + `static/viewer3d.js`** —
  single-page UI; vanilla JS frontend, no framework. Three.js handles the 3D
  character / weapon / item viewer with per-rarity glTF materials and skin
  color tinting.

## Features

**Character editing.** Level, XP, skill points, OP level. All currencies
(money, eridium, seraph crystals, torgue tokens, golden keys). Inventory /
weapon / bank slot sizes. Name, head, skin, 3-zone RGB color customization.

**Inventory.** Browse weapons, items (shields / grenades / mods / relics), and
bank contents with rarity-colored cards and live stat preview. Delete,
duplicate, or change level. Full weapon editor exposes all 11 part slots with
on-the-fly stat estimation. Add new weapons from the full balance-definition
database.

**Skill tree editor.** Full per-class skill tree visualization driven by
`static/skill_trees.json`. Live counters for points unspent / allocated /
available. Color-coded tree panels matching each character's in-game UI.
Per-skill increment / decrement controls with rank validation against tier
prerequisites. Respec by zeroing all skills and recovering points.

**Gibbed integration.** Import `BL2(...)` codes from forums, Reddit, Discord.
Export individual items or the entire inventory as Gibbed codes. Drop-in
compatibility with the Gibbed save-editor format that the community has used
for a decade.

**3D viewer.** Browser-native Three.js scene with character mesh, skin-color
zone tinting, weapon preview with per-rarity / per-element materials, item
previews for shields / grenades / relics / class mods, and an orbit-controlled
equipment carousel.

**Exe patcher + damage mod.** Raise the grade-index cap from 80 to 127 with a
verifiable single-byte patch. Optional PythonSDK mod adds a configurable
damage multiplier (1-100×) tied to a UI slider.

## Engineering decisions worth highlighting

- **Atomic writes + rotating backups + post-write verification.** Every save
  operation writes to a temp file, atomically renames to the real path,
  rotates up to 5 `.bak` generations, then re-reads the written file and
  asserts the SHA-1 digest + item count match the in-memory state. A
  half-completed write under power loss leaves the previous save intact; a
  corrupted write surfaces an explicit error and the backup is one filename
  away. Covered by a kill-mid-save test in `tests/test_save_io.py`.
- **Game-running detection with cached subprocess.** The editor refuses to
  overwrite saves while `Borderlands2.exe` is running (would race the game's
  in-memory state). Detection uses `tasklist` / `ps` polled per-write but
  cached for 3 seconds so rapid edits don't fork a flurry of subprocesses.
- **Cross-platform path auto-detection.** `config.py` probes Steam install
  paths and save folders for Windows / Linux / macOS without requiring the
  user to write any path manually.
- **glTF asset pipeline.** Custom UModel-export → glTF + PNG sidecar pipeline
  with `tools/parse_head_mics.py` and `tools/parse_weapon_mics.py`
  regenerating the per-head and per-rarity material JSON the viewer consumes. The 3D viewer degrades
  gracefully — meshes without extracted assets fall back to a procedural
  rarity-tinted placeholder rather than erroring out.
- **Single-page no-framework frontend.** Vanilla JS, ~1.5K LOC in `app.js`. No
  build step, no bundler. The whole frontend is hot-reloadable just by
  refreshing the browser.
- **Idempotent setup wizard.** `setup_wizard.py` is safe to re-run — it
  detects existing config, venv, dependencies, and the `borderlands2-tool`
  checkout and fills in only what's missing.

## Skills demonstrated

- **Binary format reverse engineering**: parsing Gearbox's encrypted +
  compressed + protobuf-encoded `.sav` format, mutating in place, re-encoding
  with byte-perfect round-trip.
- **Full-stack web app**: Flask backend with a JSON API, single-page vanilla
  JS frontend, Three.js 3D rendering pipeline.
- **Game-tooling pipeline**: UModel export → glTF transcoding → custom JSON
  sidecar with per-rarity material data, all driven by tools the user can
  re-run when game assets change.
- **Cross-platform OS integration**: auto-detection of Steam installs on
  Windows / Linux / macOS, `ctypes` Steam API binding, exe patching with
  unpatch support.
- **Data safety engineering**: atomic file writes, rotating backups,
  post-write verification, kill-mid-save resilience.
- **Test discipline**: unit tests for the save I/O pipeline and the JSON API
  surface under `tests/`.

## Manual setup (if you'd rather not use the wizard)

1. **Install Python 3.8+** and clone the repo.
2. **Install dependencies:**
   ```bash
   python -m venv .venv
   .venv/Scripts/activate    # or: source .venv/bin/activate
   pip install -r requirements.txt
   ```
3. **Clone the external save-format library:**
   ```bash
   git clone --depth 1 https://github.com/apocalyptech/borderlands2.git
   ```
4. **Configure paths.** Copy `config.example.json` to `config.json` and edit:
   ```json
   {
     "save_dir": "C:\\Users\\YourName\\Documents\\My Games\\Borderlands 2\\WillowGame\\SaveData\\YourSteamID",
     "gibbed_dir": "C:\\path\\to\\gibbed_data",
     "borderlands2_tool_dir": "./borderlands2",
     "game_dir": "C:\\Program Files (x86)\\Steam\\steamapps\\common\\Borderlands 2",
     "backup_generations": 5
   }
   ```
   Empty fields auto-detect.
5. **Run:**
   ```bash
   python app.py
   ```
   The editor opens at `http://localhost:5000` (or the next free port).

## 3D viewer asset extraction (optional)

The viewer works without extracted assets — meshes fall back to procedural
rarity-tinted placeholders. To enable real character and weapon meshes you
need to extract two things from your Borderlands 2 install with
[UModel](https://www.gildor.org/en/projects/umodel):

- **glTF meshes** (SkeletalMesh3 exports) → `static/models/`
- **Texture2D files** (the `.tga` or `.png` images referenced by materials) → `static/textures/`

Then run the Python parsers below to turn UModel's raw export into the JSON
material sidecars the viewer actually loads.

### What the viewer actually shows

The 3D viewer has two character views and one weapon view. Only three asset
classes are wired up today:

1. **Character bodies** — a single hardcoded glTF per class (Axton, Zer0, Maya,
   Salvador, Gaige, Krieg) loaded from `static/viewer3d.js`'s
   `CHARACTER_MODELS`. The body gets a skin-zone recolor from
   `static/skin_colors.json` (three RGB zone colors composited with
   `body_mask.png`); it does **not** read any MIC material.
2. **Character heads** — selected per-save from the save's `head_asset` field,
   resolved through `static/head_models.json` → glTF URL, with per-head MIC
   materials from `static/head_materials.json` (diffuse + mask + normal +
   reflect + optional decal/pattern).
3. **Weapons** — one glTF per weapon type loaded from
   `static/viewer3d.js`'s `WEAPON_MODELS`, with per-manufacturer × per-rarity
   MIC materials from `static/weapon_materials.json`.

Everything else in the game's `.upk` tree (world meshes, particles, UI,
classmods, items other than weapons, bodies other than the six hardcoded
characters, the `CD_*_Skin_*` packages, etc.) is **not** loaded by the viewer
and does not need to be extracted for the 3D viewer to work.

### What UModel must export

UModel works on `.upk` packages. Only two packages are needed for the currently
wired-up viewer:

| Package | What to export from it | Where it lands | Used by |
|---|---|---|---|
| `Startup.upk` | `MaterialInstanceConstant/MasterMati_*.props.txt` (one per manufacturer × rarity, e.g. `MasterMati_DahlCommon.props.txt`) + `Texture2D/*.tga` (the weapon material texture atlas, including shared textures like `GlossyA`, `Pattern_*`, `Logo_*`) + optionally `SkeletalMesh3/*.psk` (gestalt weapon meshes — only if you want `split_gestalt.py` to produce per-manufacturer weapon glTFs) | `static/textures/Startup/MaterialInstanceConstant/` and `static/textures/Startup/Texture2D/` (and optionally `static/models/Startup/SkeletalMesh3/`) | `parse_weapon_mics.py` (and optionally `split_gestalt.py`) |
| Head packages — every `CD_<Class>_Head_<Name>_SF.upk` listed in `static/head_mapping.json` | `SkeletalMesh3/*.gltf` + `.bin` (head mesh) + `MaterialInstanceConstant/*.props.txt` (the head's `Mati_*.props.txt`) + `Texture2D/*.tga` (head-specific diffuse / mask / normal / emissive textures) | `static/models/heads/<UPK_name>/<UPK_name>_SF/SkeletalMesh3/`, `.../MaterialInstanceConstant/`, `.../Texture2D/` | `parse_head_mics.py` |

**The short version:**
1. Export `Startup.upk` → `static/textures/Startup/` (keep the folder structure UModel creates: `MaterialInstanceConstant/`, `Texture2D/`, and optionally `SkeletalMesh3/`).
2. Export every head `.upk` referenced in `static/head_mapping.json` → `static/models/heads/<UPK_name>/`.
3. Export the six character-body glTFs that `CHARACTER_MODELS` in
   `static/viewer3d.js` references. They all live inside `Startup.upk` under
   `SkeletalMesh3/`:
   - Axton:    `Skel_SoldierBody`
   - Zer0:     `Skel_AssassinBody`
   - Maya:     `Skel_SirenBody`
   - Salvador: `Char_MercBody`
   - Gaige:    `Skel_MechromancerBody`
   - Krieg:    `Skel_PsychoBody`
   In UModel: open `Startup.upk`, go to `SkeletalMesh3`, export each as glTF
   (UModel glTF export, not PSK/FBX), and place the `.gltf` + `.bin` into
   `static/models/characters/<Name>/`.
   Optionally export the mesh's material + textures too (UModel does this if
   material/texture export is on); the viewer ignores them for the body because
   skin color comes from `skin_colors.json`, but they don't interfere.

The `CD_*_Skin_*` packages are **not** used by the viewer. The skin system works
entirely from `static/skin_colors.json`, which stores the three zone colors
(a/b/c) per skin asset path — no textures, no MICs, no `.upk` extraction
required. The `CD_*_Skin_*` `.upk`s contain the in-game skin's actual diffuse
+ mask + emissive textures, but the viewer only ever reads the zone-color
numbers from `skin_colors.json`.

### File formats

- **Textures:** UModel exports textures as `.tga`. The parser converts the ones
  it actually needs into `.png` next to the `.tga` (idempotent — it skips
  files that are already newer than the source). The browser only ever loads
  `.png`. You can pre-convert everything with `mogrify -format png` or let the
  parser do it on first run.
- **Meshes:** UModel's glTF export (`.gltf` + `.bin`) is consumed directly by
  Three.js — no conversion step. For weapon gestalt splitting you can instead
  export `.psk` from UModel and let `split_gestalt.py` rebuild per-manufacturer
  glTF files; if you skip that step the viewer uses the six hardcoded
  character body glTFs and any pre-split weapon glTFs already in
  `static/models/weapons/`.
- **Material props (`.props.txt`):** These are UModel's text dump of each
  `MaterialInstanceConstant`'s scalar / vector / texture parameter overrides.
  The parsers read them with a small brace-matching parser — format is
  standard UE3 props output, nothing custom.

### The JSON sidecars

After extraction, three JSON files tell the viewer which texture + parameter
combo belongs to which in-game asset:

| File | Produced by | Maps | Shape |
|---|---|---|---|
| `static/head_models.json` | UModel export layout (or regenerated by a discovery script) | In-game asset path (e.g. `GD_Assassin_Items_MainGame.Assassin.Head_Zero002`) → glTF URL | `{
  "<asset_path>": "/static/models/heads/CD_Assassin_Head_Zero002/.../Skel_Zero002.gltf"
}` |
| `static/head_materials.json` | `python tools/parse_head_mics.py` | Same asset path → MIC material (textures + scalars + vectors) | `{
  "<asset_path>": {
    "textures": { "p_Diffuse": "/static/.../Zero002_Dif.png", ... },
    "scalars": { "p_ShadowsIntensity": 2, ... },
    "vectors": { "p_AColorShadow": [r,g,b], ... }
  }
}` |
| `static/weapon_materials.json` | `python tools/parse_weapon_mics.py` | `[manufacturer][rarity]` → MIC material for the weapon base color/pattern/decal/normal | `{
  "Dahl": {
    "Common": { "textures": {...}, "scalars": {...}, "vectors": {...} },
    ...
  },
  ...
}` |

`head_mapping.json` is the bridge between the save format and the on-disk head
folders: it maps each in-game head asset path (the key used by the save's
`head_asset` field, `head_models.json`, and `head_materials.json`) to the UModel
package name (e.g. `CD_Assassin_Head_Zero002`) that the head parser uses to find
the right folder under `static/models/heads/`.

`skin_colors.json` is **not** produced by a parser — it is a static lookup table
shipped in the repo. Each entry's key is an in-game skin asset path
(e.g. `GD_Assassin_Items_MainGame.Assassin.Skin_BanditA`); the value holds the
skin's display name, primary/secondary/tertiary hex colors, and the three zone
RGB vectors (`a`, `b`, `c`) the body recolor uses. To add a skin the viewer
doesn't know about yet, add an entry here — no `.upk` extraction needed.

`body_mask.png` (one per character, at
`static/models/characters/<Name>/body_mask.png`) is a grayscale alpha mask that
tells the body recolor which vertices belong to skin zone A, B, or C. It is not
in any `.upk` — it's a standalone image you supply. If it's missing the viewer
falls back to a luminance-based recolor of the body's existing glTF texture. The
mask is optional; the viewer still shows a colored body either way.

### Running the parsers

```bash
# 1. Head materials — walks static/models/heads/ and parses every
#    MaterialInstanceConstant/*.props.txt it finds.
python tools/parse_head_mics.py

# 2. Weapon materials — reads MasterMati_*.props.txt from
#    static/textures/Startup/MaterialInstanceConstant/ and resolves textures
#    from static/textures/Startup/Texture2D/. Writes PNGs alongside any TGA it
#    converts.
python tools/parse_weapon_mics.py
```

Both parsers are idempotent: re-running them only does work when source files
are newer than the output JSON or when a `.tga` has no matching `.png` yet.

> **There is also a one-shot shell script** that runs UModel for you and lays
> every file out in exactly the tree `viewer3d.js` expects:
> `extract-assets.sh`. The sections below document the individual steps
> that script performs (useful if you want to do part of it by hand or verify
> what the script did). Run `bash extract-assets.sh` from the repo root.

### Environment variables

| Variable | Default | What it overrides |
|---|---|---|
| `BL2_MATI_EXTRACT` | `static/textures/Startup/` | Root dir for `parse_weapon_mics.py` (must contain `MaterialInstanceConstant/` and `Texture2D/`) |
| `BL2_PSK_DIR` | where UModel put the `Startup/SkeletalMesh3/` folder (your extract may be under `static/textures/Startup/SkeletalMesh3/` — pass that explicitly) | Input dir for `split_gestalt.py` |
| `BL2_WEAPON_MODELS_OUT` | `static/models/weapons/` | Output dir for `split_gestalt.py`'s per-manufacturer glTF files |

### Recap: minimal viable extraction

If you just want the viewer to show real assets with the least work:

1. UModel-export `Startup.upk` into `static/textures/Startup/`.
2. UModel-export every head `.upk` listed in `head_mapping.json` into
   `static/models/heads/`.
3. The six character-body glTFs are already inside the `GD_*_Streaming_SF`
   packages you extracted — each lives in its class package's
   `SkeletalMesh3/` subfolder. You have two options:

   **Option A — copy into the viewer's expected layout (closest to what
   `viewer3d.js` expects today):**
   Copy each body glTF + .bin into `static/models/characters/<Name>/`:
   - Axton:    `GD_Soldier_Streaming_SF/SkeletalMesh3/Skel_SoldierBody.*` → `static/models/characters/Axton/`
   - Zer0:     `GD_Assassin_Streaming_SF/SkeletalMesh3/Skel_AssassinBody.*` → `static/models/characters/Zer0/`
   - Maya:     `GD_Siren_Streaming_SF/SkeletalMesh3/Skel_SirenBody.*` → `static/models/characters/Maya/`
   - Salvador: `GD_Mercenary_Streaming_SF/SkeletalMesh3/Char_MercBody.*` → `static/models/characters/Salvador/`
   - Gaige:    `GD_Mechromancer_Streaming_SF/SkeletalMesh3/Skel_MechromancerBody.*` → `static/models/characters/Gaige/`
   - Krieg:    `GD_Psycho_Streaming_SF/SkeletalMesh3/Skel_PsychoBody.*` → `static/models/characters/Krieg/`

   (The actual package names on disk may differ slightly — check the folder names
   in your `static/models/` after extraction. The mesh names inside each
   `SkeletalMesh3/` folder are what matter.)

   **Option B — edit `viewer3d.js`'s `CHARACTER_MODELS`** to point directly at
   the `GD_*_Streaming_SF/SkeletalMesh3/` paths. That avoids the copy entirely
   but changes the viewer code. Either works; Option A matches the paths the
   README and `viewer3d.js` already assume.
4. (Optional but recommended) Add a `body_mask.png` into each
   `static/models/characters/<Name>/` folder. This is the per-vertex skin-zone
   alpha mask that makes the body recolor match the in-game A/B/C zone layout.
   Without it the viewer falls back to a luminance-based recolor of the body's
   existing texture — it still works, just less accurate.
5. `python tools/parse_head_mics.py`
6. `python tools/parse_weapon_mics.py`
7. Restart the Flask app.

That's it. Nothing from `GD_*_Streaming_SF.upk`, `CD_*_Skin_*.upk`, or any
world / item / UI package is needed for the currently wired-up viewer.

## Usage

1. Pick a save from the sidebar.
2. Edit character stats in the character panel (auto-saves on blur).
3. Click any weapon or item to preview stats.
4. Use the inventory tabs to manage weapons, items, and bank.
5. Import/export Gibbed codes via the toolbar buttons.
6. Use the 3D viewer to inspect character / weapon models.

**Important.** Close Borderlands 2 before editing. The editor refuses to write
while the game is running and surfaces a warning if it detects the process.

## Project structure

```
bl2-save-editor/
├── app.py                  Flask web server (JSON API)
├── save_io.py              Save file I/O (protobuf + LZO pipeline)
├── asset_db.py             Gibbed asset DB + live stat estimator
├── steam_achievements.py   Steam API ctypes shim
├── patch_bl2.py            Exe patcher (grade-cap rewrite + unpatch)
├── config.py               Cross-platform path auto-detection
├── setup_wizard.py         Interactive end-to-end setup
├── requirements.txt        Python deps
├── start_editor.bat        Windows launcher (auto-opens browser)
├── apply_patch.bat         Self-elevating exe patch launcher
├── templates/
│   └── index.html          Single-page app shell
├── static/
│   ├── app.js              Frontend application logic
│   ├── viewer3d.js         Three.js 3D viewer
│   ├── style.css           BL2-themed holographic UI
│   ├── models/             glTF character / weapon meshes (gitignored)
│   │   ├── characters/     six hardcoded body glTFs (Skel_*Body.gltf) + per-char head fallback
│   │   ├── heads/          one folder per head .upk: <UPK>_SF/{SkeletalMesh3,MaterialInstanceConstant,Texture2D}
│   │   └── weapons/        per-weapon-type glTF (model.gltf + model.bin), optionally per-manufacturer subdirs
│   └── textures/           PNG material maps + Startup.upk texture dump (gitignored)
│       └── Startup/       MaterialInstanceConstant/MasterMati_*.props.txt + Texture2D/*.tga
├── tools/
│   ├── parse_head_mics.py     static/models/heads/ → static/head_materials.json
│   ├── parse_weapon_mics.py   static/textures/Startup/ → static/weapon_materials.json
│   └── split_gestalt.py       PSK → per-weapon glTF splitter (optional, needs .psk exports)
├── tests/
│   ├── test_save_io.py        Save round-trip + atomic-write tests
│   └── test_api.py            Flask API surface tests
└── MOSCOW.md                  Feature prioritization
```

## Compared with Gibbed's Save Editor

| Feature | This editor | Gibbed |
|---|---|---|
| Character stats | ✓ | ✓ |
| Inventory editing | ✓ | ✓ |
| Weapon part editor | ✓ | ✓ |
| Gibbed code import/export | ✓ | ✓ |
| **3D model viewer** | ✓ | — |
| **Web-based UI (cross-platform)** | ✓ | — (WinForms) |
| **Live in-game stat estimation** | ✓ | — |
| **Atomic writes + rotating backups** | ✓ | partial |
| **Damage multiplier mod (PythonSDK)** | ✓ | — |
| Skill tree editor | ✓ | ✓ |

## License

Personal and educational use. Borderlands 2 is a trademark of Gearbox Software
and 2K Games. This editor reads and writes save formats; no game binaries or
copyrighted assets are distributed with this repository.

## Contact

Killian Miller — killianmiller6@gmail.com — github.com/KillianM00
