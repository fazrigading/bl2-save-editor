// Command bl2native is the native g3n-based BL2 save editor.
// Slice 0: scaffold — reports configuration status; UI wiring lands in Task 4.
package main

import (
	"fmt"
	"os"

	"bl2save/desktop/internal/platform"
)

func main() {
	cfg := platform.Load()
	if cfg == nil || !cfg.Valid() {
		fmt.Println("bl2native: not configured — complete setup first")
		return
	}
	fmt.Println("bl2native: configured, save dir:", cfg.SaveDir)
	os.Exit(0)
}
