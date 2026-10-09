// Package ui owns the g3n window. Slice 1: HSplit shell with the save
// list on the left and (right) always-visible chrome status + tab shell
// (CHARACTER form, INVENTORY tables) + viewer bar (character/preview mode
// switch, carousel prev/next, back-to-character). g3n loader API:
// gltf.ParseJSON + LoadScene(0) (engine v0.2.0).
package ui

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
// preview proxy. Preview meshes are flat-PBR proxies from inventory.go,
// never the extracted assets.
type viewerCtl struct {
	scene     *core.Node
	charNode  core.INode
	charShown bool
	preview   core.INode
	mode      *gui.Label
	inv       *invPanel
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

// Run opens the editor window. The spike auto-opens the first save;
// click-to-open lands in a later slice.
func Run(ses *session.Session, st setup.AssetStatus) error {
	a := app.App()
	scene := core.NewNode()
	gui.Manager().Set(scene)

	width, height := a.GetSize()
	split := gui.NewHSplitter(float32(width), float32(height))
	split.SetSplit(0.25)
	scene.Add(split)

	// Left: save list.
	saves, listErr := ses.ListSaves()
	left := gui.NewVList(200, float32(height))
	if listErr != nil {
		left.Add(gui.NewLabel("Setup required: " + listErr.Error()))
	} else if len(saves) == 0 {
		left.Add(gui.NewLabel("No saves found"))
	} else {
		for _, s := range saves {
			left.Add(gui.NewLabel(s.Filename))
		}
	}
	split.P0.Add(left)

	// Right: chrome status (ALWAYS visible — model errors and setup CTAs
	// live here, never inside a tab pane) + tab shell + viewer bar.
	right := gui.NewPanel(float32(width)*0.75, float32(height))
	right.SetLayout(gui.NewVBoxLayout())
	chrome := gui.NewLabel("")
	right.Add(chrome)
	split.P1.Add(right)

	vc := &viewerCtl{scene: scene}
	vc.mode = gui.NewLabel("character")
	bar := gui.NewPanel(600, 40)
	bar.SetLayout(gui.NewHBoxLayout())
	bar.Add(vc.mode)
	prevBtn := gui.NewButton("Prev")
	nextBtn := gui.NewButton("Next")
	backBtn := gui.NewButton("Back to character")
	bar.Add(prevBtn)
	bar.Add(nextBtn)
	bar.Add(backBtn)
	prevBtn.Subscribe(gui.OnClick, func(string, interface{}) {
		if vc.inv != nil {
			vc.inv.stepCarousel(-1)
		}
	})
	nextBtn.Subscribe(gui.OnClick, func(string, interface{}) {
		if vc.inv != nil {
			vc.inv.stepCarousel(1)
		}
	})
	backBtn.Subscribe(gui.OnClick, func(string, interface{}) { vc.showCharacter() })

	if listErr == nil && len(saves) > 0 {
		sv, err := ses.OpenSave(saves[0].Filename)
		if err != nil {
			chrome.SetText("Open failed: " + err.Error())
		} else {
			log.Printf("opened %s (%s, level %d)", sv.Filename, sv.Character.ClassName, sv.Character.Level)
			tb := buildTabs(float32(width)*0.75, float32(height))
			tb.TabAt(tabCharacter).SetContent(newCharacterPanel(ses, sv))
			invRoot, inv := newInventoryPanel(ses, sv, vc.showPreview)
			vc.inv = inv
			tb.TabAt(tabInventory).SetContent(invRoot)
			right.Add(tb)
			right.Add(bar)
			if !st.OK {
				chrome.SetText("3D assets missing in " + st.Dir +
					" — run setup to extract UModel assets")
			} else if path := firstModel(st.Dir); path == "" {
				chrome.SetText("No .gltf/.glb in " + st.Dir +
					" — run setup to extract UModel assets")
			} else if g, err := parseModel(path); err != nil {
				chrome.SetText("Model load failed: " + err.Error())
			} else if node, err := g.LoadScene(0); err != nil {
				chrome.SetText("Model load failed: " + err.Error())
			} else {
				vc.charNode = node
				scene.Add(node)
				vc.charShown = true
				log.Printf("rendering model %s", path)
			}
		}
	} else if listErr != nil {
		chrome.SetText("Setup required: " + listErr.Error())
	} else {
		chrome.SetText("Select a save to preview")
	}

	// Camera + lights.
	cam := camera.New(float32(width) / float32(height))
	cam.SetPosition(0, 0, 3)
	scene.Add(cam)
	camera.NewOrbitControl(cam)
	scene.Add(light.NewAmbient(&math32.Color{R: 1, G: 1, B: 1}, 0.8))
	pl := light.NewPoint(&math32.Color{R: 1, G: 1, B: 1}, 5)
	pl.SetPosition(1, 0, 2)
	scene.Add(pl)

	a.Subscribe(window.OnWindowSize, func(evname string, ev interface{}) {
		w, h := a.GetSize()
		a.Gls().Viewport(0, 0, int32(w), int32(h))
		cam.SetAspect(float32(w) / float32(h))
	})
	a.Gls().ClearColor(0.04, 0.05, 0.07, 1)
	a.Run(func(rend *renderer.Renderer, _ time.Duration) {
		a.Gls().Clear(gls.COLOR_BUFFER_BIT | gls.DEPTH_BUFFER_BIT)
		rend.Render(scene, cam)
	})
	return nil
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
