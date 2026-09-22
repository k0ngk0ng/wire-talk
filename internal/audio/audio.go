// Package audio binds one native capture/playback pair using miniaudio.
package audio

// #include <stdlib.h>
import "C"
import (
	"encoding/binary"
	"fmt"
	"github.com/gen2brain/malgo"
	"github.com/k0ngk0ng/wire-talk/internal/room"
	"sync/atomic"
	"unsafe"
)

type Device struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}
type Audio struct {
	context       *malgo.AllocatedContext
	device        *malgo.Device
	Muted         atomic.Bool
	OutputMuted   atomic.Bool
	Input, Output string
	Stopped       chan struct{}
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
	ctx, err := malgo.InitContext(backends, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	a := &Audio{context: ctx, Stopped: make(chan struct{}, 1)}
	fail := func(err error) (*Audio, error) { ctx.Uninit(); ctx.Free(); return nil, err }
	deviceType := malgo.Duplex
	if input == "none" {
		deviceType = malgo.Playback
	}
	cfg := malgo.DefaultDeviceConfig(deviceType)
	cfg.SampleRate = room.SampleRate
	cfg.PeriodSizeInFrames = room.FrameSamples
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.Playback.Format = malgo.FormatS16
	cfg.Playback.Channels = 1
	selectDevice := func(kind malgo.DeviceType, id string) (unsafe.Pointer, string, error) {
		devices, err := ctx.Devices(kind)
		if err != nil {
			return nil, "", err
		}
		for _, d := range devices {
			if (id == "" && d.IsDefault != 0) || d.ID.String() == id {
				return d.ID.Pointer(), d.Name(), nil
			}
		}
		label, option := "input (microphone)", "--input"
		if kind == malgo.Playback {
			label, option = "output (headphones/speakers)", "--output"
		}
		if id == "" {
			if len(devices) == 0 {
				return nil, "", fmt.Errorf("no audio %s devices found; connect a device and run wirectl talk devices", label)
			}
			return nil, "", fmt.Errorf("no default audio %s device; select a system default or configure %s (see wirectl talk devices)", label, option)
		}
		return nil, "", fmt.Errorf("audio %s device %q not found; run wirectl talk devices", label, id)
	}
	if input == "none" {
		a.Input = "disabled (file input only)"
	} else {
		cfg.Capture.DeviceID, a.Input, err = selectDevice(malgo.Capture, input)
	}
	if err != nil {
		return fail(err)
	}
	defer C.free(cfg.Capture.DeviceID)
	cfg.Playback.DeviceID, a.Output, err = selectDevice(malgo.Playback, output)
	if err != nil {
		return fail(err)
	}
	defer C.free(cfg.Playback.DeviceID)
	captured := make([]byte, 0, room.FrameBytes)
	played := make([]byte, room.FrameBytes)
	offset := room.FrameBytes
	guardSamples := 0
	a.device, err = malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(out, in []byte, _ uint32) {
			if input == "none" {
				in = make([]byte, len(out))
			}
			for len(in) > 0 {
				n := min(room.FrameBytes-len(captured), len(in))
				captured = append(captured, in[:n]...)
				in = in[n:]
				if len(captured) == room.FrameBytes {
					if a.Muted.Load() || (!headphones && guardSamples > 0) {
						clear(captured)
					}
					// Always clock file input, including while microphone is muted.
					capture(captured)
					captured = captured[:0]
					guardSamples = max(0, guardSamples-room.FrameSamples)
				}
			}
			for len(out) > 0 {
				if offset == room.FrameBytes {
					playback(played)
					if !headphones && !a.OutputMuted.Load() && audible(played) {
						guardSamples = room.SampleRate / 5
					}
					offset = 0
				}
				n := copyPlayback(out, played[offset:], a.OutputMuted.Load())
				offset += n
				out = out[n:]
			}
		}, Stop: func() {
			select {
			case a.Stopped <- struct{}{}:
			default:
			}
		},
	})
	if err != nil {
		return fail(err)
	}
	if err = a.device.Start(); err != nil {
		a.device.Uninit()
		return fail(err)
	}
	return a, nil
}
func (a *Audio) Close() { beforeDeviceClose(); a.device.Uninit(); a.context.Uninit(); a.context.Free() }

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
