# Fyne UI Replacement for the g3n Native App — Design

**Date:** 2026-10-10 · **Status:** implemented (branch `feat/fyne-ui`, plan `docs/superpowers/plans/2026-10-10-fyne-ui.md`)

## Goal

Replace the primitive g3n-gui widgets in `desktop/native/ui/` with a full-parity
Fyne UI that reads as *Borderlands 2's own interface* — authentic palette,
typography, and chrome — cleaner than the Flask/CSS "holographic" treatment.
Keep the live 3D viewer (g3n) intact as a companion window.

## Decisions made with the user

| Question | Answer |
|---|---|
| Why Fyne? | g3n widgets too primitive (tables/forms/theming); keep g3n for 3D only. |
| "Better" look | More game-authentic: chunky panels, hard corners, grunge-id restrained — BL2's real UI feel. |
| Scope | Full parity with Flask/web UI in one design (not incremental screens). |
| Platforms | Linux + Windows + macOS. |
| 3D strategy | **Option 1** — Fyne main window; g3n viewer as second window opened from a button. |

## Approach (Option 1)

Fyne owns every editing screen. The committed g3n `viewerCtl` re-homes into its
own package and keeps its own window, opened on demand. No FBO blit, no pixel
streaming, no engine mutation. Same binary (`cmd/bl2native`); both toolkits
independent — each owns its own GL context/window on every OS.

Rejected alternatives: offscreen g3n → `canvas.Image` blit (FBO readback cost,
HiDPI mush, 3-OS debug surface); Fyne-only prerendered turntables (breaks live
zoom/orbit parity, needs asset-generation step per model).

## Architecture

```
desktop/
├── native/
│   ├── session/          # UNTOUCHED — save model + mutations (+state contract)
│   ├── setup/            # UNTOUCHED — asset status detection
│   └── ui/               # REWRITTEN: g3n-gui → Fyne
│       ├── app.go        # fyne.App bootstrap, window creation
│       ├── theme.go      # BL2AuthenticTheme: fyne.Theme impl
│       ├── shell.go      # save list | AppTabs | status bar layout
│       ├── saves.go      # save picker list widget
│       ├── character.go  # CHARACTER tab form
│       ├── inventory.go  # INVENTORY sub-tabs + table + detail pane
│       ├── skills.go     # SKILLS tab
│       ├── achievements.go # ACHIEVEMENTS tab
│       └── viewer_bridge.go # "View 3D" → viewer.Run(...)
├── viewer/               # NEW package: moved g3n window code
│   ├── ctl.go            # viewerCtl, character/preview swap, g3n app.Run
│   ├── mesh.go           # previewMesh proxies (flat-PBR, not extracted assets)
│   └── gltf.go           # parseModel / firstModel / error chrome
```

Rules:
- `session.Session` remains the single mutation API. Fyne widgets call it
  exactly as g3n does today; `session` tests are unaffected.
- `viewer.Run(ses *session.Session, st setup.AssetStatus) error` — starts the
  g3n window on a goroutine; closing it only closes that window.
- `desktop/go.mod` gains `fyne.io/fyne/v2`; `g3n/engine` stays. No IPC, no
  second process.
- The Wails + `desktop/frontend` web track is unchanged; Fyne is the UI of the
  `bl2native` binary only.

## Theme — BL2 authentic, better

`theme.go` implements `fyne.Theme` with `static/style.css` as the palette
source of truth. Dark-only (Fyne color mode locked dark — the game has no light UI).

**Palette:** bg `#04060c`, panel `#080e1c`, card `#0a101e`, input `#040810`;
cyan `#3ecfff` primary/border/focus; gold `#e8a33a` accent/selected/legendary;
text `#d4d8e8` / dim `#6b7a9e` / muted `#3a4668`; danger `#e04040`, success `#3bbd40`. A `RarityColors` map mirrors `--rarity-*` exactly (common
`#9d9d9d`, uncommon `#3bbd40`, rare `#4b8be8`, very-rare `#9b59b6`,
legendary `#e8a33a`, e-tech `#d94fff`, seraph `#ff4081`, pearl `#00e5ff`,
effervescent `#ff6ed4`).

**Typography — bundled, offline:** Oswald (headings/tab labels, uppercase,
tracking) and Rajdhani (body/tables/inputs), embedded via `fyne bundle`
(`fyne bundle -name bundled fonts ...`). No runtime font fetch. Sizes:
heading 20, subheading 15, text 14, caption 11, input 14.

**Chrome:**
- Panels: `canvas.Rectangle` fill + 1px `StrokeColor` cyan @ ~20% alpha, hard
  corners (no rounding) — grunge textures/scanlines dropped; Fyne canvas has no
  cheap blur so glow would become baked art.
- Active tab: 2px gold top-stroke; tab labels Oswald uppercase.
- Buttons: panel bg, cyan stroke, gold stroke on hover (theme-level override),
  uppercase labels.
- Tables: no per-cell borders — rarity shown as row text color + 6px color
  square in the icon column.
- Icons: serve bundled `static/icons/*` (`vault_symbol.png` tab icon,
  `backpack.png`, `bank.png`, etc.) through `theme.Icon`.

## Screens (main window 1280×800, min 1024×700)

```
┌───────────────────────────────────────────────────────────┐
│ SAVE LIST │  [CHARACTER][INVENTORY][SKILLS][ACHIEVEMENTS] │
│ Save0001 ──────── tab content ─────────────  [View 3D ▸]  │
│ Save0002          status bar: path · guard · assets       │
└───────────────────────────────────────────────────────────┘
```

- **Save list** (`widget.List`): `SaveNNNN.sav` rows with name, level chip,
  last-modified. Selected save drives all tabs. Auto-refresh on directory
  change (same staleness rule the web app uses).
- **Tab bar**: `container.AppTabs`, four tabs. `bl2native` auto-opens the first
  save (current behavior); save switch re-renders tab content in place.
- **CHARACTER**: `container.Split` — left form grid (name, class select, XP,
  level, playthrough mode, currencies: money/eridium/golden keys/seraph
  crystals, skin/head selectors), right (DUO/prestige stats + danger-zone
  actions). Edits call `session` mutations on change. While the game guard
  reports Borderlands 2 running, inputs disable and a gold guard chip shows
  (parity with web 409 auto-disable).
- **INVENTORY**: inner `AppTabs` WEAPONS / ITEMS / BANK over `widget.Table`
  (icon cell, name [rarity-colored], level, type). Row select → always-visible
  detail pane in the right `Split` (spec lines, parts, effects — replaces the
  web item modal; always-visible is the deliberate improvement). Action
  buttons: duplicate, delete, apply level, edit → existing session/item
  services.
- **SKILLS**: per-class tree columns, point allocation, respec — data from
  committed `skill_trees.json`.
- **ACHIEVEMENTS**: read-only earned/dimmed checklist from `session` (Steam
  shim parity).
- **Status bar**: save path, `setup.AssetStatus`, game-guard chip, on-disk
  staleness warning, last error line.
- **[View 3D]**: `viewer_bridge.go` launches the g3n window (goroutine).
  Closing the viewer does not exit the app; window position remembered per run.

## Error handling

Every `session` mutation error → `dialog.ShowError` + status-bar line. Save
writes stay behind the existing `session` atomic path (temp file → rename →
backups → SHA-1 verify) — nothing in this design touches it.

## Testing

- `go test ./...`, `go vet ./...` stay green; session/setup tests untouched.
- `native/ui`: pure-func extraction for table row projection (index →
  `ItemView`), save-list filtering, currency formatting; widget-construction
  smoke tests in the current `shell_test.go` style (build shell with a fake
  session, assert no panic + tab/list populated).
- Viewer package keeps `gltf` parse tests (moved from `shell_test.go`).
- No cross-toolkit tests (Fyne × g3n never touch).
- Manual: `make dev-native` on Linux; spot-check Windows/macOS builds compile
  (`GOOS=windows go build ./...`, `GOOS=darwin go build ./...`) — runtime
  checks left to the release checklist.

## Out of scope

- Wails/web track (`desktop/frontend`, `internal/bridge`) — untouched.
- Option-2 FBO blit (only if two-window UX proves insufficient later).
- Game-asset extraction pipelines and `tools/parse_*`.
- Save-path invariants, guard mechanics, jit bytecode — read-only for UI.