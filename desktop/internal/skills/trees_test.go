package skills

import "testing"

func TestTreesKnownClass(t *testing.T) {
	gs, err := Trees("GD_Soldier")
	if err != nil {
		t.Fatalf("Trees(GD_Soldier): %v", err)
	}
	if len(gs) != 3 {
		t.Fatalf("want 3 trees, got %d", len(gs))
	}
	want := []string{"Guerrilla", "Gunpowder", "Survival"}
	for i, w := range want {
		if gs[i].Name != w {
			t.Fatalf("tree %d: want %q, got %q", i, w, gs[i].Name)
		}
	}
	first := gs[0].Skills[0]
	if first.Path != "GD_Soldier_Skills.Guerrilla.Sentry" {
		t.Fatalf("first skill path: got %q", first.Path)
	}
	if got := MaxFor(gs, "GD_Soldier_Skills.Guerrilla.Sentry"); got != 5 {
		t.Fatalf("Sentry max: want 5, got %d", got)
	}
}

func TestTreesUnknown(t *testing.T) {
	if _, err := Trees("GD_Nope"); err == nil {
		t.Fatal("unknown class should error")
	}
}

func TestMaxForUnknownPath(t *testing.T) {
	if got := MaxFor(nil, "nope"); got != 0 {
		t.Fatalf("unknown path max: want 0, got %d", got)
	}
}

func TestAllClassesLoad(t *testing.T) {
	for _, id := range []string{"GD_Soldier", "GD_Assassin", "GD_Siren", "GD_Mercenary", "GD_Tulip", "GD_Lilac"} {
		gs, err := Trees(id)
		if err != nil {
			t.Fatalf("Trees(%s): %v", id, err)
		}
		if len(gs) != 3 {
			t.Fatalf("%s: want 3 trees, got %d", id, len(gs))
		}
		for _, g := range gs {
			if len(g.Skills) == 0 {
				t.Fatalf("%s/%s: no skills", id, g.Name)
			}
		}
	}
}
