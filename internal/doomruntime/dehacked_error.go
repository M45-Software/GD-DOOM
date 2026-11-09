package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimehost"
	"gddoom/internal/session"
	"github.com/hajimehoshi/ebiten/v2"
)

// Runtime constructors predate fallible definition loading. Preserve their API
// while returning a terminal runtime before opening resources or consuming RNG.
type definitionErrorRuntime struct {
	err  error
	opts Options
}

func (r *definitionErrorRuntime) Update() error      { return r.err }
func (r *definitionErrorRuntime) Draw(*ebiten.Image) {}
func (r *definitionErrorRuntime) Layout(int, int) (int, int) {
	return max(r.opts.Width, 1), max(r.opts.Height, 1)
}
func failedDefinitionRuntime(m *mapdata.Map, opts Options, err error) (session.Runtime, runtimehost.Meta) {
	name := mapdata.MapName("")
	if m != nil {
		name = m.Name
	}
	return &definitionErrorRuntime{err: err, opts: opts}, runtimehost.Meta{
		Close: func() {}, Err: func() error { return err }, Options: func() Options { return opts },
		EffectiveDemoRecord: func() []DemoTic { return nil }, StartMapName: func() mapdata.MapName { return name }, CurrentWorldTic: func() int { return 0 },
		CaptureKeyframe: func() ([]byte, error) { return nil, err }, LoadKeyframe: func([]byte) error { return err },
	}
}
