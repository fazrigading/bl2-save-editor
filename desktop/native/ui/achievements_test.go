package ui

import (
	"testing"

	"bl2save/desktop/native/session"
)

func TestAchievementListRenders(t *testing.T) {
	tr := true
	rows := []session.AchievementRow{
		{APIName: "A1", Name: "First", Desc: "d1", Category: "story", CategoryName: "Story"},
		{APIName: "A2", Name: "Second", Desc: "d2", Category: "story", CategoryName: "Story"},
		{APIName: "A3", Name: "Third", Desc: "d3", Category: "combat", CategoryName: "Combat", Unlocked: &tr},
	}
	if n := achievementCount(rows, false); n != 3 {
		t.Fatalf("want 3 rows, got %d", n)
	}
	if obj := achievementList(rows, false); obj == nil {
		t.Fatal("list should not be nil")
	}
	// All nil-unlocked rows are unaffected by the hide filter.
	nilRows := []session.AchievementRow{
		{Name: "X", CategoryName: "Story"},
		{Name: "Y", CategoryName: "Combat"},
	}
	if n := achievementCount(nilRows, true); n != 2 {
		t.Fatalf("nil-unlocked rows should not be filtered, got %d", n)
	}
	// An unlocked row IS filtered when hideUnlocked.
	if n := achievementCount(rows, true); n != 2 {
		t.Fatalf("hideUnlocked should drop the unlocked row, got %d", n)
	}
}

func TestAchievementGlyphStates(t *testing.T) {
	tr, fa := true, false
	if g := achievementGlyph(session.AchievementRow{}); g != "—" {
		t.Fatalf("nil unlocked should render —, got %q", g)
	}
	if g := achievementGlyph(session.AchievementRow{Unlocked: &fa}); g != "·" {
		t.Fatalf("locked should render ·, got %q", g)
	}
	if g := achievementGlyph(session.AchievementRow{Unlocked: &tr}); g != "✓" {
		t.Fatalf("unlocked should render ✓, got %q", g)
	}
}

func TestAchievementsErrorState(t *testing.T) {
	obj := newAchievementsPanel(nil, &session.SaveView{Filename: "Save0001.sav"}, func() bool { return true })
	if obj == nil {
		t.Fatal("nil-session panel should still render an error state")
	}
}
