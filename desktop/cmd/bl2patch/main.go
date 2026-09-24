// Command bl2patch patches Borderlands2.exe to raise the manufacturer
// grade_index cap from 80 to 127. Port of patch_bl2.py.
package main

import (
	"fmt"
	"os"

	"bl2save/desktop/internal/platform"
)

// Patch locations: (offset of the 0x50 byte, description).
// Clamping sites clamp grade_index to 80 on item deserialization;
// validation sites reject items with grade > 80.
var patches = []struct {
	Offset int64
	Desc   string
}{
	{0x78c554, "clamp1_cmp"},
	{0x78c558, "clamp1_mov"},
	{0x78d02b, "clamp2_cmp"},
	{0x78d02f, "clamp2_mov"},
	{0x78d29b, "clamp3_cmp"},
	{0x78d29f, "clamp3_mov"},
	{0xa46c84, "validate1"},
	{0xad03f3, "validate2"},
	{0xad0522, "validate3"},
	{0xad1e31, "validate4"},
}

const (
	oldByte = 0x50 // 80
	newByte = 0x7F // 127
)

func fail(format string, args ...any) {
	fmt.Printf("ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	exePath := ""
	unpatch := false
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--unpatch":
			unpatch = true
		case "--help", "-h":
			fmt.Println("Usage: bl2patch [--unpatch] [path/to/Borderlands2.exe]")
			fmt.Println("Raises the manufacturer grade_index cap from 80 to 127.")
			fmt.Println("Without an explicit path, exe_path is read from config.json.")
			return
		default:
			exePath = args[i]
		}
	}

	if exePath == "" {
		exePath = platform.Load().ExePath
	}
	if exePath == "" {
		fail("Cannot find Borderlands2.exe. Pass the path explicitly or set exe_path in config.json.")
	}
	backupPath := exePath + ".backup"

	if unpatch {
		unpatchExe(exePath, backupPath)
		return
	}
	patch(exePath, backupPath)
}

func patch(exePath, backupPath string) {
	if _, err := os.Stat(exePath); err != nil {
		fail("Cannot find %s", exePath)
	}
	data, err := os.ReadFile(exePath)
	if err != nil {
		fail("read: %v", err)
	}
	fmt.Printf("Loaded exe: %d bytes\n", len(data))

	alreadyPatched := true
	for _, p := range patches {
		if data[p.Offset] != newByte {
			alreadyPatched = false
			break
		}
	}
	if alreadyPatched {
		fmt.Printf("Already patched! All %d locations are set to 127.\n", len(patches))
		return
	}

	for _, p := range patches {
		actual := data[p.Offset]
		switch actual {
		case oldByte:
			// expected
		case newByte:
			fmt.Printf("  %s at 0x%08x: already patched\n", p.Desc, p.Offset)
		default:
			fail("%s at 0x%08x: expected 0x%02x, got 0x%02x\nExe version mismatch. Aborting.",
				p.Desc, p.Offset, oldByte, actual)
		}
	}

	if _, err := os.Stat(backupPath); err != nil {
		fmt.Printf("Creating backup: %s\n", backupPath)
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			fail("backup write: %v", err)
		}
	} else {
		fmt.Printf("Backup already exists: %s\n", backupPath)
	}

	for _, p := range patches {
		if data[p.Offset] == oldByte {
			data[p.Offset] = newByte
			fmt.Printf("  Patched %s at 0x%08x: 0x50 -> 0x7F\n", p.Desc, p.Offset)
		}
	}
	if err := os.WriteFile(exePath, data, 0o644); err != nil {
		fail("write: %v", err)
	}
	patchedCount := 0
	for _, p := range patches {
		if data[p.Offset] == newByte {
			patchedCount++
		}
	}
	fmt.Printf("\nPatched exe written (%d bytes)\n", len(data))
	fmt.Printf("Applied %d/%d patches.\n", patchedCount, len(patches))
	fmt.Println("Grade index cap raised from 80 to 127.")
	fmt.Println()
	fmt.Println("IMPORTANT: Also enable the GradeBypass PythonSDK mod, which:")
	fmt.Println("  1) Raises MaxExperienceLevel to 127 (unlocks GameStage scaling)")
	fmt.Println("  2) Extends Grades[] arrays so grade >80 produces real stats")
}

func unpatchExe(exePath, backupPath string) {
	if _, err := os.Stat(backupPath); err != nil {
		fail("No backup found. Cannot unpatch.")
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		fail("read backup: %v", err)
	}
	if err := os.WriteFile(exePath, data, 0o644); err != nil {
		fail("restore: %v", err)
	}
	fmt.Println("Restored original exe from backup.")
}
