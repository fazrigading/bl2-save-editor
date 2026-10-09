# AGENTS.md — bl2-save-editor

Two-track repo. Python Flask app at root is functional reference; Go/Wails native rewrite in `desktop/` is active development. Desktop API intentionally diverges from Flask API (`PLAN.md`).

## Setup

- Preferred: `python setup_wizard.py` (idempotent; `--yes --launch` for zero prompts). On Windows use `python`, not `python3` (Store stub).
- Manual: `python -m venv .venv && pip install -r requirements.txt && pip install pytest`, then `git clone --depth 1 https://github.com/apocalyptech/borderlands2.git` (checked out as `borderlands2-tool/`, gitignored, **required** by Python app), copy `config.example.json` → `config.json` (gitignored; empty fields auto-detect via `config.py`).
- Run Python app: `python app.py` → `http://localhost:5000` (next free port).
- Desktop: `cd desktop && make build` / `make dev` (both pass `-tags webkit2_41`). Linux needs webkit2gtk4.1 dev pkgs first (`libgtk-3-dev libwebkit2gtk-4.1-dev` Debian/Ubuntu, `webkit2gtk4.1-devel` Fedora). Standalone patcher: `go build ./cmd/bl2patch`.

## Verify

- Python: `python -m pytest tests/ -v`; single file: `python -m pytest tests/test_save_io.py -v`.
- Desktop: `cd desktop && go test ./...` and `go vet ./...`. After touching bound methods, rebuild (`make build` regenerates `frontend/wailsjs/`) and confirm zero binding drift.
- No Python linter configured; no JS build step (vanilla ES6, refresh to reload).

## Layout

- Root: `app.py` (Flask JSON API), `save_io.py` (decrypt/parse/mutate/write pipeline), `asset_db.py` (Gibbed data + stat estimator), `steam_achievements.py` (ctypes shim), `patch_bl2.py` (grade-cap byte patch), `config.py` (auto-detect), `templates/index.html` + `static/app.js` + `static/viewer3d.js`.
- `desktop/`: `internal/{bl2save,bridge,editor,assets,platform}` (six bound services `App,Saves,Editor,Items,Assets,Steam` wrap private `*Bridge` core — never embed it), `frontend/dist/static/api.js` (`window.API` facade over `window.go.bridge.*`), `frontend/wailsjs/` bindings are **committed**, `build/bin/` ignored.
- `tools/parse_{head,weapon}_mics.py` regenerate viewer material sidecars; `BL2_MATI_EXTRACT`, `BL2_PSK_DIR`, `BL2_WEAPON_MODELS_OUT` override paths.

## Invariants (save path — do not weaken)

- `save_io.py`: temp-file write → atomic rename → rotate up to 5 `.bak` → re-read and assert SHA-1 + item count. Never bypass rotating backups, pre-write validation, or post-write verification; round-trip (read→modify→write→read) must leave unmodified fields identical.
- Writes blocked while game runs (`Borderlands2.exe` via tasklist/pgrep, 3s cache; Flask returns 409, desktop `guardGame()`). Save filenames must match `^Save\d{4}\.sav$`.
- Desktop mutations return merged save state (+state) so frontend re-renders in one round trip — except `Editor.UpdateCharacter` (returns char info only). Don't reintroduce mutate→refetch or `Invoke(method,url,body)` / `await api(` shims (grep must stay at 0).

## Gotchas

- `config.json`, `borderlands2-tool/`, `static/models/`, `static/textures/` are gitignored; viewer falls back to procedural rarity-tinted placeholders when assets absent — not an error.
- Close Borderlands 2 before editing; editor refuses writes while it runs.
- Frontend perf contracts: one delegated listener set per inventory container (cards carry `_item`), rarity pulse is opacity-only `::after` + `prefers-reduced-motion` kill switch, 3D viewers render on dirty flag, all `<script src>` deferred in order.
