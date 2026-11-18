package runtimehost

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

type Meta struct {
	Close           func()
	Err             func() error
	RecordedDemo    func() (*demo.Script, error)
	Options         func() runtimecfg.Options
	StartMapName    func() mapdata.MapName
	CurrentWorldTic func() int
	CaptureKeyframe func() ([]byte, error)
	LoadKeyframe    func([]byte) error
}
