package audio

import (
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/k0ngk0ng/wire-talk/internal/room"
)

func TestNativeNullDeviceRunsDuplexCallbacks(t *testing.T) {
	var captured, played, invalid atomic.Int32
	// malgo v0.11.24 omits ma_backend_custom from its Go enum; the pinned
	// miniaudio.h defines ma_backend_null as 14 (BackendNull incorrectly is 13).
	a, err := openWithBackends([]malgo.Backend{malgo.Backend(14)}, "", "", true, func(p []byte) {
		if len(p) != room.FrameBytes {
			invalid.Add(1)
		}
		captured.Add(1)
	}, func(p []byte) { clear(p); played.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && (captured.Load() < 3 || played.Load() < 3) {
		time.Sleep(10 * time.Millisecond)
	}
	a.Close()
	if captured.Load() < 3 || played.Load() < 3 || invalid.Load() != 0 {
		t.Fatalf("capture=%d playback=%d invalid=%d", captured.Load(), played.Load(), invalid.Load())
	}
	after := captured.Load()
	time.Sleep(40 * time.Millisecond)
	if captured.Load() != after {
		t.Fatal("capture continued after close")
	}
}
func TestSpeakerGuardEnergy(t *testing.T) {
	pcm := make([]byte, room.FrameBytes)
	if audible(pcm) {
		t.Fatal("silence gated microphone")
	}
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:], 1000)
	}
	if !audible(pcm) {
		t.Fatal("speaker output did not gate microphone")
	}
}
