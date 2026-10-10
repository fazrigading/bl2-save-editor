#!/usr/bin/env bash
# =============================================================================
# extract-assets.sh
#
# One-shot extraction + layout script for the BL2 Save Editor 3D viewer.
#
# What it produces is exactly the tree the const variables in
# static/viewer3d.js expect:
#   CHARACTER_MODELS   -> static/models/characters/<Name>/{body,gltf,body_mask.png}
#   WEAPON_MODELS      -> static/models/weapons/<Type>/model.gltf (+ .bin)
#   ITEM_MODELS        -> static/models/items/<Type>/model.gltf (+ .bin)
#   CLASSMOD_MODELS    -> static/models/items/classmods/<Type>/model.gltf (+ .bin)
#   WEAPON_TEXTURES    -> static/textures/weapons/*.png
#   ITEM_TEXTURES      -> static/textures/items/*.png
#
# UModel must be able to open BL2 .upk files. The script tries, in order:
#   1. $UMODEL           (env)
#   2. umodel in the repo root (next to this script)
#   3. umodel on PATH
#
# The game content path is either $BL2_CONTENT (env) or a prompt at runtime.
# UModel opens BL2 packages fine with -game=border and a -path to the
# WillowGame/Content/ directory (or any parent that contains it).
#
# IMPORTANT: this script copies files into static/. It is safe to re-run;
# it never deletes anything that was already there. If you re-run after a
# different UModel export, existing files are left in place and the script
# reports what is still missing.
#
# Parts that are NOT automated here (documented in the script output):
#   - head .upk extraction (too many packages; see head_mapping.json)
#   - head_models.json / head_materials.json / weapon_materials.json generation
#     (those are produced by the Python parsers: parse_head_mics.py,
#      parse_weapon_mics.py)
#   - body_mask.png (not in any .upk; supplied by the user)
#   - split_gestalt.py (optional; only relevant if you want per-manufacturer
#     weapon variants from PSKs instead of the base gestalt glTFs)
# =============================================================================
set -euo pipefail

# ---- helpers ------------------------------------------------------------------
abspath() {
    local rel="$1"
    if [ -z "${rel##/*}" ]; then echo "$rel"; else echo "$PWD/$rel"; fi
}

die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "[info] $*"; }
ok()   { echo "[ok]   $*"; }
skip() { echo "[skip] $* (already present)"; }
miss() { echo "[miss] $*"; }

# ---- locate umodel ------------------------------------------------------------
UMODEL="${UMODEL:-}"
if [ -z "$UMODEL" ]; then
    if [ -x "$REPO_ROOT/umodel" ]; then
        UMODEL="$REPO_ROOT/umodel"
    elif [ -x "$SCRIPT_DIR/umodel" ]; then
        UMODEL="$SCRIPT_DIR/umodel"
    elif command -v umodel >/dev/null 2>&1; then
        UMODEL="$(command -v umodel)"
    else
        die "umodel not found. Put umodel in the repo root, or set UMODEL=/path/to/umodel."
    fi
fi
UMODEL="$(abspath "$UMODEL")"
[ -x "$UMODEL" ] || die "umodel not executable: $UMODEL"

info "using umodel: $UMODEL"
"$UMODEL" -version >/dev/null 2>&1 && info "$("$UMODEL" -version 2>&1 | head -1)" || true

# ---- game content path -------------------------------------------------------
BL2_CONTENT="${BL2_CONTENT:-}"
if [ -z "$BL2_CONTENT" ]; then
    read -r -p "Borderlands 2 WillowGame/Content/ path (e.g. ~/.steam/steam/steamapps/common/Borderlands 2/WillowGame/Content): " BL2_CONTENT
    BL2_CONTENT="${BL2_CONTENT%"${BL2_CONTENT##*[![:space:]]}"}"  # trim trailing whitespace
fi
BL2_CONTENT="$(abspath "$BL2_CONTENT")"
[ -d "$BL2_CONTENT" ] || die "not a directory: $BL2_CONTENT"

CONTENT_BASE="$BL2_CONTENT/.."  # parent of Content/ (used with -path)
if [ ! -d "$CONTENT_BASE" ]; then
    # fallback: point -path directly at Content/
    CONTENT_BASE="$BL2_CONTENT"
fi

info "using BL2 content: $BL2_CONTENT"
info "using -path for umodel: $CONTENT_BASE"

# ---- repo root / static layout ------------------------------------------------
# Resolve the repo root as the directory containing this script. That way the
# script works when invoked as ./extract-assets.sh from the repo root, or as
# bash path/to/extract-assets.sh from elsewhere.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
STATIC_DIR="$REPO_ROOT/static"
[ -d "$STATIC_DIR" ] || die "static/ not found at $STATIC_DIR (run from repo root)"

UMODEL_OUT="$REPO_ROOT/umodel_extract"
mkdir -p "$UMODEL_OUT"

# ---- option: keep extracted tree for inspection ------------------------------
KEEP_EXTRACT="${KEEP_EXTRACT:-0}"
if [ "$KEEP_EXTRACT" = "1" ]; then
    info "KEEP_EXTRACT=1: leaving UModel export tree under $UMODEL_OUT after the run"
fi

# =============================================================================
# SECTION 1: Character bodies (Startup.upk SkeletalMesh3)
# =============================================================================
# viewer3d.js CHARACTER_MODELS wants:
#   Axton     -> static/models/characters/Axton/Skel_SoldierBody.gltf (+ .bin)
#   Zer0      -> static/models/characters/Zer0/Skel_AssassinBody.gltf
#   Maya      -> static/models/characters/Maya/Skel_SirenBody.gltf
#   Salvador  -> static/models/characters/Salvador/Char_MercBody.gltf
#   Gaige     -> static/models/characters/Gaige/Skel_MechromancerBody.gltf
#   Krieg     -> static/models/characters/Krieg/Skel_PsychoBody.gltf
#
# You told me these live inside each GD_*_Streaming_SF.upk under SkeletalMesh3/.
# UModel can export a single named object with -obj=...; that is more precise
# than exporting the whole package and then searching the tree.
# =============================================================================

declare -A BODY_SRC=(
    ["Axton"]="GD_Soldier_Streaming_SF:Skel_SoldierBody"
    ["Zer0"]="GD_Assassin_Streaming_SF:Skel_AssassinBody"
    ["Maya"]="GD_Siren_Streaming_SF:Skel_SirenBody"
    ["Salvador"]="GD_Mercenary_Streaming_SF:Char_MercBody"
    ["Gaige"]="GD_Mechromancer_Streaming_SF:Skel_MechromancerBody"
    ["Krieg"]="GD_Psycho_Streaming_SF:Skel_PsychoBody"
)

info "=== Section 1: character bodies (Startup.upk / GD_*_Streaming_SF) ==="

UMODEL_EXTRACT_SUB="$UMODEL_OUT/bodies"
rm -rf "$UMODEL_EXTRACT_SUB"
mkdir -p "$UMODEL_EXTRACT_SUB"

for char in Axton Zer0 Maya Salvador Gaige Krieg; do
    IFS=: read -r pkg mesh <<< "${BODY_SRC[$char]}"
    dest_dir="$STATIC_DIR/models/characters/$char"
    dest_gltf="$dest_dir/$mesh.gltf"
    dest_bin="$dest_dir/$mesh.bin"
    pkgfile="$BL2_CONTENT/$pkg.upk"

    if [ -f "$dest_gltf" ] && [ -f "$dest_bin" ]; then
        skip "character body $char -> $dest_gltf"
        continue
    fi

    mkdir -p "$dest_dir"

    if [ ! -f "$pkgfile" ]; then
        miss "package not found for $char: $pkgfile (need to extract $pkg.upk)"
        continue
    fi

    info "extracting $pkg.mesh=$mesh for $char from $pkgfile"
    # UModel: -export -gltf -obj=<MeshName> -out=<dir> <package>
    # The exported file lands in <out>/<mesh>.gltf (and .bin when glTF writes buffers external).
    #
    # If the mesh name parsing fails (UModel can't find the object), it will emit a
    # warning to stderr; we still attempt to pick up whatever landed in the out dir.
    if "$UMODEL" -export -gltf \
        -game=border \
        -path="$CONTENT_BASE" \
        -out="$UMODEL_EXTRACT_SUB/$char" \
        -obj="$mesh" \
        "$pkgfile" >/dev/null 2>"$UMODEL_EXTRACT_SUB/$char.err"; then

        # UModel often writes into <out>/<mesh>/ or directly <out>/ depending on version.
        # Find the gltf we just asked for.
        found_gltf="$(find "$UMODEL_EXTRACT_SUB/$char" -name "$mesh.gltf" -print -quit 2>/dev/null || true)"
        if [ -z "$found_gltf" ]; then
            # Maybe UModel exported the whole package tree; search one level deeper.
            found_gltf="$(find "$UMODEL_EXTRACT_SUB/$char" -name "$mesh.gltf" -print -quit 2>/dev/null || true)"
        fi

        if [ -n "$found_gltf" ]; then
            cp "$found_gltf" "$dest_gltf"
            bin="$(echo "$found_gltf" | sed 's/\.gltf$/.bin/')"
            if [ -f "$bin" ]; then
                cp "$bin" "$dest_bin"
            else
                miss "expected companion .$mesh.bin not found next to $found_gltf (body may still render but is incomplete)"
            fi
            ok "placed $char body: $dest_gltf"
        else
            errfile="$UMODEL_EXTRACT_SUB/$char.err"
            miss "no .$mesh.gltf produced for $char; umodel stderr:"
            sed 's/^/    /' "$errfile" >&2 || true
        fi
    else
        errfile="$UMODEL_EXTRACT_SUB/$char.err"
        miss "umodel export failed for $char ($pkg / $mesh); umodel stderr:"
        sed 's/^/    /' "$errfile" >&2 || true
    fi
done

# ---- body_mask.png note -----------------------------------------------------
info "=== body_mask.png (NOT in any .upk; user-supplied) ==="
for char in Axton Zer0 Maya Salvador Gaige Krieg; do
    p="$STATIC_DIR/models/characters/$char/body_mask.png"
    if [ -f "$p" ]; then
        ok "body_mask.png present: $p"
    else
        miss "body_mask.png missing: $p"
    fi
done

# =============================================================================
# SECTION 2: Weapon + item + class mod glTFs
# =============================================================================
# Viewer consts:
#   WEAPON_MODELS  -> static/models/weapons/<Type>/model.gltf (+ .bin)
#   ITEM_MODELS    -> static/models/items/<Type>/model.gltf (+ .bin)
#   CLASSMOD_MODELS-> static/models/items/classmods/<Type>/model.gltf (+ .bin)
#
# The base weapon / item / class mod meshes are gestalt skeletal meshes inside
# Startup.upk / SkeletalMesh3/. For a minimal viewer setup you can either:
#   (a) extract those PSKs with UModel and run split_gestalt.py, OR
#   (b) place pre-split glTFs manually at the paths above.
#
# This script does NOT run split_gestalt.py automatically (it reads PSKs and
# emits per-manufacturer glTFs; that is a separate, optional step). Instead it
# checks whether the final model.gltf paths exist and reports gaps.
# =============================================================================
info "=== Section 2: weapon / item / class mod glTFs (model.gltf) ==="

declare -A WEAPON_TYPES=(
    ["Pistol"]="Pistol"
    ["Assault Rifle"]="Assault_Rifle"
    ["SMG"]="SMG"
    ["Shotgun"]="Shotgun"
    ["Sniper Rifle"]="Sniper_Rifle"
    ["Rocket Launcher"]="Rocket_Launcher"
)

for label in "${!WEAPON_TYPES[@]}"; do
    folder="${WEAPON_TYPES[$label]}"
    dest="$STATIC_DIR/models/weapons/$folder/model.gltf"
    if [ -f "$dest" ]; then
        ok "weapon model present: $dest"
    else
        miss "weapon model missing: $dest (run UModel on Startup.upk SkeletalMesh3, then split_gestalt.py, or place glTF manually)"
    fi
done

declare -A ITEM_TYPES=(
    ["Shield"]="Shield"
    ["Grenade Mod"]="Grenade_Mod"
    ["Relic"]="Relic"
)
for label in "${!ITEM_TYPES[@]}"; do
    folder="${ITEM_TYPES[$label]}"
    dest="$STATIC_DIR/models/items/$folder/model.gltf"
    if [ -f "$dest" ]; then
        ok "item model present: $dest"
    else
        miss "item model missing: $dest (Startup.upk SkeletalMesh3 gestalt PSK -> split_gestalt.py, or place glTF manually)"
    fi
done

declare -A CLASSMOD_TYPES=(
    ["Axton"]="ClassMod_Soldier"
    ["Zer0"]="ClassMod_Assassin"
    ["Maya"]="ClassMod_Siren"
    ["Salvador"]="ClassMod_Merc"
    ["Gaige"]="ClassMod_Merc"
    ["Krieg"]="ClassMod_Merc"
)
for char in Axton Zer0 Maya Salvador Gaige Krieg; do
    folder="${CLASSMOD_TYPES[$char]}"
    dest="$STATIC_DIR/models/items/classmods/$folder/model.gltf"
    if [ -f "$dest" ]; then
        ok "class mod model present: $dest ($char)"
    else
        miss "class mod model missing: $dest ($char) (Startup.upk SkeletalMesh3 -> split_gestalt.py, or place glTF manually)"
    fi
done

# =============================================================================
# SECTION 3: Startup.upk textures -> static/textures/weapons + items
# =============================================================================
# The viewer loads these PNGs by path. They originate from
# static/textures/Startup/Texture2D/ (your UModel export of Startup.upk
# Texture2D/), but must be reachable at the viewer's expected paths.
#
# UModel `-png` exports textures as PNG, so we want to ensure PNGs exist in
# the Startup/Texture2D dump; if they are .tga only, this script cannot
# convert them (that is what parse_weapon_mics.py does on first run for
# weapon MICs, but the viewer itself needs PNG at the weapons/items paths).
# =============================================================================
info "=== Section 3: Startup textures -> static/textures/weapons + items ==="

STARTUP_TEX="$STATIC_DIR/textures/Startup/Texture2D"
WEAPON_TEX_DEST="$STATIC_DIR/textures/weapons"
ITEM_TEX_DEST="$STATIC_DIR/textures/items"

declare -A WEAPON_TEX=(
    ["Weap_Pistols_Comp.png"]="Pistol"
    ["Weap_Pistols_Nrm.png"]="Pistol"
    ["Weap_LauncherShotgunPistol_Comp.png"]="Pistol/Shotgun/RL shared"
    ["Weap_AssaultRifle_Comp.png"]="Assault Rifle"
    ["Weap_AssaultRifle_Nrm.png"]="Assault Rifle"
    ["Weap_AssaultSubSniper_Comp.png"]="AR/SMG/Sniper shared"
    ["Weap_SMG_Comp.png"]="SMG"
    ["Weap_SMG_Nrm.png"]="SMG"
    ["Weap_Shotgun_Comp.png"]="Shotgun"
    ["Weap_Shotgun_Nor.png"]="Shotgun"
    ["Weap_SniperRifle_Comp.png"]="Sniper Rifle"
    ["Weap_SniperRifle_Nrm.png"]="Sniper Rifle"
    ["Weap_Launchers_Comp.png"]="Rocket Launcher"
    ["Weap_Launchers_Nrm.png"]="Rocket Launcher"
)
declare -A ITEM_TEX=(
    ["Shield_Dif.png"]="Shield"
    ["Shield_Nrm.png"]="Shield"
    ["Grenades_Dif.png"]="Grenade Mod"
    ["Grenades_Nrm.png"]="Grenade Mod"
    ["ItemArtifacts_Comp.png"]="Relic"
    ["SoldierClassMod01_Diff.png"]="Axton class mod"
    ["SoldierClassMod01_Norm.png"]="Axton class mod"
    ["AssassinClassMod01_Dif.png"]="Zer0 class mod"
    ["AssassinClassMod01_Nrm.png"]="Zer0 class mod"
    ["SirenClassMod01_Dif.png"]="Maya class mod"
    ["SirenClassMod01_Nrm.png"]="Maya class mod"
    ["MercClassMod01_Dif.png"]="Salvador/Gaige/Krieg class mod"
    ["MercClassMod01_Nrm.png"]="Salvador/Gaige/Krieg class mod"
)

mkdir -p "$WEAPON_TEX_DEST" "$ITEM_TEX_DEST"

for file in "${!WEAPON_TEX[@]}"; do
    src="$STARTUP_TEX/$file"
    dst="$WEAPON_TEX_DEST/$file"
    if [ -f "$dst" ]; then
        skip "weapon texture present: $dst"
        continue
    fi
    if [ -f "$src" ]; then
        cp "$src" "$dst" && ok "copied weapon texture: $src -> $dst" || miss "failed to copy $src -> $dst"
    else
        # .tga variant?
        tga="${file%.png}.tga"
        if [ -f "$STARTUP_TEX/$tga" ]; then
            miss "weapon texture $file absent as PNG in $STARTUP_TEX (only .$tga found); convert with ImageMagick or let parse_weapon_mics.py handle it for MICs — but viewer needs PNG at $dst"
        else
            miss "weapon texture source missing: $src (and .$tga not found either)"
        fi
    fi
done

for file in "${!ITEM_TEX[@]}"; do
    src="$STARTUP_TEX/$file"
    dst="$ITEM_TEX_DEST/$file"
    if [ -f "$dst" ]; then
        skip "item texture present: $dst"
        continue
    fi
    if [ -f "$src" ]; then
        cp "$src" "$dst" && ok "copied item texture: $src -> $dst" || miss "failed to copy $src -> $dst"
    else
        tga="${file%.png}.tga"
        if [ -f "$STARTUP_TEX/$tga" ]; then
            miss "item texture $file absent as PNG in $STARTUP_TEX (only .$tga found); convert with ImageMagick or let parsers handle MICs — viewer needs PNG at $dst"
        else
            miss "item texture source missing: $src (and .$tga not found either)"
        fi
    fi
done

# =============================================================================
# SECTION 4: PSK source check for split_gestalt.py (optional)
# =============================================================================
# split_gestalt.py reads PSKs under BL2_PSK_DIR (default: static/models/Startup/SkeletalMesh3).
# If you extracted Startup.upk's SkeletalMesh3 into that folder already, we can
# confirm the PSKs it needs are present. Otherwise we just note what is missing.
# =============================================================================
info "=== Section 4: PSK source for split_gestalt.py (optional) ==="
PSK_DIR="${BL2_PSK_DIR:-$STATIC_DIR/models/Startup/SkeletalMesh3}"

declare -A PSK_NEEDED=(
    ["Pistol"]="GestaltDef_Pistols_GestaltSkeletalMesh.psk"
    ["Assault Rifle"]="GestaltDef_AssaultRifles_GestaltSkeletalMesh.psk"
    ["SMG"]="GestaltDef_SMGs_GestaltSkeletalMesh.psk"
    ["Shotgun"]="GestaltDef_Shotguns_GestaltSkeletalMesh.psk"
    ["Sniper Rifle"]="GestaltDef_SniperRifles_GestaltSkeletalMesh.psk"
    ["Rocket Launcher"]="GestaltDef_RocketLaunchers_GestaltSkeletalMesh.psk"
    ["Shield"]="GestaltDef_Shields_GestaltSkeletalMesh.psk"
    ["Grenade Mod"]="GestaltDef_Grenades_GestaltSkeletalMesh.psk"
    ["Relic"]="GestaltDef_Artifacts_GestaltSkeletalMesh.psk"
    ["Axton"]="SoldierClassMod01.psk"
    ["Zer0"]="ZeroClassMod01.psk"
    ["Maya"]="SirenClassmod01.psk"
    ["Salvador"]="MercClassmod01.psk"
    ["Gaige"]="MercClassmod01.psk"
    ["Krieg"]="MercClassmod01.psk"
)

if [ -d "$PSK_DIR" ]; then
    for label in "${!PSK_NEEDED[@]}"; do
        psk="${PSK_NEEDED[$label]}"
        if [ -f "$PSK_DIR/$psk" ]; then
            ok "PSK present for $label: $psk"
        else
            miss "PSK missing for $label: $psk (needed by split_gestalt.py to build $label model.gltf)"
        fi
    done
else
    miss "PSK dir not found: $PSK_DIR"
    miss "If you want weapon/item/classmod glTFs, extract Startup.upk/SkeletalMesh3 into that folder and run: BL2_PSK_DIR=$PSK_DIR python tools/split_gestalt.py"
fi

# =============================================================================
# SECTION 5: Head packages (informational only)
# =============================================================================
# Heads are too many to automate in one script. Point the user at head_mapping.json.
# =============================================================================
info "=== Section 5: heads (manual) ==="
HEAD_MAPPING="$STATIC_DIR/head_mapping.json"
HEAD_DIR="$STATIC_DIR/models/heads"
if [ -f "$HEAD_MAPPING" ]; then
    count="$(python -c "import json,sys;print(len(json.load(open('$HEAD_MAPPING'))))" 2>/dev/null || echo "?")"
    info "$count head asset entries in head_mapping.json"
    info "To render heads, extract each UPK named in head_mapping.json into $HEAD_DIR/<UPK_name>/"
    info "Then run: python tools/parse_head_mics.py  (produces static/head_materials.json)"
    info "and create static/head_models.json mapping in-game asset paths to glTF URLs."
    head_count="$(find "$HEAD_DIR" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l)"
    info "head dirs already on disk: $head_count"
else
    miss "head_mapping.json not found at $HEAD_MAPPING"
fi

# =============================================================================
# SECTION 6: JSON sidecars check
# =============================================================================
info "=== Section 6: JSON sidecars ==="
for j in head_models.json head_materials.json weapon_materials.json skin_colors.json gestalt_map.json part_mesh_names.json; do
    if [ -f "$STATIC_DIR/$j" ]; then
        ok "sidecar present: static/$j"
    else
        miss "sidecar missing: static/$j"
    fi
done

# =============================================================================
# SUMMARY
# =============================================================================
info "=== Summary ==="
info "UModel        : $UMODEL"
info "BL2 content   : $BL2_CONTENT"
info "UModel -path  : $CONTENT_BASE"
info "UModel extract: $UMODEL_OUT (temporary unless KEEP_EXTRACT=1)"
info ""
info "Body glTFs are placed into static/models/characters/<Name>/ by Section 1."
info "Weapon/item/classmod glTFs must be placed into static/models/weapons|items|items/classmods/ by you or split_gestalt.py (Section 2/4)."
info "Startup textures are copied to static/textures/weapons/ and static/textures/items/ by Section 3."
info "Heads, body_mask.png, and JSON sidecars are NOT fully automated here — see the notes above."
info ""
info "Next steps after this script:"
info "  1. If weapon/item/classmod glTFs are missing: extract Startup.upk/SkeletalMesh3 to $STATIC_DIR/models/Startup/SkeletalMesh3, then:"
info "       BL2_PSK_DIR=$STATIC_DIR/models/Startup/SkeletalMesh3 BL2_WEAPON_MODELS_OUT=$STATIC_DIR/models/weapons python tools/split_gestalt.py"
info "  2. Extract head .upks per head_mapping.json into static/models/heads/, then:"
info "       python tools/parse_head_mics.py"
info "       python tools/parse_weapon_mics.py"
info "  3. Add body_mask.png per character if you want accurate skin zones (optional; fallback is luminance-based)."
info "  4. Restart Flask: python app.py"
