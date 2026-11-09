package launchcatalog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gddoom/internal/dehacked"
	"gddoom/internal/wad"
)

// LoadDehacked follows DSDA's explicit-content order: all DEHACKED lumps in
// WAD/directory order, then external patches in argument order. It performs no
// sidecar/autoload search. An external INCLUDE is relative to its parent file;
// embedded includes have no filesystem resolver.
func LoadDehacked(file *wad.File, paths []string, skipEmbedded bool) (*dehacked.Definitions, []dehacked.Diagnostic, error) {
	defs := dehacked.Vanilla()
	var diagnostics []dehacked.Diagnostic
	bytesRead, changes := 0, 0
	account := func(size int) error {
		bytesRead += size
		if bytesRead > dehacked.DefaultMaxBytes {
			return fmt.Errorf("DeHackEd patch stack exceeds %d bytes", dehacked.DefaultMaxBytes)
		}
		return nil
	}
	read := func(path string) (dehacked.Source, error) {
		f, err := os.Open(path)
		if err != nil {
			return dehacked.Source{}, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, int64(dehacked.DefaultMaxBytes-bytesRead)+1))
		if err != nil {
			return dehacked.Source{}, err
		}
		if err := account(len(data)); err != nil {
			return dehacked.Source{}, err
		}
		return dehacked.Source{Name: filepath.Clean(path), Data: data}, nil
	}
	apply := func(source dehacked.Source, external bool) error {
		options := dehacked.ParseOptions{}
		if external {
			options.ResolveInclude = func(parent, name string) (dehacked.Source, error) {
				if !filepath.IsAbs(name) {
					name = filepath.Join(filepath.Dir(parent), name)
				}
				return read(name)
			}
		}
		patch, err := dehacked.ParseWithOptions(source.Name, source.Data, options)
		if err != nil {
			return err
		}
		changes += patch.ChangeCount()
		if changes > dehacked.DefaultMaxChanges {
			return fmt.Errorf("DeHackEd patch stack exceeds change limit")
		}
		diagnostics = append(diagnostics, patch.Diagnostics()...)
		defs, err = defs.Apply(patch)
		return err
	}
	if file != nil && !skipEmbedded {
		for _, lump := range file.Lumps {
			if !strings.EqualFold(lump.Name, "DEHACKED") {
				continue
			}
			data, err := file.LumpDataView(lump)
			if err != nil {
				return nil, diagnostics, err
			}
			if err := account(len(data)); err != nil {
				return nil, diagnostics, err
			}
			source := dehacked.Source{Name: fmt.Sprintf("%s:DEHACKED[%d]", file.Path, lump.Index), Data: data}
			if err := apply(source, false); err != nil {
				return nil, diagnostics, err
			}
		}
	}
	for _, path := range paths {
		source, err := read(path)
		if err != nil {
			return nil, diagnostics, fmt.Errorf("load DeHackEd %s: %w", path, err)
		}
		if err := apply(source, true); err != nil {
			return nil, diagnostics, err
		}
	}
	if changes == 0 {
		return nil, diagnostics, nil
	}
	return defs, diagnostics, nil
}

// DehackedPaths permits both repeated flags and comma-separated paths.
type DehackedPaths []string

func (p *DehackedPaths) String() string { return strings.Join(*p, ",") }
func (p *DehackedPaths) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if path := strings.TrimSpace(part); path != "" {
			*p = append(*p, path)
		}
	}
	return nil
}
