module github.com/k0ngk0ng/wire-talk

go 1.25.0

require (
	github.com/gen2brain/malgo v0.11.24
	github.com/gopxl/beep v1.4.1
	github.com/k0ngk0ng/wirectl v0.1.0
	github.com/schollz/pake/v3 v3.2.0
	golang.org/x/sys v0.31.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/icza/bitio v1.1.0 // indirect
	github.com/mewkiz/flac v1.0.8 // indirect
	github.com/mewkiz/pkg v0.0.0-20230226050401-4010bf0fec14 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/tscholl2/siec v0.0.0-20240310163802-c2c6f6198406 // indirect
)

replace github.com/k0ngk0ng/wirectl => ./third_party/wirectl
