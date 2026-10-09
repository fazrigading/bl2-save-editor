package ui

import (
	"encoding/json"
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
		viewer.Run(ses, st, item, onClose)
	}()
	if status != nil {
		status("3D viewer open")
	}
	return true
}

// openForTest exposes the open-state machine without launching a GL window:
// the returned func runs the same lock/flag/status logic and captures onClose.
func (b *viewerBridge) openForTest(capture func(func())) func(*session.Session, setup.AssetStatus, session.ItemView, func(string)) bool {
	return func(ses *session.Session, st setup.AssetStatus, item session.ItemView, status func(string)) bool {
		b.mu.Lock()
		if b.open {
			b.mu.Unlock()
			if status != nil {
				status("3D viewer already open")
			}
			return false
		}
		b.open = true
		onClose := func() {
			b.mu.Lock()
			b.open = false
			b.mu.Unlock()
			if status != nil {
				status("3D viewer closed")
			}
		}
		b.mu.Unlock()
		if capture != nil {
			capture(onClose)
		}
		if status != nil {
			status("3D viewer open")
		}
		return true
	}
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
