// Package bridge exposes the Go backend to the Wails frontend. Invoke replicates
// the Flask API surface (method + path + JSON body in, JSON out) so the existing
// frontend needs only a transport swap.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bl2save/desktop/internal/platform"
)

type Bridge struct {
	ctx context.Context
	cfg *platform.Config
}

func New() *Bridge {
	return &Bridge{}
}

func (b *Bridge) Startup(ctx context.Context) {
	b.ctx = ctx
	b.cfg = platform.Load()
}

// SelectFolder opens a native directory picker (used by the setup screen).
func (b *Bridge) SelectFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{Title: title})
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

// Invoke routes a frontend API call. method is GET/POST/DELETE, url is the
// original Flask-style path (with optional query string), body is a JSON
// string or empty. Always resolves with a JSON string; error responses are
// {"error": ...} payloads so the frontend's existing error handling applies.
func (b *Bridge) Invoke(method, rawurl, body string) (string, error) {
	method = strings.ToUpper(method)
	u, err := url.Parse(rawurl)
	if err != nil {
		return apiErrJSON(errBad(400, "invalid path")), nil
	}
	path := u.Path
	q := u.Query()

	handler, ok := routes[method+" "+path]
	if !ok {
		return apiErrJSON(errBad(404, "not found: %s %s", method, path)), nil
	}
	var payload any
	if body != "" {
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return apiErrJSON(errBad(400, "invalid JSON body: %v", err)), nil
		}
	}
	result, apiErr := handler(b, payload, q)
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

type handlerFunc func(b *Bridge, payload any, q url.Values) (any, *apiError)

var routes = map[string]handlerFunc{
	"GET /api/setup/detect": (*Bridge).hDetect,
	"POST /api/setup/save":  (*Bridge).hSetupSave,
}

func (b *Bridge) hDetect(_ any, _ url.Values) (any, *apiError) {
	cfg := b.currentConfig()
	return map[string]any{
		"save_dir":   platform.DetectSaveDir(),
		"gibbed_dir": platform.DetectGibbedDir(),
		"game_dir":   platform.DetectSteamDir(),
		"exe_path":   cfg.ExePath,
		"current": map[string]any{
			"save_dir":   cfg.SaveDir,
			"gibbed_dir": cfg.GibbedDir,
			"game_dir":   cfg.GameDir,
		},
	}, nil
}

func (b *Bridge) hSetupSave(payload any, _ url.Values) (any, *apiError) {
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
	cfg := &platform.Config{
		SaveDir:           str("save_dir"),
		GibbedDir:         str("gibbed_dir"),
		GameDir:           str("game_dir"),
		BackupGenerations: intFrom(obj["backup_generations"], 5),
	}
	if cfg.SaveDir == "" {
		return nil, errBad(400, "save_dir is required")
	}
	if !dirExists(cfg.SaveDir) {
		return nil, errBad(400, "save_dir does not exist: %s", cfg.SaveDir)
	}
	if cfg.ExePath == "" && cfg.GameDir != "" {
		cfg.ExePath = platform.DeriveExePath(cfg.GameDir)
	}
	if err := cfg.Save(); err != nil {
		return nil, errBad(500, "failed to write config: %v", err)
	}
	b.cfg = cfg
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) currentConfig() *platform.Config {
	if b.cfg == nil {
		b.cfg = platform.Load()
	}
	return b.cfg
}

func intFrom(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return def
	}
}

func dirExists(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
