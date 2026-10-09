# Slice 0: Foundation + Native Spike — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the native toolchain (g3n window + real save read + one glTF render) with a headless-tested session core.

**Architecture:** Same-module native packages under `desktop/` reuse `internal/{editor,assets,platform}` directly; a headless `session` package owns load/open logic (fully unit-tested); a thin `ui` shell proves the GL window. Later slices build character/inventory tabs on this session.

**Tech Stack:** Go 1.25, `github.com/g3n/engine` (version pinned in Task 1), `github.com/sqweek/dialog` (picker; wired in slice 3, dependency added now only if needed — default: defer), existing `bl2save/desktop` module.

**Spec:** `docs/superpowers/specs/2026-10-09-g3n-redesign-design.md` (§§1,2,4–6; slice-0 scope)

## Global Constraints

- Module stays `bl2save/desktop` (DP-1: same-module placement, see Task 1); Go 1.25.0.
- `desktop/internal/*` is read-only in this plan — no edits, reuse only.
- Fixture: `desktop/tests/Save0001.sav` (exists; bridge tests use it).
- Tests must NOT write the real user config file (unlike `bridge_test.go`'s `testBridge`, which does) — construct `platform.Config` in memory.
- Errors are plain `errors.New` (backend convention).
- No HTML/JS/CSS in this plan. No new Python.
- Every task ends with a commit; `go test ./...` + `go vet ./...` green before each commit.

## Spec Deviation DP-1 (needs owner sign-off with this plan)

Spec §1 says "new Go module `desktop-native/`". Plan implements `desktop/cmd/bl2native` + `desktop/native/...` in the existing module instead: `internal/` packages are only importable inside their parent module, so a sibling module would need `replace` directives and a second `go.mod` to track — heavier for zero isolation benefit at slice 0. Revisit only if Wails and g3n build tags ever conflict.

## Review Focus

1. Missing/unconfigured `config.json` → app must show setup CTA, never nil-panic. (Test: Task 2.)
2. Empty save dir → save list renders empty state, no error. (Test: Task 2.)
3. Corrupt/truncated `.sav` bytes → `OpenSave` returns error, window stays alive. (Test: Task 2.)
4. Missing/empty asset dir → viewer pane shows CTA, app still runs. (Test: Task 3.)
5. Path traversal filename (`../../etc/passwd`) → rejected before touching disk. (Test: Task 2.)

---

### Task 1: Scaffold entry, deps, build targets

**Files:**
- Modify: `desktop/go.mod` (add g3n require)
- Create: `desktop/cmd/bl2native/main.go`
- Modify: `desktop/Makefile` (add `build-native`, `dev-native`)

**Interfaces:**
- Consumes: `platform.Load() *Config`, `platform.ConfigPath() string` (existing).
- Produces: runnable `bl2native` binary; `setup.DefaultAssetDir() string` consumed by Task 3 (signature fixed here, implemented in Task 3).

- [ ] **Step 1: Pin g3n version.** Run `go list -m -versions github.com/g3n/engine`. If `v0.2.0` builds under Go 1.25, `go get github.com/g3n/engine@v0.2.0`; else use latest master pseudo-version (`go get github.com/g3n/engine@master`). Record chosen version in commit message.
- [ ] **Step 2: Write `desktop/cmd/bl2native/main.go`.** Signature: `func main()` — loads `platform.Load()` and prints configured/unconfigured status to stdout. It must NOT import `native/session`, `native/setup`, or `native/ui` yet (those packages don't exist until Tasks 2–4; Task 4 does the wiring). Body: `cfg := platform.Load()`; if `!cfg.Valid()` print setup notice, else print save dir. This task's deliverable is compilation + dep pin, not behavior.
- [ ] **Step 3: Add Makefile targets.**
```make
build-native:
	go build -o build/bin/bl2native ./cmd/bl2native
dev-native:
	go run ./cmd/bl2native
```
- [ ] **Step 4: Verify compile.** Run `go build ./cmd/bl2native` from `desktop/`. Expected: success (window need not open yet — `ui.Run` stub returns nil).
- [ ] **Step 5: Commit.**
```bash
git add desktop/go.mod desktop/go.sum desktop/cmd/bl2native/main.go desktop/Makefile
git commit -m "feat(native): scaffold bl2native entry, g3n <version>, make targets"
```

### Task 2: Headless session core

**Files:**
- Create: `desktop/native/session/session.go`
- Create: `desktop/native/session/session_test.go`

**Interfaces:**
- Consumes: `editor.NewStore(saveDir string, backupGens int, loadoutDir string) *Store`, `Store.ListSaves()`, `Store.ReadSave(filename)`, `Store.ExtractCharacterInfo(tree)`, `assets.New(gibbedDir string) *DB`, `platform.Config{SaveDir, GibbedDir, BackupGenerations}`.
- Produces (exact, later tasks depend on these names):
```go
type SaveSummary struct { Filename string; SizeKB float64; Modified int64 }
type Color struct { A, R, G, B uint64 }
type CharacterView struct {
    Class, ClassName, Name, HeadAsset, SkinAsset string
    Level uint64
    Colors []Color
}
type SaveView struct { Filename string; Character CharacterView }
type Session struct { /* cfg, store, adb — unexported */ }
func New(cfg *platform.Config) *Session
func (s *Session) ListSaves() ([]SaveSummary, error)
func (s *Session) OpenSave(filename string) (*SaveView, error)
```
`OpenSave` rejects unless filename is basename-equal (`filepath.Base(filename) == filename`), 12 chars, `Save` prefix, `.sav` suffix, digits in `[4:8]` (mirrors `editor.validSaveName` + traversal guard). `CharacterView` maps 1:1 from `ExtractCharacterInfo` keys `class`, `class_name`, `name`, `level`, `head_asset`, `skin_asset`, `appearance_colors[{a,r,g,b}]`.

- [ ] **Step 1: Write failing tests** in `session_test.go` (fixture pattern: `t.TempDir()` + copy `../../tests/Save0001.sav`; in-memory `&platform.Config{SaveDir: dir, BackupGenerations: 5}` — never touch real config):
```go
func TestListSavesFindsOne(t *testing.T)      // len==1, [0].Filename=="Save0001.sav"
func TestListSavesEmptyDir(t *testing.T)      // empty TempDir → 0 saves, nil error (Review Focus 2)
func TestOpenSaveCharacter(t *testing.T)      // Class/ClassName/Name non-empty, Level>=1, len(Colors)==3
func TestOpenSaveRejectsTraversal(t *testing.T) // "evil.sav" and "../../etc/passwd" → error (Review Focus 5)
func TestOpenSaveCorruptReturnsError(t *testing.T) // truncated copy of fixture → error, no panic (Review Focus 3)
func TestNewNilConfigShowsSetup(t *testing.T) // New(&platform.Config{}) → ListSaves errors mentioning setup, no nil deref (Review Focus 1)
```
- [ ] **Step 2: Run tests, verify they fail.** Run: `go test ./native/session/ -v`. Expected: FAIL (package undefined).
- [ ] **Step 3: Implement `session.go`** per Interfaces above. `New` builds store with `loadoutDir = filepath.Join(saveDir, "loadouts")` only when configured; nil/empty config yields a session whose methods return setup errors.
- [ ] **Step 4: Run tests, verify pass.** Run: `go test ./native/session/ -v` + `go vet ./native/...`. Expected: all PASS.
- [ ] **Step 5: Commit.**
```bash
git add desktop/native/session/
git commit -m "feat(native): headless session core with typed view-models"
```

### Task 3: Asset-dir probe + default location

**Files:**
- Create: `desktop/native/setup/setup.go`
- Create: `desktop/native/setup/setup_test.go`

**Interfaces:**
- Consumes: `platform.ConfigPath() string`.
- Produces (exact):
```go
type AssetStatus struct {
    Dir string
    ModelCount, TextureCount, MaterialCount int
    OK bool
    Missing []string
}
func DefaultAssetDir() string  // filepath.Join(filepath.Dir(platform.ConfigPath()), "bl2models"), overridden by $BL2_NATIVE_ASSETS
func ProbeAssetDir(dir string) AssetStatus
```
`OK` rule (slice 0): dir exists AND `ModelCount > 0` (any `.gltf`/`.glb` recursively). `Missing` names the absent classes (`"models"`, `"textures"`, `"materials"` when zero). Slice 3 refines with a manifest — do not build one here.

- [ ] **Step 1: Write failing tests:**
```go
func TestProbeMissingDir(t *testing.T)   // nonexistent dir → OK==false, Missing contains "models" (Review Focus 4)
func TestProbeEmptyDir(t *testing.T)     // empty TempDir → OK==false
func TestProbeFindsModel(t *testing.T)   // TempDir + one dummy "w.gltf" → ModelCount==1, OK==true
func TestDefaultAssetDirRespectsEnv(t *testing.T) // t.Setenv("BL2_NATIVE_ASSETS", dir) → equal
```
- [ ] **Step 2: Run, verify fail.** Run: `go test ./native/setup/ -v`. Expected: FAIL.
- [ ] **Step 3: Implement `setup.go`.** Walk dir with `filepath.WalkDir`; count by extension (`.gltf`/`.glb`, `.png`/`.jpg`, `.json`).
- [ ] **Step 4: Run, verify pass.** Run: `go test ./native/setup/ -v` + `go vet ./native/...`. Expected: PASS.
- [ ] **Step 5: Commit.**
```bash
git add desktop/native/setup/
git commit -m "feat(native): asset-dir probe and default location"
```

### Task 4: Spike shell — window, save list, one glTF

**Files:**
- Create: `desktop/native/ui/shell.go`
- Wire: `desktop/cmd/bl2native/main.go` (replace stubs with real calls)

**Interfaces:**
- Consumes: `session.Session` (Task 2), `setup.AssetStatus` (Task 3).
- Produces: `func Run(ses *session.Session, st setup.AssetStatus) error` — opens 1500x950 g3n window "BL2 Save Editor"; HSplit: left `gui.List` of `ses.ListSaves()` filenames (empty state label when none / setup CTA label when ses reports unconfigured); right pane: if `!st.OK`, CTA label naming `st.Dir` + missing classes; else load first model file found under `st.Dir` with the pinned g3n version's `loader/gltf` API (resolve exact func from that version's godoc at implementation time; record choice in a code comment) + `camera.NewOrbitControl`.

- [ ] **Step 1: Implement `shell.go` + wire `main.go`.** No headless test possible (GL context); keep GL code isolated in this file only.
- [ ] **Step 2: Vet.** Run: `go vet ./...` from `desktop/`. Expected: clean.
- [ ] **Step 3: Manual verification runbook** (do all three, record results in commit message):
  1. `make dev-native` with valid config + real `Save0001.sav` → window opens, save listed, model renders, orbit drag works.
  2. Point `$BL2_NATIVE_ASSETS` at empty dir → CTA shown, app stays alive.
  3. Empty save dir → empty-state label, no error.
- [ ] **Step 4: Commit.**
```bash
git add desktop/native/ui/ desktop/cmd/bl2native/
git commit -m "feat(native): slice-0 spike shell with save list and glTF view"
```

## Deferred (explicitly NOT in this plan)

MVP tabs, character/inventory editing, texture compositing port, Gestalt sets, modelpipe Go port, setup wizard, Gibbed UI, loadouts UI, menus/dialogs, Wails retirement, README updates. Each gets its own slice plan after slice 0 validates the g3n pin.
