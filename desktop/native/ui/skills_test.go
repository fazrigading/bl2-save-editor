package ui

import (
	"testing"

	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/internal/skills"
)

func TestSkillProjection(t *testing.T) {
	groups := []skills.TreeGroup{{
		Name:  "Guerrilla",
		Color: "#ff4444",
		Skills: []skills.Skill{
			{Path: "GD_Soldier_Skills.Guerrilla.Sentry", Name: "Sentry", Max: 5},
			{Path: "GD_Soldier_Skills.Guerrilla.Other", Name: "Other", Max: 3},
		},
	}}
	cols := skillRows(groups, map[string]uint64{
		"GD_Soldier_Skills.Guerrilla.Sentry": 3,
		"GD_Soldier_Skills.Guerrilla.Other":  9, // over max → clamp for display
	})
	if len(cols) != 1 || len(cols[0]) != 2 {
		t.Fatalf("want 1 col / 2 rows, got %d / %d", len(cols), len(cols[0]))
	}
	if cols[0][0].Cur != 3 || cols[0][0].Max != 5 || cols[0][0].AtMax {
		t.Fatalf("Sentry projection wrong: %+v", cols[0][0])
	}
	if cols[0][1].Cur != 3 || !cols[0][1].AtMax {
		t.Fatalf("over-max row should clamp to max/AtMax: %+v", cols[0][1])
	}
	if opts := optionsFor(cols[0][0]); len(opts) != 6 || opts[0] != "0" || opts[5] != "5" {
		t.Fatalf("options for max 5: %v", opts)
	}
}

func TestSkillStepperDisabled(t *testing.T) {
	p := &skillsPanel{}
	sel := widget.NewSelect([]string{"0", "1"}, nil)
	p.selects = append(p.selects, sel)
	p.setDisabled(true)
	if !sel.Disabled() {
		t.Fatal("stepper should be disabled")
	}
	p.setDisabled(false)
	if sel.Disabled() {
		t.Fatal("stepper should be re-enabled")
	}
}
