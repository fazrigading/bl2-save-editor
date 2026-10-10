package ui

import (
	"strings"
	"testing"
	"time"

	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
)

func TestOpenTwiceSecondFails(t *testing.T) {
	old := runViewer
	release := make(chan struct{})
	runViewer = func(ses *session.Session, st setup.AssetStatus, item session.ItemView, onClose func()) {
		// Block until the test releases: proves Open returns while the
		// viewer runs and the second Open sees the busy flag.
		<-release
		if onClose != nil {
			onClose()
		}
	}
	defer func() { runViewer = old }()

	b := &viewerBridge{}
	status1 := []string{}
	if !b.Open(nil, setup.AssetStatus{}, session.ItemView{}, func(s string) { status1 = append(status1, s) }) {
		t.Fatal("first open should succeed")
	}
	if len(status1) == 0 || status1[0] != "3D viewer open" {
		t.Fatalf("want '3D viewer open', got %v", status1)
	}
	status2 := []string{}
	if b.Open(nil, setup.AssetStatus{}, session.ItemView{}, func(s string) { status2 = append(status2, s) }) {
		t.Fatal("second open before close should fail")
	}
	if len(status2) < 1 || status2[0] != "3D viewer already open" {
		t.Fatalf("want '3D viewer already open', got %v", status2)
	}
	close(release)
	waitClosed(t, b)
	if !b.Open(nil, setup.AssetStatus{}, session.ItemView{}, func(string) {}) {
		t.Fatal("open after close should succeed")
	}
}

func TestOpenRecoversViewerPanic(t *testing.T) {
	old := runViewer
	runViewer = func(ses *session.Session, st setup.AssetStatus, item session.ItemView, onClose func()) {
		panic("glfw boom")
	}
	defer func() { runViewer = old }()

	b := &viewerBridge{}
	var msgs []string
	if !b.Open(nil, setup.AssetStatus{}, session.ItemView{}, func(s string) { msgs = append(msgs, s) }) {
		t.Fatal("open should succeed (failure surfaces async)")
	}
	waitClosed(t, b)
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "failed") {
		t.Fatalf("want a failure status line, got %v", msgs)
	}
	old2 := runViewer
	release := make(chan struct{})
	runViewer = func(ses *session.Session, st setup.AssetStatus, item session.ItemView, onClose func()) {
		close(release)
		if onClose != nil {
			onClose()
		}
	}
	defer func() { runViewer = old2 }()
	if !b.Open(nil, setup.AssetStatus{}, session.ItemView{}, func(string) {}) {
		t.Fatal("bridge should be reusable after panic")
	}
	<-release
}

func waitClosed(t *testing.T, b *viewerBridge) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.mu.Lock()
		open := b.open
		b.mu.Unlock()
		if !open {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("bridge stuck open")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
