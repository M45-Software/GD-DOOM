//go:build integration

package dehacked

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestDSDAResolvedDefinitions compares independent native parser output, not a
// Go-generated expected fixture. The native observer exits before gameplay.
func TestDSDAResolvedDefinitions(t *testing.T) {
	native := os.Getenv("GD_DEH_NATIVE")
	if native == "" {
		t.Skip("requires the independent DSDA definition-table observer")
	}
	var paths []string
	if err := json.Unmarshal([]byte(os.Getenv("GD_DEH_PATCHES")), &paths); err != nil {
		t.Fatal(err)
	}
	d := Vanilla()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		patch, err := ParseWithOptions(path, data, ParseOptions{ResolveInclude: func(parent, name string) (Source, error) {
			if !filepath.IsLocal(name) {
				return Source{}, fmt.Errorf("include must be local to the fixture")
			}
			path := filepath.Join(filepath.Dir(parent), name)
			data, err := os.ReadFile(path)
			return Source{path, data}, err
		}})
		if err != nil {
			t.Fatal(err)
		}
		d = mustApply(t, d, patch)
	}
	wanted := map[string][]any{}
	add := func(row []any) {
		// The C observer escapes each legacy byte as a JSON code point. Do
		// the same for Go strings so invalid UTF-8 is not replaced by U+FFFD.
		for i, value := range row {
			if text, ok := value.(string); ok {
				points := make([]rune, len(text))
				for j, b := range []byte(text) {
					points[j] = rune(b)
				}
				row[i] = string(points)
			}
		}
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		var normalized []any
		if err = json.Unmarshal(data, &normalized); err != nil {
			t.Fatal(err)
		}
		wanted[fmt.Sprint(normalized[:2])] = normalized
	}
	structure := func(kind string, index int, value any, skipName bool) {
		row := []any{kind, index}
		v := reflect.ValueOf(value)
		for i := 0; i < v.NumField(); i++ {
			if skipName && i == 0 {
				continue
			}
			row = append(row, v.Field(i).Interface())
		}
		if kind == "state" {
			row[5] = strings.ToUpper(string(value.(State).Action))
		}
		add(row)
	}
	s := d.Snapshot()
	for i, v := range s.Things {
		structure("thing", i, v, true)
		defined := 0
		if d.FieldWasSet("thing", i, "Flags") {
			defined = 1
		}
		add([]any{"defined-bits", i, defined})
	}
	for i, v := range s.States {
		structure("state", i, v, true)
	}
	for i, v := range s.Weapons {
		structure("weapon", i, v, false)
	}
	for i, v := range s.Ammo {
		structure("ammo", i, v, false)
	}
	for i, v := range s.Sounds {
		add([]any{"sound", i, lower(v.Name), v.Priority, v.Pitch})
	}
	for i, v := range s.Sprites {
		add([]any{"sprite", i, lower(v)})
	}
	for i, v := range s.Music {
		if i != 0 {
			add([]any{"music", i, lower(v)})
		}
	}
	for name, seconds := range s.Pars {
		add([]any{"par", name, seconds})
	}
	v := reflect.ValueOf(s.Misc)
	for i := 0; i < v.NumField(); i++ {
		add([]any{"misc", v.Type().Field(i).Name, v.Field(i).Interface()})
	}
	for _, field := range []string{"MaxHealth", "MaxSoulsphere", "MegasphereHealth", "BFGCellsPerShot"} {
		defined := 0
		if d.FieldWasSet("misc", 0, field) {
			defined = 1
		}
		add([]any{"defined-misc", field, defined})
	}
	// English string catalogs differ between engines. Compare every string the
	// patch actually changes; leave unrelated port messages out of this gate.
	base := Vanilla().data.Strings
	for key, value := range s.Strings {
		if value != base[key] {
			add([]any{"string", key, value})
		}
	}
	for key, value := range s.Cheats {
		if d.FieldWasSet("cheat", 0, key) {
			add([]any{"cheat", key, value})
		}
	}
	f, err := os.Open(native)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), DefaultMaxBytes)
	compared := 0
	seen := map[string]bool{}
	for scanner.Scan() {
		var row []any
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil || len(row) < 2 {
			t.Fatalf("invalid native row: %s (%v)", scanner.Text(), err)
		}
		if row[0] == "cheat" {
			row[1] = lower(row[1].(string))
		}
		key := fmt.Sprint(row[:2])
		expected, ok := wanted[key]
		if !ok {
			if row[0] == "string" || row[0] == "cheat" {
				continue
			}
			t.Fatalf("unexpected native row: %v", row)
		}
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		switch row[0] {
		case "state":
			row[5] = strings.ToUpper(row[5].(string))
		case "sound", "sprite", "music":
			row[2] = lower(row[2].(string))
		}
		if !reflect.DeepEqual(row, expected) {
			t.Fatalf("%s differs\nDSDA=%v\nGo=%v", key, row, expected)
		}
		compared++
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if compared != len(wanted) {
		for key := range wanted {
			if !seen[key] {
				t.Errorf("missing native row %s", key)
			}
		}
		t.Fatalf("compared=%d want=%d", compared, len(wanted))
	}
	t.Log("independent native definition rows matched: " + strconv.Itoa(compared))
}
