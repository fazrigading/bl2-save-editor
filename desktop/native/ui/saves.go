package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bl2save/desktop/native/session"
)

// saveRows renders one line per save: "Save0001.sav  ·  12.3 KB  ·  3d ago".
func saveRows(sums []session.SaveSummary) []string {
	rows := make([]string, 0, len(sums))
	for _, s := range sums {
		rows = append(rows, fmt.Sprintf("%s  ·  %s  ·  %s",
			s.Filename, formatKB(s.SizeKB), ago(s.Modified)))
	}
	return rows
}

// parseSaveRow is the inverse of saveRows: extracts the filename. Only
// accepts names matching the session save pattern (Save####.sav).
func parseSaveRow(row string) (string, bool) {
	name := strings.TrimSpace(row)
	if i := strings.Index(name, "  ·  "); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if len(name) != 12 || !strings.HasPrefix(name, "Save") ||
		!strings.HasSuffix(name, ".sav") {
		return "", false
	}
	for _, c := range name[4:8] {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return name, true
}

func formatKB(kb float64) string {
	if kb >= 1024 {
		return fmt.Sprintf("%.1f MB", kb/1024)
	}
	return fmt.Sprintf("%.1f KB", kb)
}

// ago renders a coarse relative time like the web save list.
func ago(modified int64) string {
	if modified <= 0 {
		return "?"
	}
	d := time.Since(time.Unix(modified, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

var _ = strconv.Itoa // reserved for level chips in save rows
