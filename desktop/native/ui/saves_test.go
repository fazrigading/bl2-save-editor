package ui

import (
	"strings"
	"testing"

	"bl2save/desktop/native/session"
)

func TestSaveRowsRoundTrip(t *testing.T) {
	sums := []session.SaveSummary{
		{Filename: "Save0001.sav", SizeKB: 12.3, Modified: 1700000000},
		{Filename: "Save0002.sav", SizeKB: 98.7, Modified: 1700000000},
	}
	rows := saveRows(sums)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	for i, row := range rows {
		filename, ok := parseSaveRow(row)
		if !ok || filename != sums[i].Filename {
			t.Fatalf("row %d %q: want %q ok, got %q ok=%v", i, row, sums[i].Filename, filename, ok)
		}
	}
	if _, ok := parseSaveRow("garbage row"); ok {
		t.Fatal("garbage row should not parse")
	}
	if _, ok := parseSaveRow(""); ok {
		t.Fatal("empty row should not parse")
	}
	if !strings.Contains(rows[0], "Save0001.sav") {
		t.Fatalf("row should contain filename: %q", rows[0])
	}
}
