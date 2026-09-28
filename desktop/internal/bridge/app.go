package bridge

import (
	"errors"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bl2save/desktop/internal/platform"
)

// App serves setup, configuration and app-status calls.
type App struct{ b *Bridge }

func (s *App) Detect() map[string]any {
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
	}
}

func (s *App) SetupSave(payload map[string]any) (map[string]any, error) {
	backupGens := pInt(payload, "backup_generations", 5)
	if backupGens < 1 {
		backupGens = 1
	}
	if backupGens > 20 {
		backupGens = 20
	}
	cfg := &platform.Config{
		SaveDir:           pStr(payload, "save_dir", ""),
		GibbedDir:         pStr(payload, "gibbed_dir", ""),
		GameDir:           pStr(payload, "game_dir", ""),
		BackupGenerations: backupGens,
	}
	if cfg.SaveDir == "" {
		return nil, errors.New("Save directory not found. Please check the path.")
	}
	if fi, err := os.Stat(cfg.SaveDir); err != nil || !fi.IsDir() {
		return nil, errors.New("Save directory not found. Please check the path.")
	}
	if cfg.ExePath == "" && cfg.GameDir != "" {
		cfg.ExePath = platform.DeriveExePath(cfg.GameDir)
	}
	if err := cfg.Save(); err != nil {
		return nil, errors.New("failed to write config: " + err.Error())
	}
	s.b.ReloadConfig()
	return map[string]any{"ok": true}, nil
}

func (s *App) DownloadGibbed() (map[string]any, error) {
	if s.b.ctx == nil {
		return nil, errors.New("app not started")
	}
	dir := platform.GibbedDataDir()
	missing := len(platform.GibbedMissing(dir))
	if missing == 0 {
		return map[string]any{"ok": true, "dir": dir, "missing": 0}, nil
	}
	s.b.startGibbedDownload()
	return map[string]any{"ok": true, "dir": dir, "missing": missing}, nil
}

// Configured reports whether a valid config is loaded.
func (s *App) Configured() bool {
	return s.b.cfg != nil && s.b.cfg.Valid()
}

func (s *App) ConfigPaths() map[string]any {
	return s.b.CurrentPaths()
}

func (s *App) GameStatus() map[string]any {
	return map[string]any{"running": platform.IsGameRunning()}
}

// SelectFolder opens a native directory picker (used by the setup screen).
func (s *App) SelectFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(s.b.ctx, runtime.OpenDialogOptions{Title: title})
}

// OpenPath opens a file or directory in the platform file manager.
func (s *App) OpenPath(path string) error {
	return platform.OpenPath(path)
}
