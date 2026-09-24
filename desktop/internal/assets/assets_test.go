package assets

import "testing"

func TestLevelDamageScale(t *testing.T) {
	if LevelDamageScale(1) != 1.0 {
		t.Fatal("level 1 should be identity")
	}
	if LevelDamageScale(0) != 1.0 {
		t.Fatal("level 0 should be identity")
	}
	prev := 0.0
	for lv := 1; lv <= 80; lv++ {
		s := LevelDamageScale(lv)
		if s <= prev {
			t.Fatalf("scale not monotonic at level %d", lv)
		}
		prev = s
	}
	s72 := LevelDamageScale(72)
	if s72 < 100 || s72 > 50000 {
		t.Fatalf("level 72 scale out of sane range: %v", s72)
	}
}

func TestDetectRarity(t *testing.T) {
	db := New("")
	r := db.DetectRarity("GD_ItemGrades.Balance.5_Legendary_Something")
	if r.Name != "Legendary" {
		t.Fatalf("expected Legendary, got %v", r.Name)
	}
	r = db.DetectRarity("")
	if r.Rank != 0 {
		t.Fatal("empty path should be common")
	}
	r = db.DetectRarity("GD_Weapons.Balance.PlainBalance")
	if r.Name != "Common" {
		t.Fatalf("plain path should be common, got %v", r.Name)
	}
}

func TestDetectCategories(t *testing.T) {
	db := New("")
	if got := db.DetectWeaponCategory("GD_Weapons.SMG.SMG_Default"); got != "SMG" {
		t.Fatalf("weapon category: %v", got)
	}
	if got := db.DetectWeaponCategory(""); got != "Unknown" {
		t.Fatalf("empty weapon category: %v", got)
	}
	if got := db.DetectItemCategory("GD_Shields.ItemType.Shield", ""); got != "Shield" {
		t.Fatalf("item category: %v", got)
	}
}

func TestWeaponStatsShape(t *testing.T) {
	db := New("")
	resolved := map[string]any{
		"category":          "Pistol",
		"manufacturer_name": "Jakobs",
		"level":             []any{float64(72), float64(72)},
		"resolved_parts":    []map[string]any{},
	}
	stats := db.EstimateWeaponStats(resolved)
	if stats == nil {
		t.Fatal("no stats")
	}
	if stats["damage"].(int64) <= 0 {
		t.Fatalf("damage: %v", stats["damage"])
	}
	acc, ok := stats["accuracy"].(float64)
	if !ok || acc < 0 || acc > 100 {
		t.Fatalf("accuracy out of range: %v", stats["accuracy"])
	}
}
