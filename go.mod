module gddoom

go 1.27.4

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/Distortions81/GoBeep86 v0.0.1
	github.com/Distortions81/g726 v0.0.8
	github.com/Distortions81/impsynth v0.1.3
	github.com/coder/websocket v1.8.15
	github.com/dustin/go-humanize v1.1.0
	github.com/gen2brain/raylib-go/raylib v0.60.1
	github.com/hajimehoshi/ebiten/v2 v2.10.6
	github.com/klauspost/compress v1.20.4
	github.com/quic-go/quic-go v0.64.1 // webtransport-go v0.13.0 does not support the v0.65 HTTPStream API.
	github.com/quic-go/webtransport-go v0.13.0
	github.com/remeh/sizedwaitgroup v1.0.0
	github.com/sinshu/go-meltysynth v0.1.2
	github.com/youthlin/silk v0.0.4
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/sys v0.50.0
)

require (
	github.com/dunglas/httpsfv v1.1.2 // indirect
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.1 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/jupiterrider/ffi v0.8.0 // indirect
	github.com/klauspost/cpuid/v2 v2.4.2 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/image v0.48.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

replace github.com/sinshu/go-meltysynth => github.com/Distortions81/go-meltysynth v0.1.4

// Pinned Oto with adaptive browser output buffering; see third_party/oto/GDDOOM.md.
replace github.com/ebitengine/oto/v3 => ./third_party/oto
