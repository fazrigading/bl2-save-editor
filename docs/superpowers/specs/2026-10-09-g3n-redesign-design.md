# Native g3n Redesign — Design Spec (v2, approved 2026-10-09)

## Intent

Replace the Wails web frontend (`desktop/frontend/dist`, a ~1:1 copy of the
Flask `templates/` + `static/`: vanilla JS ~3.8k LOC, CSS ~2.5k LOC, Three.js
r128) with a native Go UI driven by `g3n/engine` (GUI + 3D in one window).
Backend phases 1–5 per `PLAN.md` are done and frozen; the Go core is reused
as a library. 3D viewer assets (README "3D viewer asset extraction") become a
**required** first-launch setup step, not optional.

Assumptions (confirmed with owner): stack = pure g3n, no HTML retained;
scope = MVP slice first; BL2 visual identity kept, layout rebuilt.

## §1 Architecture

- New Go module `desktop-native/` (sibling of `desktop/`, same repo):
  `cmd/bl2native` + `internal/ui/{shell,saves,character,inventory,viewer,theme}`
  + `internal/modelpipe/` (Go port of the asset pipeline scripts).
- Reuse `desktop/internal/{editor,assets,platform,bl2save}` by direct import.
  Verified Wails-free: 52 `(*Store)` methods, `assets.DB` resolvers
  (`ResolveItemParts`, `EstimateWeaponStats`, rarity/element/manufacturer
  detection, `CleanPartName`, `BuildDisplayName`), `platform` config/detect/
  Gibbed/process APIs. Only `desktop/internal/bridge` + `desktop/main.go`
  touch `wails/runtime` (menus, dialogs, `EventsEmit` progress).
- `native/session.go` replaces `bridge/`: typed view-model structs instead of
  `map[string]any`; mutation → re-extract → re-render fully in-process
  (no IPC round trip — kills even the `window.API` facade call);
  Gibbed-download progress via Go channels polled per frame (replaces
  `EventsEmit`); folder picker via `sqweek/dialog` (zenity on Linux, native
  on Windows/macOS).
- `desktop/` Wails app frozen until native reaches parity, then deleted.
  Python `app.py` / `templates/` / `static/` untouched throughout.

## §2 3D viewer (three.js → g3n)

- One g3n scene, two modes. Character mode: glTF character mesh + head swap
  (`swapHeadModel` equivalent), skin-zone tint (`applySkinColors`/
  `applyCharacterTint` equivalents), equipped accent light, orbit control
  (`camera.NewOrbitControl`). Preview mode: weapon/item mesh (glTF where
  extracted, procedural placeholder only when an individual mesh is missing),
  per-rarity/per-element PBR `material.Standard` materials, equipment
  carousel prev/next, back-to-character. (Per-mesh procedural fallback for
  individually missing meshes is retained per README; what is removed is the
  old app-level silent degrade when the whole asset dir is absent.)
- Texture compositing (mask + diffuse + MIC colors, skin masks) ported from
  `viewer3d.js` canvas code to Go `image` at load time with in-memory cache.
- Gestalt part-visibility rules ported as static visibility sets for MVP;
  animated pulses / special-effect ticks deferred to slice 2.
- Existing glTF/GLB + PNG + material JSON sidecars reused as-is.
- **No silent placeholder fallback at app level**: missing asset dir renders
  an explicit setup CTA panel in the viewer pane.

## §3 UI structure + theme

- HSplit shell: left save `List` (name/class/level, active highlight),
  right `TabBar`. MVP tabs: CHARACTER, INVENTORY only; viewer docked right
  of inventory content.
- Character tab: name `Edit`, class `Label`, XP bar (custom `Panel`),
  level/skill-point numeric `Edit`s, currency grid, 3-zone color editors.
- Inventory tab: weapons/items/bank sub-tabs as `Table` rows
  (name/rarity/level) instead of HTML cards; row click selects → preview
  pane + stat panel (`BuildDisplayName` + `EstimateWeaponStats`).
  Actions via right-click `Menu` (delete/duplicate/transfer/set-level/
  move up/down/top) — replaces contextmenu **and** drag-drop (g3n `List`
  has no DnD; accepted).
- Theme: BL2 identity rebuilt as g3n `Style` — amber `#fcb100`, dark slate
  background, Oswald/Rajdhani loaded as TrueType, full rarity palette.
  Fresh layout, same soul.

## §4 Data flow + errors + first-launch

- Main-thread UI rule: g3n renders on main thread; `Store` mutations run
  synchronously (ms-scale) with busy cursor; goroutines only for Gibbed
  download and preview-asset loads, posting to channels polled each frame.
- Flow: select save → `ReadSave` + `Extract*` once → session holds tree +
  derived view-models; mutate → `Store` call → re-extract affected tab →
  patch widgets in place (mirrors `applySaveState` single-pass, no full
  rebuild, no refetch round trip).
- `platform.IsGameRunning()` checked before every write + banner label
  while running (replaces backend guard + web banner).
- Errors surface as modal `gui.Window` message boxes; invalid save names
  rejected at selection; nothing fails silently.
- **Required-asset startup probe**: on launch (and on config load), check
  asset dir for models + textures + material JSONs. Absent → viewer pane
  shows setup CTA; MVP setup panel exposes path `Edit`s + Detect + Save +
  asset check. Full guided wizard (incl. UModel-export step) lands in
  slice 3.

## §5 Testing + verification

- Existing `go test ./...` golden/unit coverage of reused packages stays
  green and untouched.
- New (gui-free, headless): `session_test.go` (mutate → re-extract →
  view-model asserts; session decoupled from widgets), texture-composite
  unit tests (mask+dif+color fixtures), rarity→material mapping table
  tests, `modelpipe` port tests (Python-vs-Go output parity on fixtures),
  asset-probe tests (present/missing/partial).
- Manual gate per slice mirroring `PLAN.md` Phase 5: load/edit/delete/
  duplicate/transfer/set-level, guard blocks writes while game runs,
  Gibbed progress visible, stderr clean. Slice 0 adds toolchain proof:
  hellog3n-style window + one real glTF render on owner's Windows +
  Linux boxes.

## §6 Build + risks + slices

- System deps swap webkit2gtk for GL/X11: Linux
  `xorg-dev libgl1-mesa-dev` (+ glfw X11 deps) / Fedora equivalents;
  Windows mingw-w64 + g3n audio DLLs on PATH. `Makefile` gains
  `build-native` / `dev-native`; CI builds both modules.
- Risks: (1) g3n latest tag is `v0.2.0` (2021), master untagged — slice 0
  pins the version (`v0.2.0` first, master pseudo-version fallback,
  recorded in `go.mod`); (2) g3n gui lacks DnD/date-picker/native dialogs
  — accepted (§3 menu-reorder, `sqweek/dialog`); (3) CGO cross-compilation
  is harder — releases via per-OS runners (existing Releases flow).
- Slices: **0** spike (window + save list + one glTF, toolchain proof) →
  **1** MVP (character + inventory + preview, usable) → **2** remaining
  tabs (skills/missions/fast-travel/challenges/achievements) → **3** setup
  wizard + Go-ported extraction pipeline (`parse_head_mics.py`,
  `parse_weapon_mics.py`, `split_gestalt.py` if needed) + Gibbed
  codes + loadouts + menus/dialogs → retire Wails `desktop/`.
- Asset storage: user-data dir beside `config.json`, env overrides carried
  over from `BL2_MATI_EXTRACT` / `BL2_PSK_DIR` / `BL2_WEAPON_MODELS_OUT`
  analogues. UModel stays external (user-run, or shelled if on PATH);
  wizard consumes its export dir, runs the Go pipeline, verifies with a
  trial g3n glTF load.
- Success bar: slice 1 edits real saves with byte-identical round-trip
  (existing goldens) and the viewer renders extracted character/weapon
  meshes. No redistribution of game assets (owner-extracted only, per
  License section).

## Explicitly skipped

Fyne/Gio hybrid UIs (two GL contexts), big-bang full parity, retaining any
HTML/CSS/JS, shelling out to Python for the model pipeline (Go port
instead). Revisit only if g3n gui gaps force escape hatch B.
