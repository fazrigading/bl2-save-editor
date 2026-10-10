// Package skills loads the bundled Borderlands 2 skill-tree data
// (static/skill_trees.json, committed) for the SKILLS tab. Pure data:
// no UI, no session dependency.
package skills

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed skill_trees.json
var raw []byte

// Skill is one skill-tree entry.
type Skill struct {
	Path, Name, Desc, Tree string
	Tier, Col, Max         int
	Color                  string
}

// TreeGroup is one named tree (3 per class) with its skills.
type TreeGroup struct {
	Name, Color string
	Skills      []Skill
}

var (
	parsed     = map[string]classDef{}
	treeOrders = map[string][]string{}
)

type classDef struct {
	Trees map[string]treeDef `json:"trees"`
}

type treeDef struct {
	Name   string     `json:"name"`
	Color  string     `json:"color"`
	Skills []skillDef `json:"skills"`
}

type skillDef struct {
	Path, Name, Desc string
	Tier, Col, Max   int
}

func init() {
	_ = json.Unmarshal(raw, &parsed)

	// Tree order: encoding/json maps are unordered, so re-decode each
	// class's "trees" object keys with a streaming decoder.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return
	}
	for classID, body := range top {
		var wrap struct {
			Trees json.RawMessage `json:"trees"`
		}
		if err := json.Unmarshal(body, &wrap); err != nil || wrap.Trees == nil {
			continue
		}
		treeOrders[classID] = objectKeys(wrap.Trees)
	}
}

// objectKeys returns the top-level keys of a JSON object in document order.
func objectKeys(obj json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(obj))
	if _, err := dec.Token(); err != nil { // opening '{'
		return nil
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return keys
		}
		if k, ok := tok.(string); ok {
			keys = append(keys, k)
		}
		if err := skipValue(dec); err != nil {
			return keys
		}
	}
	return keys
}

// skipValue consumes one JSON value (scalar or nested container).
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil // scalar
	}
	if d != '{' && d != '[' {
		return nil
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case json.Delim:
			if t == '{' || t == '[' {
				depth++
			} else {
				depth--
			}
		}
	}
	return nil
}

// Trees returns the three tree groups for a class id (e.g. "GD_Soldier").
// Unknown class ids return an error.
func Trees(classID string) ([]TreeGroup, error) {
	cf, ok := parsed[classID]
	if !ok {
		return nil, fmt.Errorf("unknown class %q", classID)
	}
	groups := make([]TreeGroup, 0, len(cf.Trees))
	for _, name := range treeOrders[classID] {
		t, ok := cf.Trees[name]
		if !ok {
			continue
		}
		g := TreeGroup{Name: t.Name, Color: t.Color}
		for _, s := range t.Skills {
			g.Skills = append(g.Skills, Skill{
				Path: s.Path, Name: s.Name, Desc: s.Desc, Tree: t.Name,
				Tier: s.Tier, Col: s.Col, Max: s.Max, Color: t.Color,
			})
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// MaxFor returns the max points for a skill path (0 if unknown).
func MaxFor(groups []TreeGroup, path string) int {
	for _, g := range groups {
		for _, s := range g.Skills {
			if s.Path == path {
				return s.Max
			}
		}
	}
	return 0
}
