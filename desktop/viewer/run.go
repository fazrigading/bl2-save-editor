// Package viewer owns the g3n 3D window: character model display and item
// preview proxies. Extracted from native/ui (slice 1) so the Fyne shell can
// launch it as a companion window. g3n-gui widget chrome is gone; only the
// engine scene, camera, and lights remain.
package viewer

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/loader/gltf"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/window"

	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
)

// viewerCtl swaps the 3D scene between the character model and the item
// preview proxy. Preview meshes are flat-PBR proxies from mesh.go, never
// the extracted assets.
type viewerCtl struct {
	scene     *core.Node
	charNode  core.INode
	charShown bool
	preview   core.INode
	mode      *gui.Label
}

// showPreview drops the character model and shows the item proxy.
func (v *viewerCtl) showPreview(item session.ItemView) {
	if v.preview != nil {
		v.scene.Remove(v.preview)
		v.preview = nil
	}
	if v.charNode != nil && v.charShown {
		v.scene.Remove(v.charNode)
		v.charShown = false
	}
	v.preview = previewMesh(item)
	v.scene.Add(v.preview)
	v.mode.SetText("preview: " + item.DisplayName)
}

// showCharacter drops the preview and restores the character model.
func (v *viewerCtl) showCharacter() {
	if v.preview != nil {
		v.scene.Remove(v.preview)
		v.preview = nil
	}
	if v.charNode != nil && !v.charShown {
		v.scene.Add(v.charNode)
		v.charShown = true
	}
	v.mode.SetText("character")
}

// windowTitle renders the chrome status string chain as a window title so
// the headless test can assert setup messaging without opening GL.
func windowTitle(st setup.AssetStatus) string {
	if !st.OK {
		return "3D viewer — assets missing in " + st.Dir +
			" — run setup to extract UModel assets"
	}
	if st.ModelCount == 0 {
		return "3D viewer — No .gltf/.glb in " + st.Dir +
			" — run setup to extract UModel assets"
	}
	return "3D viewer"
}

// Viewer position hooks let the UI bridge persist/restore the g3n window
// position without importing g3n there.
var (
	LastViewerPos    *ViewerPos
	pendingViewerPos *ViewerPos
)

// ViewerPos is a screen position for the g3n window.
type ViewerPos struct {
	X, Y int
}

// SetPendingViewerPos restores a saved position on the next Run.
func SetPendingViewerPos(p *ViewerPos) { pendingViewerPos = p }

// Run opens the g3n window and blocks (like g3n app.Run). A zero-value item
// selects character mode. onClose fires when the window closes.
func Run(ses *session.Session, st setup.AssetStatus, item session.ItemView, onClose func()) {
	defer func() {
		if onClose != nil {
			onClose()
		}
	}()
	a := app.App()
	if gw, ok := a.IWindow.(*window.GlfwWindow); ok {
		if pendingViewerPos != nil {
			gw.SetPos(pendingViewerPos.X, pendingViewerPos.Y)
			pendingViewerPos = nil
		}
		// Record the last position for the bridge to persist on close.
		defer func() {
			x, y := gw.GetPos()
			LastViewerPos = &ViewerPos{X: x, Y: y}
		}()
	}
	scene := core.NewNode()
	gui.Manager().Set(scene)

	vc := &viewerCtl{scene: scene}
	vc.mode = gui.NewLabel("character")
	bar := gui.NewPanel(600, 40)
	bar.SetLayout(gui.NewHBoxLayout())
	bar.Add(vc.mode)
	backBtn := gui.NewButton("Back to character")
	bar.Add(backBtn)
	backBtn.Subscribe(gui.OnClick, func(string, interface{}) { vc.showCharacter() })
	scene.Add(bar)

	if !st.OK {
		log.Printf("viewer: assets missing in %s — run setup to extract UModel assets", st.Dir)
	} else if path := firstModel(st.Dir); path == "" {
		log.Printf("viewer: no .gltf/.glb in %s — run setup to extract UModel assets", st.Dir)
	} else if g, err := parseModel(path); err != nil {
		log.Printf("viewer: model load failed: %v", err)
	} else if node, err := g.LoadScene(0); err != nil {
		log.Printf("viewer: model load failed: %v", err)
	} else {
		vc.charNode = node
		scene.Add(node)
		vc.charShown = true
		log.Printf("viewer: rendering model %s", path)
	}
	if item.DisplayName != "" {
		vc.showPreview(item)
	}

	// Camera + lights.
	width, height := a.GetSize()
	cam := camera.New(float32(width) / float32(height))
	cam.SetPosition(0, 0, 3)
	scene.Add(cam)
	camera.NewOrbitControl(cam)
	scene.Add(light.NewAmbient(&math32.Color{R: 1, G: 1, B: 1}, 0.8))
	pl := light.NewPoint(&math32.Color{R: 1, G: 1, B: 1}, 5)
	pl.SetPosition(1, 0, 2)
	scene.Add(pl)

	a.SubscribeID(window.OnWindowSize, vc, func(evname string, ev interface{}) {
		w, h := a.GetSize()
		a.Gls().Viewport(0, 0, int32(w), int32(h))
		cam.SetAspect(float32(w) / float32(h))
	})
	// The app singleton survives window close: drop this run's resize
	// handler so relaunches never accumulate handlers over dead cameras.
	defer a.UnsubscribeID(window.OnWindowSize, vc)
	a.Gls().ClearColor(0.04, 0.05, 0.07, 1)
	a.Run(func(rend *renderer.Renderer, _ time.Duration) {
		a.Gls().Clear(gls.COLOR_BUFFER_BIT | gls.DEPTH_BUFFER_BIT)
		rend.Render(scene, cam)
	})
}

// firstModel returns the first .gltf/.glb file under dir.
func firstModel(dir string) string {
	var found string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".gltf", ".glb":
			found = path
		}
		return nil
	})
	return found
}

// parseModel parses .glb via ParseBin and anything else via ParseJSON.
func parseModel(path string) (*gltf.GLTF, error) {
	if strings.EqualFold(filepath.Ext(path), ".glb") {
		return gltf.ParseBin(path)
	}
	return gltf.ParseJSON(path)
}
