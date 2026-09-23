package audio

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/k0ngk0ng/wire-talk/internal/room"
)

type fakeAudioDevice struct {
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	teardown <-chan struct{}
}

func (d *fakeAudioDevice) Close() {
	d.once.Do(func() { close(d.stop) })
	<-d.done
	if d.teardown != nil {
		<-d.teardown
	}
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestMicrophoneDisconnectKeepsPlaybackAndRecovers(t *testing.T) {
	var available atomic.Bool
	available.Store(true)
	var inputStarts, outputStarts, heard atomic.Int32
	var captures, plays atomic.Int32
	factory := func(kind malgo.DeviceType, _ string, cb malgo.DeviceCallbacks) (deviceHandle, string, error) {
		if kind == malgo.Capture && !available.Load() {
			return nil, "", errors.New("unplugged")
		}
		if kind == malgo.Capture {
			inputStarts.Add(1)
		} else {
			outputStarts.Add(1)
		}
		d := &fakeAudioDevice{stop: make(chan struct{}), done: make(chan struct{})}
		go func() {
			defer close(d.done)
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			p := make([]byte, room.FrameBytes)
			for {
				select {
				case <-d.stop:
					return
				case <-ticker.C:
					if kind == malgo.Capture {
						if !available.Load() {
							cb.Stop()
							return
						}
						for i := range p {
							p[i] = 1
						}
						cb.Data(nil, p, room.FrameSamples)
					} else {
						cb.Data(p, nil, room.FrameSamples)
						if audible(p) {
							heard.Add(1)
						}
					}
				}
			}
		}()
		return d, "test device", nil
	}
	a := openWithFactory(factory, "mic", "speaker", true, func(p []byte) {
		if audible(p) {
			captures.Add(1)
		}
	}, func(p []byte) {
		for i := range p {
			p[i] = 2
		}
		plays.Add(1)
	})
	defer a.Close()
	eventually(t, func() bool { return captures.Load() > 2 && heard.Load() > 2 })
	available.Store(false)
	eventually(t, func() bool { i, o := a.Devices(); return !i.Online && o.Online })
	before := heard.Load()
	oldCapture := captures.Load()
	eventually(t, func() bool { return heard.Load() > before+10 })
	if captures.Load() != oldCapture {
		t.Fatal("stale microphone audio sent while offline")
	}
	a.Muted.Store(true)
	available.Store(true)
	eventually(t, func() bool { i, _ := a.Devices(); return i.Online && inputStarts.Load() >= 2 })
	time.Sleep(100 * time.Millisecond)
	if captures.Load() != oldCapture {
		t.Fatal("reconnect cleared microphone mute")
	}
	a.Muted.Store(false)
	eventually(t, func() bool { return captures.Load() > oldCapture+2 })
	if outputStarts.Load() != 1 {
		t.Fatal("microphone reconnect reopened output")
	}
}

func TestNoDevicesStillClocksMediaAndCanStop(t *testing.T) {
	var capture, playback atomic.Int32
	a := openWithFactory(func(malgo.DeviceType, string, malgo.DeviceCallbacks) (deviceHandle, string, error) {
		return nil, "", errors.New("absent")
	}, "missing", "missing", true, func(p []byte) {
		capture.Add(1)
		if audible(p) {
			t.Error("offline capture not silent")
		}
	}, func(p []byte) { clear(p); playback.Add(1) })
	eventually(t, func() bool { return capture.Load() >= 3 && playback.Load() >= 3 })
	a.Close()
	c := capture.Load()
	time.Sleep(50 * time.Millisecond)
	if capture.Load() != c {
		t.Fatal("clock continued after shutdown")
	}
}

func TestPCMQueueDropsStaleAudio(t *testing.T) {
	var q pcmQueue
	q.write(make([]byte, room.FrameBytes*4))
	fresh := make([]byte, room.FrameBytes*4)
	for i := range fresh {
		fresh[i] = 42
	}
	q.write(fresh)
	out := make([]byte, len(fresh)+2)
	q.read(out)
	for _, b := range out[:len(fresh)] {
		if b != 42 {
			t.Fatal("stale audio kept")
		}
	}
	if out[len(fresh)] != 0 || out[len(fresh)+1] != 0 {
		t.Fatal("underrun not silent")
	}
}

func TestStuckCaptureTeardownDoesNotStopOutput(t *testing.T) {
	release := make(chan struct{})
	var playback atomic.Int32
	callbacks := make(chan malgo.DeviceCallbacks, 1)
	factory := func(kind malgo.DeviceType, _ string, cb malgo.DeviceCallbacks) (deviceHandle, string, error) {
		d := &fakeAudioDevice{stop: make(chan struct{}), done: make(chan struct{})}
		close(d.done)
		if kind == malgo.Capture {
			d.teardown = release
			callbacks <- cb
		}
		return d, "test device", nil
	}
	a := openWithFactory(factory, "mic", "speaker", true, func([]byte) {}, func(p []byte) { clear(p); playback.Add(1) })
	defer func() { close(release); a.Close() }()
	cb := <-callbacks
	cb.Stop()
	eventually(t, func() bool { i, _ := a.Devices(); return !i.Online })
	n := playback.Load()
	eventually(t, func() bool { return playback.Load() > n+5 })
	_, o := a.Devices()
	if !o.Online {
		t.Fatal("stuck microphone teardown took output offline")
	}
}
