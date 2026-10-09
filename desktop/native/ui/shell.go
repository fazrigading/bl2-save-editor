// Package ui owns the g3n window. Slice 0 spike: HSplit shell with the
// save list on the left and a single-glTF viewer (or setup CTA) on the
// right. g3n loader API used here: gltf.ParseJSON + LoadScene(0)
// (engine v0.2.0).
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

// Run opens the editor window. Click-to-open and tabs land in slice 1;
// the spike auto-opens the first save to prove session wiring.
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

	// Right: viewer or setup CTA.
	if !st.OK {
		split.P1.Add(gui.NewLabel("3D assets missing in " + st.Dir +
			" — run setup to extract UModel assets"))
	} else if len(saves) == 0 || listErr != nil {
		split.P1.Add(gui.NewLabel("Select a save to preview"))
	} else {
		sv, err := ses.OpenSave(saves[0].Filename)
		if err != nil {
			split.P1.Add(gui.NewLabel("Open failed: " + err.Error()))
		} else {
			log.Printf("opened %s (%s, level %d)", sv.Filename, sv.Character.ClassName, sv.Character.Level)
			split.P1.Add(gui.NewLabel(sv.Filename + " — " + sv.Character.ClassName))
			if err := showFirstModel(scene, st.Dir); err != nil {
				split.P1.Add(gui.NewLabel("Model load failed: " + err.Error()))
			}
		}
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

// showFirstModel parses the first model in dir and adds scene 0 to the
// scene.
func showFirstModel(scene *core.Node, dir string) error {
	path := firstModel(dir)
	if path == "" {
		return errNoModel
	}
	g, err := gltf.ParseJSON(path)
	if err != nil {
		return err
	}
	node, err := g.LoadScene(0)
	if err != nil {
		return err
	}
	scene.Add(node)
	log.Printf("rendering model %s", path)
	return nil
}

type modelError string

func (e modelError) Error() string { return string(e) }

const errNoModel = modelError("no .gltf/.glb found")
