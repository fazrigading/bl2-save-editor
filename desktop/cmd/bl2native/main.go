// Command bl2native is the native g3n-based BL2 save editor.
package main

import (
	"fmt"
	"os"

	"bl2save/desktop/internal/platform"
	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
	"bl2save/desktop/native/ui"
)

func main() {
	cfg := platform.Load()
	ses := session.New(cfg)
	st := setup.ProbeAssetDir(setup.DefaultAssetDir())
	if cfg == nil || !cfg.Valid() {
		fmt.Println("bl2native: not configured — complete setup first")
	}
	if err := ui.Run(ses, st); err != nil {
		fmt.Fprintln(os.Stderr, "bl2native:", err)
		os.Exit(1)
	}
}
