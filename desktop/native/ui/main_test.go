package ui

import (
	"os"
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestMain starts a headless Fyne test app. Widgets that build renderers
// (widget.Select.SetSelected, Refresh) call fyne.CurrentApp() and panic
// with no app running.
func TestMain(m *testing.M) {
	test.NewApp()
	os.Exit(m.Run())
}
