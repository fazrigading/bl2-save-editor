package ui

import (
	"testing"

	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
)

func TestOpenTwiceSecondFails(t *testing.T) {
	b := &viewerBridge{}
	var onClose func()
	// Open records the onClose so the test can fire it.
	open := b.openForTest(func(fn func()) { onClose = fn })

	status1 := []string{}
	if !open(nil, setup.AssetStatus{}, session.ItemView{}, func(s string) { status1 = append(status1, s) }) {
		t.Fatal("first open should succeed")
	}
	if len(status1) == 0 || status1[0] != "3D viewer open" {
		t.Fatalf("want '3D viewer open', got %v", status1)
	}
	status2 := []string{}
	if open(nil, setup.AssetStatus{}, session.ItemView{}, func(s string) { status2 = append(status2, s) }) {
		t.Fatal("second open before close should fail")
	}
	if len(status2) < 1 || status2[0] != "3D viewer already open" {
		t.Fatalf("want '3D viewer already open', got %v", status2)
	}
	if onClose == nil {
		t.Fatal("onClose not captured")
	}
	onClose()
	if !open(nil, setup.AssetStatus{}, session.ItemView{}, func(string) {}) {
		t.Fatal("open after close should succeed")
	}
}
