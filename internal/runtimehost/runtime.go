package runtimehost

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/session"
)

type Accessors struct {
	Close           func()
	Err             func() error
	RecordedDemo    func() (*demo.Script, error)
	Options         func() runtimecfg.Options
	StartMapName    func() mapdata.MapName
	CurrentWorldTic func() int
	CaptureKeyframe func() ([]byte, error)
	LoadKeyframe    func([]byte) error
}

func NewGame(runtime session.Runtime, accessors Accessors) (session.Runtime, Meta) {
	return runtime, Meta{
		Close:           accessors.Close,
		Err:             accessors.Err,
		RecordedDemo:    accessors.RecordedDemo,
		Options:         accessors.Options,
		StartMapName:    accessors.StartMapName,
		CurrentWorldTic: accessors.CurrentWorldTic,
		CaptureKeyframe: accessors.CaptureKeyframe,
		LoadKeyframe:    accessors.LoadKeyframe,
	}
}
