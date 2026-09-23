// Package audio manages capture and playback independently of the room connection.
package audio

import (
	"encoding/binary"
	"github.com/gen2brain/malgo"
	"github.com/k0ngk0ng/wire-talk/internal/room"
	"sync"
	"sync/atomic"
	"time"
)

type Device struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

type DeviceState struct {
	Name     string `json:"name"`
	Online   bool   `json:"online"`
	Error    string `json:"error,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type Audio struct {
	Muted         atomic.Bool
	OutputMuted   atomic.Bool
	input, output *endpoint
	stop          chan struct{}
	done          chan struct{}
	workers       sync.WaitGroup
	once          sync.Once
}

func List() ([]Device, error) {
	ctx, err := malgo.InitContext(nativeBackends(), malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer ctx.Free()
	defer ctx.Uninit()
	out := []Device{}
	for _, kind := range []malgo.DeviceType{malgo.Capture, malgo.Playback} {
		ds, err := ctx.Devices(kind)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			label := "input"
			if kind == malgo.Playback {
				label = "output"
			}
			out = append(out, Device{label, d.ID.String(), d.Name(), d.IsDefault != 0})
		}
	}
	return out, nil
}

func Open(input, output string, headphones bool, capture func([]byte), playback func([]byte)) (*Audio, error) {
	return openWithBackends(nativeBackends(), input, output, headphones, capture, playback)
}

func openWithBackends(backends []malgo.Backend, input, output string, headphones bool, capture func([]byte), playback func([]byte)) (*Audio, error) {
	return openWithFactory(nativeFactory(backends), input, output, headphones, capture, playback), nil
}

func openWithFactory(factory deviceFactory, input, output string, headphones bool, capture func([]byte), playback func([]byte)) *Audio {
	a := &Audio{stop: make(chan struct{}), done: make(chan struct{})}
	a.input = newEndpoint(malgo.Capture, input)
	a.output = newEndpoint(malgo.Playback, output)
	for _, e := range []*endpoint{a.input, a.output} {
		if e.snapshot().Disabled {
			continue
		}
		a.workers.Add(1)
		go func(e *endpoint) { defer a.workers.Done(); e.run(a.stop, factory) }(e)
	}
	// A software clock keeps received recording, file input and UDP audio alive
	// even when a native device is absent. Native callbacks only move bounded PCM.
	go func() {
		defer close(a.done)
		ticker := time.NewTicker(time.Duration(room.FrameSamples) * time.Second / room.SampleRate)
		defer ticker.Stop()
		in, out := make([]byte, room.FrameBytes), make([]byte, room.FrameBytes)
		guard := 0
		for {
			select {
			case <-a.stop:
				return
			case <-ticker.C:
				a.input.pcm.read(in)
				if a.Muted.Load() || !a.input.snapshot().Online || (!headphones && guard > 0) {
					clear(in)
				}
				capture(in)
				guard = max(0, guard-room.FrameSamples)
				playback(out)
				if a.OutputMuted.Load() {
					clear(out)
				}
				if a.output.snapshot().Online {
					if !headphones && audible(out) {
						guard = room.SampleRate / 5
					}
					a.output.pcm.write(out)
				}
			}
		}
	}()
	return a
}

func (a *Audio) Devices() (DeviceState, DeviceState) { return a.input.snapshot(), a.output.snapshot() }
func (a *Audio) Close() {
	a.once.Do(func() { close(a.stop) })
	<-a.done
	beforeDeviceClose()
	a.workers.Wait()
}

// audible ignores very quiet output so silence does not suppress the microphone.
// This is half-duplex speaker protection, not acoustic echo cancellation.
func audible(pcm []byte) bool {
	var energy int64
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int64(int16(binary.LittleEndian.Uint16(pcm[i:])))
		energy += v * v
	}
	return len(pcm) > 0 && energy > int64(len(pcm)/2)*100*100
}

// copyPlayback consumes the same audio while muted, preventing queued speech
// from being replayed when output is enabled again.
func copyPlayback(out, frame []byte, muted bool) int {
	n := copy(out, frame)
	if muted {
		clear(out[:n])
	}
	return n
}
