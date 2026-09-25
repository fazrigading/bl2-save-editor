package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// GibbedFiles are the data dumps the asset database reads
// (github.com/gibbed/Borderlands2Dumps).
var GibbedFiles = []string{
	"Asset Library Manager.json",
	"Weapon Types.json",
	"Weapon Parts.json",
	"Weapon Balance.json",
	"Weapon Balance Part Lists.json",
	"Weapon Name Parts.json",
	"Weapon Part Lists.json",
	"Items.json",
	"Item Parts.json",
	"Item Balance.json",
}

const gibbedBaseURL = "https://raw.githubusercontent.com/gibbed/Borderlands2Dumps/master/"

// GibbedDataDir is the managed location for auto-downloaded Gibbed data:
// next to config.json.
func GibbedDataDir() string {
	return filepath.Join(filepath.Dir(ConfigPath()), "gibbed_data")
}

// GibbedMissing returns the required dump files not present (or not valid
// JSON) in dir.
func GibbedMissing(dir string) []string {
	var missing []string
	for _, name := range GibbedFiles {
		if !gibbedFileValid(filepath.Join(dir, name)) {
			missing = append(missing, name)
		}
	}
	return missing
}

func gibbedFileValid(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 50 {
		return false
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	return json.Valid(data)
}

var gibbedHTTPClient = &http.Client{Timeout: 120 * time.Second}

// gibbedFetch is swappable for tests.
var gibbedFetch = func(name string) ([]byte, error) {
	resp, err := gibbedHTTPClient.Get(gibbedBaseURL + url.PathEscape(name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, name)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// DownloadGibbedData fetches the missing dump files into dir, validating each
// as JSON before writing. A leading UTF-8 BOM (present in the upstream dumps)
// is stripped so Go's json package accepts the files. progress (may be nil)
// reports each finished file.
func DownloadGibbedData(dir string, progress func(done, total int, name string)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	missing := GibbedMissing(dir)
	total := len(missing)
	for i, name := range missing {
		var data []byte
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			data, err = gibbedFetch(name)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if len(data) < 50 || !json.Valid(data) {
			return fmt.Errorf("%s: downloaded file is not valid JSON", name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if progress != nil {
			progress(i+1, total, name)
		}
	}
	return nil
}
