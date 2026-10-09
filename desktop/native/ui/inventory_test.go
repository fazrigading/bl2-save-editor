package ui

import "testing"

// Headless-safe: pure helpers only (no GL/panels).

func TestParseHexColor(t *testing.T) {
	c := parseHexColor("#e8a33a")
	if c.R < 0.9 || c.G < 0.6 || c.G > 0.7 || c.B < 0.2 || c.B > 0.25 {
		t.Fatalf("legendary gold parsed wrong: %+v", c)
	}
	gray := parseHexColor("nope")
	if gray.R != 0.6 || gray.G != 0.6 || gray.B != 0.6 {
		t.Fatalf("bad input should yield gray: %+v", gray)
	}
	if e := parseHexColor(""); e != gray {
		t.Fatal("empty input should yield gray")
	}
}

func TestInvCatsFields(t *testing.T) {
	want := map[string]int{"weapons": 54, "items": 53, "bank": 41}
	cats := invCats()
	if len(cats) != 3 {
		t.Fatalf("want 3 categories, got %d", len(cats))
	}
	for _, c := range cats {
		if want[c.key] != c.field {
			t.Fatalf("category %s: want field %d, got %d", c.key, want[c.key], c.field)
		}
	}
}

func TestElementColorsKnown(t *testing.T) {
	for _, name := range []string{"Fire", "Shock", "Corrosive", "Slag", "Explosive"} {
		ec, ok := elementColors()[name]
		if !ok {
			t.Fatalf("missing element color %s", name)
		}
		c := parseHexColor(ec)
		if c.R == 0.6 && c.G == 0.6 && c.B == 0.6 && ec != "#9d9d9d" {
			t.Fatalf("element %s has unparseable color %s", name, ec)
		}
	}
}
