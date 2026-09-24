package platform

import (
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	gameMu       sync.Mutex
	gameCached   bool
	gameCachedAt time.Time
)

// IsGameRunning reports whether Borderlands 2 is running, cached for 3
// seconds to avoid spawning process listings on rapid calls.
func IsGameRunning() bool {
	gameMu.Lock()
	defer gameMu.Unlock()
	if time.Since(gameCachedAt) < 3*time.Second {
		return gameCached
	}
	running := checkGameRunning()
	gameCached = running
	gameCachedAt = time.Now()
	return running
}

func checkGameRunning() bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq Borderlands2.exe")
	} else {
		cmd = exec.Command("pgrep", "-f", "Borderlands2")
	}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.Contains(string(out), "Borderlands2.exe")
	}
	return len(strings.TrimSpace(string(out))) > 0
}
