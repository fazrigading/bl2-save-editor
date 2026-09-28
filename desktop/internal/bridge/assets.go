package bridge

import (
	"errors"
	"strings"
	"time"
)

// Assets serves the Gibbed item/weapon database queries.
type Assets struct{ b *Bridge }

func (s *Assets) WeaponTypes() (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetWeaponCategories(), nil
}

func (s *Assets) Balances(weaponType string) (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetBalancesForType(weaponType), nil
}

func (s *Assets) Parts(balance string) (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetPartsForBalance(balance), nil
}

func (s *Assets) ItemCategories() (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetItemCategories(), nil
}

func (s *Assets) ItemBalances(category string) (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetItemBalancesForCategory(category), nil
}

func (s *Assets) ItemParts(balance string) (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetItemPartsForBalance(balance), nil
}

func (s *Assets) AllPartsForSlot(slot string, kind string) (any, error) {
	s.b.adb.EnsureLoaded()
	if kind == "item" {
		return s.b.adb.GetAllItemPartsForSlot(slot), nil
	}
	return s.b.adb.GetAllPartsForSlot(slot), nil
}

func (s *Assets) AllPartsBatch(kind string, slots string) (map[string]any, error) {
	defer slowLog("Assets.AllPartsBatch", time.Now())
	s.b.adb.EnsureLoaded()
	if slots == "" {
		return nil, errors.New("slots required")
	}
	result := map[string]any{}
	for _, slot := range strings.Split(slots, ",") {
		slot = strings.TrimSpace(slot)
		if slot == "" {
			continue
		}
		if kind == "item" {
			result[slot] = s.b.adb.GetAllItemPartsForSlot(slot)
		} else {
			result[slot] = s.b.adb.GetAllPartsForSlot(slot)
		}
	}
	return result, nil
}

func (s *Assets) AllBalances() (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetAllBalances(), nil
}

func (s *Assets) Manufacturers() (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.GetAllManufacturers(), nil
}

func (s *Assets) Customizations(class string) (any, error) {
	s.b.adb.EnsureLoaded()
	return s.b.adb.Customizations(class), nil
}
