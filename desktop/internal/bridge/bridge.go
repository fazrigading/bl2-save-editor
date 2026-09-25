// Package bridge exposes the Go backend to the Wails frontend. Invoke
// replicates the Flask API surface (method + path + JSON body in, JSON out)
// so the existing frontend needs only a transport swap.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/editor"
	"bl2save/desktop/internal/platform"
)

type Bridge struct {
	ctx context.Context
	cfg *platform.Config

	store *editor.Store
	adb   *assets.DB

	mu           sync.Mutex
	previewCache map[string]previewEntry

	cfgMu      sync.Mutex
	gibbedBusy atomic.Bool
}

type previewEntry struct {
	mtime int64
	data  map[string]any
}

func New() *Bridge {
	return &Bridge{previewCache: map[string]previewEntry{}}
}

func (b *Bridge) Startup(ctx context.Context) {
	b.ctx = ctx
	b.ReloadConfig()
}

// ReloadConfig (re)loads config.json and rebuilds the store. When the config
// is valid but Gibbed data is not configured, the dumps are downloaded
// automatically in the background.
func (b *Bridge) ReloadConfig() {
	b.cfg = platform.Load()
	b.adb = assets.New(b.cfg.GibbedDir)
	b.store = editor.NewStore(b.cfg.SaveDir, b.cfg.BackupGenerations,
		filepath.Join(filepath.Dir(platform.ConfigPath()), "loadouts"))
	if b.ctx != nil && b.cfg.Valid() && b.cfg.GibbedDir == "" {
		b.startGibbedDownload()
	}
}

// startGibbedDownload downloads the Gibbed data dumps into the managed
// directory (unless already running) and updates the config when finished.
func (b *Bridge) startGibbedDownload() {
	if b.ctx == nil || !b.gibbedBusy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		dir := platform.GibbedDataDir()
		err := platform.DownloadGibbedData(dir, func(done, total int, name string) {
			runtime.EventsEmit(b.ctx, "gibbed:progress", map[string]any{
				"done": done, "total": total, "name": name,
			})
		})
		if err != nil {
			runtime.EventsEmit(b.ctx, "gibbed:error", map[string]any{"error": err.Error()})
			b.gibbedBusy.Store(false)
			return
		}
		b.cfgMu.Lock()
		if b.cfg != nil && b.cfg.GibbedDir == "" {
			b.cfg.GibbedDir = dir
			_ = b.cfg.Save()
		}
		b.cfgMu.Unlock()
		b.ReloadConfig()
		runtime.EventsEmit(b.ctx, "gibbed:done", map[string]any{"dir": dir})
		b.gibbedBusy.Store(false)
	}()
}

// SelectFolder opens a native directory picker (used by the setup screen).
func (b *Bridge) SelectFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{Title: title})
}

// OpenPath opens a file or directory in the platform file manager.
func (b *Bridge) OpenPath(path string) error {
	return platform.OpenPath(path)
}

// Configured reports whether a valid config is loaded.
func (b *Bridge) Configured() bool {
	return b.cfg != nil && b.cfg.Valid()
}

// apiError mirrors an HTTP error response.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func errBad(status int, format string, args ...any) *apiError {
	return &apiError{status: status, msg: fmt.Sprintf(format, args...)}
}

func (b *Bridge) requireStore() (*editor.Store, *apiError) {
	if b.store == nil || b.cfg == nil || !b.cfg.Valid() {
		return nil, errBad(400, "Editor is not configured. Complete the setup first.")
	}
	return b.store, nil
}

func guardGame() *apiError {
	if platform.IsGameRunning() {
		return errBad(409, "Borderlands 2 is running. Close the game before editing saves.")
	}
	return nil
}

func validSaveName(filename string) bool {
	if len(filename) != 12 || !strings.HasPrefix(filename, "Save") || !strings.HasSuffix(filename, ".sav") {
		return false
	}
	for _, c := range filename[4:8] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func requireSaveName(filename string) *apiError {
	if !validSaveName(filename) {
		return errBad(400, "Invalid save filename")
	}
	return nil
}

var validItemFields = map[int]bool{41: true, 53: true, 54: true}

// Invoke routes a frontend API call. Error responses resolve as
// {"error": ...} payloads so the frontend's existing handling applies.
func (b *Bridge) Invoke(method, rawurl, body string) (string, error) {
	method = strings.ToUpper(method)
	u, err := url.Parse(rawurl)
	if err != nil {
		return apiErrJSON(errBad(400, "invalid path")), nil
	}
	var payload any
	if body != "" {
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return apiErrJSON(errBad(400, "invalid JSON body: %v", err)), nil
		}
	}
	result, apiErr := b.dispatch(method, u.Path, u.Query(), payload)
	if apiErr != nil {
		return apiErrJSON(apiErr), nil
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal response: %w", err)
	}
	return string(out), nil
}

func apiErrJSON(e *apiError) string {
	msg, _ := json.Marshal(e.msg)
	return fmt.Sprintf(`{"error":%s,"status":%d}`, msg, e.status)
}

type handlerFunc func(b *Bridge, p params, payload any, q url.Values) (any, *apiError)

type params map[string]string

func (p params) int(name string) (int, *apiError) {
	v, err := atoiSafe(p[name])
	if err != nil {
		return 0, errBad(400, "invalid %s", name)
	}
	return v, nil
}

func atoiSafe(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("bad digit")
		}
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

type route struct {
	method  string
	segs    []string
	tail    bool
	handler handlerFunc
}

// routes mirrors the Flask route table from app.py.
var routes = []route{
	{method: "GET", segs: []string{"api", "setup", "detect"}, handler: (*Bridge).hDetect},
	{method: "POST", segs: []string{"api", "setup", "save"}, handler: (*Bridge).hSetupSave},
	{method: "POST", segs: []string{"api", "setup", "download-gibbed"}, handler: (*Bridge).hDownloadGibbed},
	{method: "GET", segs: []string{"api", "configured"}, handler: (*Bridge).hConfigured},
	{method: "GET", segs: []string{"api", "config", "paths"}, handler: (*Bridge).hConfigPaths},

	{method: "GET", segs: []string{"api", "saves"}, handler: (*Bridge).hListSaves},
	{method: "GET", segs: []string{"api", "saves", "previews"}, handler: (*Bridge).hSavePreviews},
	{method: "GET", segs: []string{"api", "game-status"}, handler: (*Bridge).hGameStatus},

	{method: "GET", segs: []string{"api", "save", "{filename}"}, handler: (*Bridge).hLoadSave},
	{method: "POST", segs: []string{"api", "save", "{filename}", "duplicate"}, handler: wrapSaveName((*Bridge).hDuplicateSave)},
	{method: "DELETE", segs: []string{"api", "save", "{filename}", "delete"}, handler: wrapSaveName((*Bridge).hDeleteSave)},
	{method: "GET", segs: []string{"api", "save", "{filename}", "backups"}, handler: wrapSaveName((*Bridge).hListBackups)},
	{method: "POST", segs: []string{"api", "save", "{filename}", "backups", "restore"}, handler: wrapSaveName(wrapGame((*Bridge).hRestoreBackup))},

	{method: "POST", segs: []string{"api", "save", "{filename}", "character"}, handler: wrapSaveName(wrapGame((*Bridge).hUpdateCharacter))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "skills"}, handler: wrapSaveName(wrapGame((*Bridge).hSetSkills))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "ammo"}, handler: wrapSaveName(wrapGame((*Bridge).hSetAmmo))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "ammo", "fill"}, handler: wrapSaveName(wrapGame((*Bridge).hFillAmmo))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "playthrough"}, handler: wrapSaveName(wrapGame((*Bridge).hPlaythrough))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "unlock-achievements"}, handler: wrapSaveName(wrapGame((*Bridge).hUnlockAchievements))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "spawn-test-weapons"}, handler: wrapSaveName(wrapGame((*Bridge).hSpawnTestWeapons))},

	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "complete-all"}, handler: wrapSaveName(wrapGame((*Bridge).hCompleteAllMissions))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "add"}, handler: wrapSaveName(wrapGame((*Bridge).hAddMission))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "remove"}, handler: wrapSaveName(wrapGame((*Bridge).hRemoveMission))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "add-all-story"}, handler: wrapSaveName(wrapGame((*Bridge).hAddAllStory))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "set-active"}, handler: wrapSaveName(wrapGame((*Bridge).hSetActiveMission))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "missions", "{pt}", "{*mission}"}, handler: wrapSaveName(wrapGame((*Bridge).hSetMissionStatus))},
	{method: "GET", segs: []string{"api", "missions", "db"}, handler: (*Bridge).hMissionDB},

	{method: "POST", segs: []string{"api", "save", "{filename}", "fast-travel", "unlock-all"}, handler: wrapSaveName(wrapGame((*Bridge).hUnlockAllFastTravel))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "fast-travel"}, handler: wrapSaveName(wrapGame((*Bridge).hUpdateFastTravel))},
	{method: "GET", segs: []string{"api", "fast-travel", "all-stations"}, handler: (*Bridge).hAllStations},

	{method: "GET", segs: []string{"api", "save", "{filename}", "challenges"}, handler: wrapSaveName((*Bridge).hGetChallenges)},
	{method: "POST", segs: []string{"api", "save", "{filename}", "challenges", "update"}, handler: wrapSaveName(wrapGame((*Bridge).hSetChallenge))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "challenges", "complete-all"}, handler: wrapSaveName(wrapGame((*Bridge).hCompleteAllChallenges))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "challenges", "reset-all"}, handler: wrapSaveName(wrapGame((*Bridge).hResetAllChallenges))},

	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "add"}, handler: wrapSaveName(wrapGame((*Bridge).hAddWeapon))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "add-item"}, handler: wrapSaveName(wrapGame((*Bridge).hAddItem))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "reorder"}, handler: wrapSaveName(wrapGame((*Bridge).hReorderItem))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "bulk-level"}, handler: wrapSaveName(wrapGame((*Bridge).hBulkLevel))},
	{method: "DELETE", segs: []string{"api", "save", "{filename}", "items", "{field}", "{idx}"}, handler: wrapSaveName(wrapGame((*Bridge).hDeleteItem))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "{idx}", "level"}, handler: wrapSaveName(wrapGame((*Bridge).hSetItemLevel))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "{idx}", "duplicate"}, handler: wrapSaveName(wrapGame((*Bridge).hDuplicateItem))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "{idx}", "transfer"}, handler: wrapSaveName(wrapGame((*Bridge).hTransferItem))},
	{method: "POST", segs: []string{"api", "save", "{filename}", "items", "{field}", "{idx}", "edit"}, handler: wrapSaveName(wrapGame((*Bridge).hEditItem))},
	{method: "GET", segs: []string{"api", "save", "{filename}", "export", "{field}", "{idx}"}, handler: wrapSaveName((*Bridge).hExportCode)},
	{method: "GET", segs: []string{"api", "save", "{filename}", "export-all"}, handler: wrapSaveName((*Bridge).hExportAll)},
	{method: "POST", segs: []string{"api", "save", "{filename}", "import"}, handler: wrapSaveName(wrapGame((*Bridge).hImportCodes))},
	{method: "POST", segs: []string{"api", "preview-codes"}, handler: (*Bridge).hPreviewCodes},

	{method: "GET", segs: []string{"api", "assets", "weapon-types"}, handler: (*Bridge).hWeaponTypes},
	{method: "GET", segs: []string{"api", "assets", "balances"}, handler: (*Bridge).hBalances},
	{method: "GET", segs: []string{"api", "assets", "parts"}, handler: (*Bridge).hParts},
	{method: "GET", segs: []string{"api", "assets", "item-categories"}, handler: (*Bridge).hItemCategories},
	{method: "GET", segs: []string{"api", "assets", "item-balances"}, handler: (*Bridge).hItemBalances},
	{method: "GET", segs: []string{"api", "assets", "item-parts"}, handler: (*Bridge).hItemParts},
	{method: "GET", segs: []string{"api", "assets", "all-parts", "{slot}"}, handler: (*Bridge).hAllPartsForSlot},
	{method: "GET", segs: []string{"api", "assets", "all-parts-batch"}, handler: (*Bridge).hAllPartsBatch},
	{method: "GET", segs: []string{"api", "assets", "all-balances"}, handler: (*Bridge).hAllBalances},
	{method: "GET", segs: []string{"api", "assets", "manufacturers"}, handler: (*Bridge).hManufacturers},
	{method: "GET", segs: []string{"api", "assets", "customizations", "{class}"}, handler: (*Bridge).hCustomizations},

	{method: "GET", segs: []string{"api", "loadouts"}, handler: (*Bridge).hListLoadouts},
	{method: "POST", segs: []string{"api", "loadouts", "save"}, handler: (*Bridge).hSaveLoadout},
	{method: "POST", segs: []string{"api", "loadouts", "{file}", "restore"}, handler: wrapGame((*Bridge).hRestoreLoadout)},
	{method: "DELETE", segs: []string{"api", "loadouts", "{file}"}, handler: (*Bridge).hDeleteLoadout},

	{method: "GET", segs: []string{"api", "steam", "status"}, handler: (*Bridge).hSteamStatus},
	{method: "POST", segs: []string{"api", "steam", "init"}, handler: (*Bridge).hSteamInit},
	{method: "GET", segs: []string{"api", "steam", "achievements"}, handler: (*Bridge).hSteamAchievements},
	{method: "POST", segs: []string{"api", "steam", "achievements", "unlock"}, handler: (*Bridge).hSteamUnlock},
	{method: "POST", segs: []string{"api", "steam", "achievements", "unlock-all"}, handler: (*Bridge).hSteamUnlockAll},
	{method: "POST", segs: []string{"api", "steam", "achievements", "clear"}, handler: (*Bridge).hSteamClear},
}

func wrapGame(h handlerFunc) handlerFunc {
	return func(b *Bridge, p params, payload any, q url.Values) (any, *apiError) {
		if e := guardGame(); e != nil {
			return nil, e
		}
		return h(b, p, payload, q)
	}
}

func wrapSaveName(h handlerFunc) handlerFunc {
	return func(b *Bridge, p params, payload any, q url.Values) (any, *apiError) {
		if e := requireSaveName(p["filename"]); e != nil {
			return nil, e
		}
		return h(b, p, payload, q)
	}
}

func (b *Bridge) dispatch(method, path string, q url.Values, payload any) (any, *apiError) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil, errBad(404, "not found")
	}
	segs := strings.Split(trimmed, "/")
	var matched *route
	var bestParams params
	bestScore := -1
	for i := range routes {
		r := &routes[i]
		if r.method != method {
			continue
		}
		p, score, ok := matchRoute(r, segs)
		if ok && score > bestScore {
			matched = r
			bestParams = p
			bestScore = score
		}
	}
	if matched == nil {
		return nil, errBad(404, "not found: %s %s", method, path)
	}
	return matched.handler(b, bestParams, payload, q)
}

func matchRoute(r *route, segs []string) (params, int, bool) {
	p := params{}
	score := 0
	idx := 0
	for ri, rs := range r.segs {
		if strings.HasPrefix(rs, "{*") && strings.HasSuffix(rs, "}") {
			name := rs[2 : len(rs)-1]
			if idx >= len(segs) {
				return nil, 0, false
			}
			p[name] = strings.Join(segs[idx:], "/")
			score += len(segs) - idx
			idx = len(segs)
			_ = ri
			if ri != len(r.segs)-1 {
				return nil, 0, false
			}
			return p, score, true
		}
		if idx >= len(segs) {
			return nil, 0, false
		}
		if strings.HasPrefix(rs, "{") && strings.HasSuffix(rs, "}") {
			p[rs[1:len(rs)-1]] = segs[idx]
			score++
		} else {
			if rs != segs[idx] {
				return nil, 0, false
			}
			score += 2
		}
		idx++
	}
	if idx != len(segs) {
		return nil, 0, false
	}
	return p, score, true
}

func (b *Bridge) hConfigured(_ params, _ any, _ url.Values) (any, *apiError) {
	return map[string]any{"configured": b.Configured()}, nil
}

// CurrentPaths returns the configured paths for menus and the frontend.
func (b *Bridge) CurrentPaths() map[string]any {
	cfg := b.currentConfig()
	return map[string]any{
		"save_dir":    cfg.SaveDir,
		"gibbed_dir":  cfg.GibbedDir,
		"game_dir":    cfg.GameDir,
		"config_path": platform.ConfigPath(),
	}
}

func (b *Bridge) currentConfig() *platform.Config {
	if b.cfg == nil {
		b.cfg = platform.Load()
	}
	return b.cfg
}

func (b *Bridge) hConfigPaths(_ params, _ any, _ url.Values) (any, *apiError) {
	return b.CurrentPaths(), nil
}

func (b *Bridge) hDownloadGibbed(_ params, _ any, _ url.Values) (any, *apiError) {
	if b.ctx == nil {
		return nil, errBad(500, "app not started")
	}
	dir := platform.GibbedDataDir()
	missing := len(platform.GibbedMissing(dir))
	if missing == 0 {
		return map[string]any{"ok": true, "dir": dir, "missing": 0}, nil
	}
	b.startGibbedDownload()
	return map[string]any{"ok": true, "dir": dir, "missing": missing}, nil
}

func (b *Bridge) hDetect(_ params, _ any, _ url.Values) (any, *apiError) {
	current := platform.Load()
	saveDir := platform.DetectSaveDir()
	if current.SaveDir != "" {
		saveDir = current.SaveDir
	}
	gibbedDir := platform.DetectGibbedDir()
	if current.GibbedDir != "" {
		gibbedDir = current.GibbedDir
	}
	gameDir := platform.DetectSteamDir()
	if current.GameDir != "" {
		gameDir = current.GameDir
	}
	return map[string]any{
		"save_dir":   saveDir,
		"game_dir":   gameDir,
		"gibbed_dir": gibbedDir,
		"current": map[string]any{
			"save_dir":           current.SaveDir,
			"gibbed_dir":         current.GibbedDir,
			"game_dir":           current.GameDir,
			"backup_generations": current.BackupGenerations,
		},
	}, nil
}

func (b *Bridge) hSetupSave(_ params, payload any, _ url.Values) (any, *apiError) {
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, errBad(400, "expected JSON object")
	}
	str := func(key string) string {
		if v, ok := obj[key].(string); ok {
			return v
		}
		return ""
	}
	backupGens := pInt(obj, "backup_generations", 5)
	if backupGens < 1 {
		backupGens = 1
	}
	if backupGens > 20 {
		backupGens = 20
	}
	cfg := &platform.Config{
		SaveDir:           str("save_dir"),
		GibbedDir:         str("gibbed_dir"),
		GameDir:           str("game_dir"),
		BackupGenerations: backupGens,
	}
	if cfg.SaveDir == "" {
		return nil, errBad(400, "Save directory not found. Please check the path.")
	}
	if fi, err := os.Stat(cfg.SaveDir); err != nil || !fi.IsDir() {
		return nil, errBad(400, "Save directory not found. Please check the path.")
	}
	if cfg.ExePath == "" && cfg.GameDir != "" {
		cfg.ExePath = platform.DeriveExePath(cfg.GameDir)
	}
	if err := cfg.Save(); err != nil {
		return nil, errBad(500, "failed to write config: %v", err)
	}
	b.ReloadConfig()
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hListSaves(_ params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	saves, err := store.ListSaves()
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return saves, nil
}

func (b *Bridge) hGameStatus(_ params, _ any, _ url.Values) (any, *apiError) {
	return map[string]any{"running": platform.IsGameRunning()}, nil
}

func (b *Bridge) hDuplicateSave(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	newName, err := store.DuplicateSave(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "new_filename": newName}, nil
}

func (b *Bridge) hDeleteSave(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	ok, err := store.DeleteSave(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Save not found")
	}
	b.mu.Lock()
	delete(b.previewCache, p["filename"])
	b.mu.Unlock()
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hListBackups(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	backups, err := store.ListBackups(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "backups": backups}, nil
}

func (b *Bridge) hRestoreBackup(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	gen := 0
	if obj, ok := payload.(map[string]any); ok {
		gen = int(toFloat(obj["generation"]))
	}
	ok, err := store.RestoreBackup(p["filename"], gen)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Backup not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hSavePreviews(_ params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	saves, err := store.ListSaves()
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	results := map[string]any{}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range saves {
		fname := s["filename"].(string)
		mtime := s["modified"].(int64)
		if cached, ok := b.previewCache[fname]; ok && cached.mtime == mtime {
			results[fname] = cached.data
			continue
		}
		preview := map[string]any{"class_name": "?", "level": 0}
		if _, tree, err := store.ReadSave(fname); err == nil {
			preview = map[string]any{
				"class_name": classNameFromTree(tree),
				"level":      getUint64(tree, 2, 1),
			}
		}
		b.previewCache[fname] = previewEntry{mtime: mtime, data: preview}
		results[fname] = preview
	}
	return results, nil
}

func (b *Bridge) hLoadSave(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	_, tree, err := store.ReadSave(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	charInfo := store.ExtractCharacterInfo(tree)
	inventory := store.ExtractInventory(tree)
	for _, category := range []string{"weapons", "items", "bank"} {
		items, _ := inventory[category].([]map[string]any)
		for _, item := range items {
			if info, ok := item["info"].(map[string]any); ok {
				item["resolved"] = b.adb.ResolveItemParts(info)
			}
		}
	}
	return map[string]any{
		"character":   charInfo,
		"inventory":   inventory,
		"missions":    store.ExtractMissions(tree),
		"fast_travel": store.ExtractFastTravel(tree),
		"challenges":  store.ExtractChallenges(tree),
	}, nil
}
