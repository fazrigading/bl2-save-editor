package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"bl2save/desktop/internal/platform"
	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
	"bl2save/desktop/viewer"
)

// viewerBridge owns the single g3n viewer window: Open launches it on a
// goroutine (non-blocking); closing it never quits the app. Window position
// is remembered per run in viewer_window.json beside config.json.
type viewerBridge struct {
	mu   sync.Mutex
	open bool
}

// runViewer opens the g3n window (package var so tests can stub the GL
// path — g3n windows cannot open headless).
var runViewer = viewer.Run

// Open starts the viewer window for the given item (zero value = character
// mode). Returns false (and a status line) when already open.
func (b *viewerBridge) Open(ses *session.Session, st setup.AssetStatus, item session.ItemView, status func(string)) bool {
	b.mu.Lock()
	if b.open {
		b.mu.Unlock()
		if status != nil {
			status("3D viewer already open")
		}
		return false
	}
	b.open = true
	b.mu.Unlock()

	go func() {
		// A g3n panic (second Run on the singleton app, dead GL) must
		// not kill the Fyne process: report it and free the bridge.
		defer func() {
			if r := recover(); r != nil {
				b.mu.Lock()
				b.open = false
				b.mu.Unlock()
				if status != nil {
					status(fmt.Sprintf("3D viewer failed: %v", r))
				}
			}
		}()
		onClose := func() {
			b.mu.Lock()
			b.open = false
			b.mu.Unlock()
			saveViewerPos()
			if status != nil {
				status("3D viewer closed")
			}
		}
		restoreViewerPos()
		runViewer(ses, st, item, onClose)
	}()
	if status != nil {
		status("3D viewer open")
	}
	return true
}


func viewerPosPath() string {
	return filepath.Join(filepath.Dir(platform.ConfigPath()), "viewer_window.json")
}

// saveViewerPos writes the g3n window position next to config.json. Best
// effort: failures are silent (position memory is a nicety).
func saveViewerPos() {
	if viewer.LastViewerPos == nil {
		return
	}
	data, err := json.Marshal(viewer.LastViewerPos)
	if err != nil {
		return
	}
	_ = os.WriteFile(viewerPosPath(), data, 0o644)
}

// restoreViewerPos applies a saved position to the next g3n window, if the
// file parses. Missing/corrupt file = default position, silently.
func restoreViewerPos() {
	data, err := os.ReadFile(viewerPosPath())
	if err != nil {
		return
	}
	var p viewer.ViewerPos
	if err := json.Unmarshal(data, &p); err != nil {
		return
	}
	viewer.SetPendingViewerPos(&p)
}
