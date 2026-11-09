package app

import (
	"fmt"
	"gddoom/internal/dehacked"
	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/wad"
	"io"
)

func loadGameplayDehacked(wf *wad.File, paths []string, skipEmbedded bool, stderr io.Writer) (*dehacked.Definitions, error) {
	defs, diagnostics, err := launchcatalog.LoadDehacked(wf, paths, skipEmbedded)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	if err != nil {
		return nil, err
	}
	if err := doomruntime.ValidateDehackedDefinitions(defs); err != nil {
		return nil, err
	}
	return defs, nil
}
