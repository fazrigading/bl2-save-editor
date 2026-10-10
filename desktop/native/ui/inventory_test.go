package ui

import (
	"testing"

	"bl2save/desktop/native/session"
)

// Headless-safe: pure helpers only (no GL/panels).

func TestParseHexColor(t *testing.T) {
	c := RarityColor("Legendary")
	if c.R != 0xe8 || c.G != 0xa3 || c.B != 0x3a {
		t.Fatalf("legendary gold resolved wrong: %+v", c)
	}
	gray := RarityColor("nope")
	if gray.R != 0x9d || gray.G != 0x9d || gray.B != 0x9d {
		t.Fatalf("bad input should yield common gray: %+v", gray)
	}
	if e := RarityColor(""); e != gray {
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

func TestInventoryTableData(t *testing.T) {
	sv := &session.SaveView{
		Filename: "Save0001.sav",
		Inventory: map[string][]session.ItemView{
			"weapons": {{
				Index: 0, Field: 54,
				DisplayName: "Unkempt Harold", RarityName: "Legendary",
				RarityColor: "#e8a33a", Level: 50,
				Category: "Combat Rifle", ElementName: "Incendiary",
			}},
			"bank": {{Index: 0, Field: 41, DisplayName: "Bank Thing", Level: 1}},
		},
	}
	d := inventoryTableData(sv, "Weapons")
	if len(d.rows) != 1 || d.cols != invTableCols {
		t.Fatalf("want 1 weapon / %d cols, got %d / %d", invTableCols, len(d.rows), d.cols)
	}
	if hex := raritySwatchHex(d.rows[0]); hex != "#e8a33a" {
		t.Fatalf("want #e8a33a, got %s", hex)
	}
	if b := inventoryTableData(sv, "bank"); len(b.rows) != 1 {
		t.Fatalf("bank row should be separate, got %d", len(b.rows))
	}
	if u := inventoryTableData(sv, "nope"); len(u.rows) != 0 {
		t.Fatalf("unknown cat should be empty, got %d", len(u.rows))
	}
	if n := inventoryTableData(nil, "Weapons"); len(n.rows) != 0 {
		t.Fatalf("nil save should be empty, got %d", len(n.rows))
	}
}

func TestRaritySwatchFallback(t *testing.T) {
	if hex := raritySwatchHex(session.ItemView{}); hex != "#9d9d9d" {
		t.Fatalf("empty rarity color should fall back to common gray, got %s", hex)
	}
}

func TestInvActionsFoldMap(t *testing.T) {
	p := &invPanel{
		cats:  invCats(),
		items: map[string][]session.ItemView{},
	}
	inv := map[string][]session.ItemView{
		"weapons": {{DisplayName: "A"}},
		"items":   {{DisplayName: "B"}, {DisplayName: "C"}},
		"bank":    {{DisplayName: "D"}},
	}
	p.applyInventory(inv)
	for _, c := range p.cats {
		if len(p.items[c.key]) != len(inv[c.key]) {
			t.Fatalf("cat %s: want %d rows, got %d", c.key, len(inv[c.key]), len(p.items[c.key]))
		}
	}
}
