// Package bridge exposes the Go backend to the Wails frontend as typed
// bound services (App, Saves, Editor, Items, Assets). Frontend calls map
// directly to methods; Go errors surface as promise rejections.
package bridge

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/editor"
	"bl2save/desktop/internal/platform"
)

// Bridge holds the shared backend state. It is not bound to the frontend
// directly — the service structs below are.
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

// Services returns the bound service instances.
func (b *Bridge) Services() (*App, *Saves, *Editor, *Items, *Assets, *Steam) {
	return &App{b}, &Saves{b}, &Editor{b}, &Items{b}, &Assets{b}, &Steam{b}
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

// CurrentPaths returns the configured paths for the native menu and frontend.
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

// payload helpers -------------------------------------------------------------

func pStr(obj map[string]any, key, def string) string {
	if v, ok := obj[key].(string); ok && v != "" {
		return v
	}
	return def
}

func pInt(obj map[string]any, key string, def int) int {
	if v, ok := obj[key].(float64); ok {
		return int(v)
	}
	return def
}

func toFloat(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

// guards ----------------------------------------------------------------------

func (b *Bridge) requireStore() (*editor.Store, error) {
	if b.store == nil || b.cfg == nil || !b.cfg.Valid() {
		return nil, errors.New("Editor is not configured. Complete the setup first.")
	}
	return b.store, nil
}

func guardGame() error {
	if platform.IsGameRunning() {
		return errors.New("Borderlands 2 is running. Close the game before editing saves.")
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

func requireSaveName(filename string) error {
	if !validSaveName(filename) {
		return errors.New("Invalid save filename")
	}
	return nil
}

// deps resolves the store for a save-scoped call and validates the filename.
func (b *Bridge) deps(filename string) (*editor.Store, error) {
	store, err := b.requireStore()
	if err != nil {
		return nil, err
	}
	if err := requireSaveName(filename); err != nil {
		return nil, err
	}
	return store, nil
}

// writeDeps additionally guards against mutating saves while the game runs.
func (b *Bridge) writeDeps(filename string) (*editor.Store, error) {
	store, err := b.deps(filename)
	if err != nil {
		return nil, err
	}
	if err := guardGame(); err != nil {
		return nil, err
	}
	return store, nil
}

// instrumentation --------------------------------------------------------------

// slowLog reports calls slower than 100ms to stderr (visible in wails dev).
func slowLog(name string, start time.Time) {
	if d := time.Since(start); d > 100*time.Millisecond {
		log.Printf("[perf] %s took %s", name, d)
	}
}

// state builds the full save payload handed to the frontend — the same shape
// the old GET /api/save/{filename} returned.
func (b *Bridge) state(filename string) (map[string]any, error) {
	defer slowLog("state("+filename+")", time.Now())
	store, err := b.deps(filename)
	if err != nil {
		return nil, err
	}
	_, tree, err := store.ReadSave(filename)
	if err != nil {
		return nil, err
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

// withState merges the fresh save state into a mutation result so the
// frontend can re-render from a single round trip.
func (b *Bridge) withState(result map[string]any, filename string) (map[string]any, error) {
	state, err := b.state(filename)
	if err != nil {
		return nil, err
	}
	for k, v := range state {
		result[k] = v
	}
	return result, nil
}
