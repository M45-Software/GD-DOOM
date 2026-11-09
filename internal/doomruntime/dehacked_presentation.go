package doomruntime

import (
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
	"strconv"
	"strings"
)

func definitionMapTitle(opts Options, name mapdata.MapName) (string, bool) {
	if opts.DehackedDefinitions == nil {
		return "", false
	}
	number, err := strconv.Atoi(strings.TrimPrefix(string(name), "MAP"))
	if err != nil {
		return "", false
	}
	key := fmt.Sprintf("HUSTR_%d", number)
	if !opts.DehackedDefinitions.FieldWasSet("strings", 0, key) {
		return "", false
	}
	return opts.DehackedDefinitions.String(key)
}
func applyDefinitionPresentationOptions(opts Options) Options {
	if opts.DehackedDefinitions == nil {
		return opts
	}
	opts.MusicPlayerCatalog = append([]runtimecfg.MusicPlayerWAD(nil), opts.MusicPlayerCatalog...)
	for wi := range opts.MusicPlayerCatalog {
		w := &opts.MusicPlayerCatalog[wi]
		w.Episodes = append([]runtimecfg.MusicPlayerEpisode(nil), w.Episodes...)
		for ei := range w.Episodes {
			ep := &w.Episodes[ei]
			ep.Tracks = append([]runtimecfg.MusicPlayerTrack(nil), ep.Tracks...)
			for ti := range ep.Tracks {
				track := &ep.Tracks[ti]
				if title, ok := definitionMapTitle(opts, track.MapName); ok {
					track.Label = title
				}
			}
		}
	}
	return opts
}
func (sg *sessionGame) applyDefinitionFinaleText(state sessionflow.Finale) sessionflow.Finale {
	if sg.opts.DehackedDefinitions == nil {
		return state
	}
	key := ""
	if state.Commercial {
		number := map[mapdata.MapName]int{"MAP06": 1, "MAP11": 2, "MAP20": 3, "MAP30": 4, "MAP15": 5, "MAP31": 6}[state.MapName]
		if number != 0 {
			key = fmt.Sprintf("C%dTEXT", number)
		}
	} else if len(state.MapName) >= 2 && state.MapName[0] == 'E' {
		key = fmt.Sprintf("E%cTEXT", state.MapName[1])
	}
	if key != "" && sg.opts.DehackedDefinitions.FieldWasSet("strings", 0, key) {
		if text, ok := sg.opts.DehackedDefinitions.String(key); ok {
			state.Text = text
		}
	}
	return state
}
