// Package platform provides configuration management and path auto-detection,
// ported from the Python app's config.py.
package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	SaveDir           string `json:"save_dir"`
	GibbedDir         string `json:"gibbed_dir"`
	GameDir           string `json:"game_dir"`
	BackupGenerations int    `json:"backup_generations"`
	ExePath           string `json:"exe_path"`
}

// ConfigPath returns the path of config.json: beside the binary when that
// directory is writable (portable mode), otherwise the user config directory.
func ConfigPath() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if writable(dir) {
			return filepath.Join(dir, "config.json")
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "bl2-save-editor", "config.json")
}

func writable(dir string) bool {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return false
	}
	probe := filepath.Join(dir, ".bl2-write-test")
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}

func detectSteamDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		pf86 := os.Getenv("ProgramFiles(x86)")
		pf := os.Getenv("ProgramFiles")
		if pf86 != "" {
			candidates = append(candidates, filepath.Join(pf86, "Steam", "steamapps", "common", "Borderlands 2"))
		}
		if pf != "" {
			candidates = append(candidates, filepath.Join(pf, "Steam", "steamapps", "common", "Borderlands 2"))
		}
	case "linux":
		candidates = []string{
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "Borderlands 2"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Borderlands 2"),
		}
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Borderlands 2"),
		}
	}
	return firstDir(candidates)
}

func detectSaveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	var docs string
	switch runtime.GOOS {
	case "windows":
		docs = filepath.Join(os.Getenv("USERPROFILE"), "Documents", "My Games", "Borderlands 2", "WillowGame", "SaveData")
	case "linux":
		docs = filepath.Join(home, ".local", "share", "aspyr-media", "borderlands 2", "willowgame", "savedata")
	default:
		return ""
	}
	if fi, err := os.Stat(docs); err != nil || !fi.IsDir() {
		return ""
	}
	entries, err := os.ReadDir(docs)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			return filepath.Join(docs, e.Name())
		}
	}
	return ""
}

func detectGibbedDir() string {
	candidates := []string{
		GibbedDataDir(),
		"gibbed_data",
		filepath.Join("..", "gibbed_data"),
	}
	if runtime.GOOS == "windows" {
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "gibbed_data"))
		}
	}
	return firstDir(candidates)
}

func firstDir(candidates []string) string {
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c
			}
			return abs
		}
	}
	return ""
}

// Exported detection helpers for the setup screen.

func DetectSteamDir() string  { return detectSteamDir() }
func DetectSaveDir() string   { return detectSaveDir() }
func DetectGibbedDir() string { return detectGibbedDir() }

// DeriveExePath returns the game executable path for a given install dir.
func DeriveExePath(gameDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(gameDir, "Binaries", "Win32", "Borderlands2.exe")
	}
	return filepath.Join(gameDir, "Binaries", "Linux", "Borderlands2")
}

// Load reads config.json, auto-detecting missing values (port of config.load_config).
func Load() *Config {
	cfg := &Config{}
	if data, err := os.ReadFile(ConfigPath()); err == nil {
		json.Unmarshal(data, cfg)
	}

	if cfg.SaveDir == "" {
		cfg.SaveDir = detectSaveDir()
	}
	if cfg.GibbedDir == "" {
		cfg.GibbedDir = detectGibbedDir()
	}
	if cfg.GameDir == "" {
		cfg.GameDir = detectSteamDir()
	}
	if cfg.BackupGenerations <= 0 {
		cfg.BackupGenerations = 5
	}
	if cfg.ExePath == "" && cfg.GameDir != "" {
		if runtime.GOOS == "windows" {
			cfg.ExePath = filepath.Join(cfg.GameDir, "Binaries", "Win32", "Borderlands2.exe")
		} else {
			cfg.ExePath = filepath.Join(cfg.GameDir, "Binaries", "Linux", "Borderlands2")
		}
	}
	return cfg
}

// Save writes config.json.
func (c *Config) Save() error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ConfigFileExists reports whether a config.json is present (Python parity:
// the setup wizard shows whenever it is missing, regardless of detection).
func ConfigFileExists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}

// Valid reports whether the config is complete enough to run the editor.
// Mirrors app.py: config.json must exist AND save_dir must be set —
// auto-detected values alone don't count as configured.
func (c *Config) Valid() bool {
	return ConfigFileExists() && c.SaveDir != ""
}
