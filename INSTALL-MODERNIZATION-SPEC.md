# Install/Run Modernization — Spec

## Goal
Make the BL2 Save Editor easy to install and run for end users (primary) while keeping a clean developer path (secondary), without changing the Flask backend or the 3D viewer/SPA.

Concretely: a user should be able to download one archive, extract it, and start the editor with a single run script — no `python -m venv`, no `pip install -r requirements.txt`, no `git clone` of a separate save-format library.

Flask stays. The current app entrypoint (`app.py` → `app.run(...)`) stays as the server core. What changes is the *distribution and bootstrap* around it.

---

## Current pain (why this exists)

1. **`requirements.txt` is one unpinned line:** `flask>=2.0`. No pinned versions, no dev deps, no build metadata.
2. **External save-format dependency is a git clone:** `apocalyptech/borderlands2` into `borderlands2-tool/`, added to `sys.path` at runtime. First run needs git + network.
3. **Advertised setup is venv-centric:** `setup_wizard.py` and the README lead with `.venv` + pip. That's fine for devs but heavyweight for end users.
4. **No Python packaging metadata:** no `pyproject.toml`, no installable package boundary, no CLI entrypoint. The app is invoked as `python app.py`.
5. **CI only covers Go, not Python:** there's no pipeline verifying the Python side installs/runs from a clean env.
6. **PyInstaller already present in the environment** (6.18.0), but not wired into the project as a build step.

---

## Non-go (for now)

- Do not replace Flask.
- Do not rewrite the 3D viewer or SPA.
- Do not touch the desktop/ Fyne/PyQt track yet — that stays a separate concern.
- Do not require a running game, Steam, or BL2 install to *start* the editor; those are runtime feature prerequisites, not install prerequisites.
- Do not promise an auto-updater in this pass.

---

## Target outcomes

### End-user default flow (primary)
1. Download one archive (zip/tar) from a release, or grab a pre-built folder.
2. Extract it.
3. Run the provided launcher:
   - Windows: `start_editor.bat` (or similar)
   - macOS / Linux: a shell launcher (e.g. `run.sh` or `bl2editor`)
4. Browser opens to the local server. No venv activation, no pip, no git.

If a user *does* have Python and wants to run from source, that should also work cleanly (see developer path).

### Developer path (secondary)
1. Clone the repo.
2. One command to install dev deps into an isolated env (venv/uv/pip-tools — TBD).
3. Run from source with a defined entrypoint (not `python app.py` by folklore).
4. Tests and lint pass from a clean install.

The dev path should be a clean derivative of the same packaged source, not a separate hand-rolled setup.

---

## Scope of change

### 1. Packaging metadata
- Add `pyproject.toml` for this project as a Python package/app.
- Define project metadata (name, version, description, license, entry points).
- Declare dependencies explicitly and pinned where practical.
- Keep Flask as the only *runtime* web-framework dep for the Flask app; add everything else the app actually imports at runtime.

### 2. Vendor the save-format library
- Bring `borderlands/` into this repo so the save-format library is part of the project.
- Drop the `.git` and independent history from `borderlands2-tool/` — this is a vendor snapshot, not a submodule in this pass.
- Decide on the on-disk layout for the vendored code (see Open Questions).
- Update the runtime bootstrap so the app finds `borderlands/` without `sys.path.insert` hacks if possible, or with a minimal, well-defined bootstrap if not.

### 3. Entrypoint / launcher story
- Provide a clean way to start the server that is not "open a terminal and run `python app.py`".
- For end users, that means a launcher script/batch in the distributed archive.
- For devs, that means a defined CLI entrypoint or one obvious run command.

### 4. Distribution artifact
- Produce at least one distributable archive from the project (scripted, not manual).
- Ideally also produce a standalone bundled artifact (no system Python required) using the already-available PyInstaller, **once** we confirm the vendored borderlands/ and Flask app bundle cleanly.
- Keep the two artifacts distinct in the story:
  - *Portable archive* = source-ish layout + launcher + bundled deps (if we go that route).
  - *Standalone executable/bundle* = PyInstaller or similar, if viable.

### 5. First-run / config
- Keep config.json + auto-detection, but make failure modes clearer at startup.
- The editor should start even if some optional features are missing (Gibbed data, Steam DLL, BL2 install), with clear logs rather than silent breakage.

### 6. Python CI (minimum viable)
- Add a pipeline step that installs the project from its packaging metadata in a clean environment and runs the existing Python tests.
- This validates the install story continuously.

---

## Proposed structure (illustrative)

This is a sketch, not a final decision. The key constraint is that `borderlands/` must live somewhere this repo can ship it, and the app must find it without the current runtime `sys.path` dance if feasible.

### Option A — vendor into a `vendor/` or `libs/` tree
- `vendor/borderlands/` contains the save-format package.
- App adds that location to `sys.path` once at startup, or uses a wrapper package.
- Simple, explicit, low risk. Mostly a file move + import-path update.

### Option B — make `borderlands` a first-class package in this repo
- The repo contains a `borderlands/` package at top level (or under a clear package dir).
- The app imports it as a normal package.
- Cleaner imports, but touches more import statements and test/sys.path scaffolding.

Either is acceptable; the spec prefers the smallest safe migration that removes the git-clone + sys.path bootstrap.

---

## Runtime dependency inventory (what the Flask app actually needs)

Must capture before finalizing `pyproject.toml`. Known today:

- Flask (web server + JSON API)
- The vendored `borderlands/` save-format library (save read/write, protobuf helpers, huffman, lzo, bitstreams)
- `protobuf` (used directly in app.py for weapon/edit item handling)
- `steam_achievements.py` runtime needs (ctypes against `steam_api64.dll`; no Python Steam SDK required if done via ctypes — confirm)
- Any stdlib-only parts are fine.

Need to confirm:
- Which of the many packages currently installed in the environment are *actually* used by the Flask app vs. pulled in incidentally.
- Whether `borderlands/` itself has its own Python deps that must be declared.

---

## Open questions

### Q1. Exactly where does vendored `borderlands/` live, and what do we call it?
- `vendor/borderlands/`?
- `packages/borderlands/`?
- Top-level `borderlands/` replacing `borderlands2-tool/`?
- Does it keep the `borderlands2-tool/bl2_save_edit.py` CLI shims, or only the `borderlands/` library package?

### Q2. Do we bundle a Python runtime in the end-user archive, or assume Python is present?
- If "assume Python present", the launcher still needs to get the right deps loaded without user pip steps.
- If "bundle Python", PyInstaller (or similar) becomes the main path, and the archive is a standalone app.
- This decision affects whether `start_editor.bat` is thin (find python, run app) or thick (bundled runtime).

### Q3. What is the standalone artifact, exactly?
- PyInstaller onefile or onefolder?
- If onefolder, does it still include the static/ tree and templates/ in a way the Flask app serves correctly?
- Does PyInstaller's import of the vendored borderlands/ and Flask work without special hooks? (To be tested; PyInstaller is already present, which is why this is plausible.)

### Q4. How do we get from "git clone + venv" to "one command" for devs without abandoning isolation?
- `uv`? `pip-tools`? `hatch`? `venv` but scripted?
- Should the dev env be created by a single script/command, with the venv living in a defined location?

### Q5. Versioning
- How is the project version defined and bumped? Single source of truth in `pyproject.toml`, or also in README badges, app metadata, release workflow?
- Do we tag releases from this repo, or is the editor meant to be released as a versioned downloadable asset only?

### Q6. Config + first-run in the standalone case
- In a standalone bundle, where does `config.json` live?
- Where do saves/gibbed data get written?
- Do we keep the current auto-detection (Steam paths, save dirs) in the standalone case, or simplify?

### Q7. What happens to `borderlands2-tool/` in the repo?
- Remove it entirely and replace with vendored `borderlands/`?
- Keep the folder name for compatibility and just strip its `.git` + independent history?
- Any code in this project still references the old path `borderlands2-tool` that needs updating?

### Q8. What about the Gibbed data folder?
- It's optional and user-supplied today.
- In the end-user story, do we ship an empty/default placeholder, document it, or bundle a minimal set?

---

## Suggested implementation order

1. **Inventory true runtime deps** — confirm what the Flask app + vendored borderlands/ actually require.
2. **Add `pyproject.toml`** with metadata + pinned deps, no behavior change yet.
3. **Vendor `borderlands/`** into the repo, drop borderlands2-tool's `.git`, update import/bootstrap.
4. **Make `python -m <entrypoint>` or a defined CLI** work, so the app has a real entrypoint.
5. **Script the distributable archive build** (source-ish layout + launcher + vendored deps).
6. **Validate standalone bundling** with PyInstaller (onefolder vs onefile, static/templates inclusion, borderlands import).
7. **Update README / setup instructions** to reflect the new flow.
8. **Add minimal Python CI** that installs from `pyproject.toml` and runs existing tests.

---

## Not yet decided (leave for after spec review)

- Whether the end-user artifact is source-ish + launcher, or fully standalone via PyInstaller — likely both are useful, but the primary default flow should be the simpler one for gamers.
- Whether to keep a venv at all for devs, or switch to uv/pip-tools.
- The exact launcher script contents per platform.
- Release/publishing mechanism (GitHub release assets vs other).

---

## Success criteria

- A fresh user with no Python/git can download an archive, extract, and run the launcher to see the editor in a browser.
- A developer can clone and run a single command to install deps and start the app from source.
- `borderlands2-tool` git clone is no longer a first-run requirement.
- `requirements.txt`-only unpinned dependency story is replaced by `pyproject.toml` with pinned/explicit deps.
- Python tests can be run from a clean install, and CI reflects that.
