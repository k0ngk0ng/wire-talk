package room

import (
	"encoding/binary"
	"sync"
	"time"
)

type stream struct {
	frames  [][]byte
	seq     uint64
	last    time.Time
	started bool
}

// Mixer bounds latency and memory per speaker. Missing audio becomes silence;
// simultaneous speakers are summed with saturation, never integer wraparound.
type Mixer struct {
	mu      sync.Mutex
	streams map[[16]byte]*stream
}

func NewMixer() *Mixer { return &Mixer{streams: make(map[[16]byte]*stream)} }
func (m *Mixer) Push(id [16]byte, seq uint64, pcm []byte) {
	if len(pcm) != FrameBytes {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.streams[id]
	if s == nil {
		if len(m.streams) >= MaxPeers {
			return
		}
		s = &stream{}
		m.streams[id] = s
	}
	if seq <= s.seq {
		return
	}
	s.seq = seq
	s.last = time.Now()
	if len(s.frames) >= 6 {
		s.frames = s.frames[1:]
	}
	s.frames = append(s.frames, append([]byte(nil), pcm...))
}
func (m *Mixer) Read(out []byte) {
	clear(out)
	if len(out) != FrameBytes {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sums := [FrameSamples]int32{}
	for id, s := range m.streams {
		if time.Since(s.last) > time.Second*5 {
			delete(m.streams, id)
			continue
		}
		if !s.started {
			if len(s.frames) < 2 {
				continue
			}
			s.started = true
		}
		if len(s.frames) == 0 {
			s.started = false
			continue
		}
		frame := s.frames[0]
		s.frames = s.frames[1:]
		for i := range sums {
			sums[i] += int32(int16(binary.LittleEndian.Uint16(frame[i*2:])))
		}
	}
	for i, v := range sums {
		if v > 32767 {
			v = 32767
		}
		if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(v)))
	}
}
