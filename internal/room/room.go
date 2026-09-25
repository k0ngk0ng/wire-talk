package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-talk/internal/volume"
	"net"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Peer struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
	Level    Level     `json:"level"`
	levels   levelWindow
}
type Status struct {
	SelfLevel Level  `json:"self_level"`
	Listen    string `json:"listen"`
	ID        string `json:"id"`
	Peers     []Peer `json:"peers"`
	Sent      uint64 `json:"sent_frames"`
	Received  uint64 `json:"received_frames"`
	Rejected  uint64 `json:"rejected_packets"`
	Dropped   uint64 `json:"dropped_frames"`
}
type Room struct {
	conn                              *net.UDPConn
	codec                             *codec
	id                                [16]byte
	mixer                             *Mixer
	mu                                sync.Mutex
	peers                             map[[16]byte]Peer
	seeds                             []netip.AddrPort
	audio                             chan []byte
	selfLevels                        levelWindow
	sent, received, rejected, dropped atomic.Uint64
}

func Open(listen string, seeds []string, key []byte) (*Room, error) {
	c, err := newCodec(key)
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, err
	}
	r := &Room{codec: c, mixer: NewMixer(), peers: make(map[[16]byte]Peer), audio: make(chan []byte, 3)}
	if _, err = rand.Read(r.id[:]); err != nil {
		return nil, err
	}
	if len(seeds) > MaxPeers {
		return nil, fmt.Errorf("at most %d peers", MaxPeers)
	}
	for _, s := range seeds {
		a, e := net.ResolveUDPAddr("udp", s)
		if e != nil {
			return nil, e
		}
		p := a.AddrPort()
		if !validAddress(p) {
			return nil, fmt.Errorf("invalid peer %q", s)
		}
		r.seeds = append(r.seeds, p)
	}
	r.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return r, nil
}
func validAddress(a netip.AddrPort) bool {
	return a.IsValid() && a.Port() != 0 && !a.Addr().IsUnspecified() && !a.Addr().IsMulticast() && a.Addr().Unmap() != netip.MustParseAddr("255.255.255.255")
}
func (r *Room) Close() error { return r.conn.Close() }
func (r *Room) Capture(frame []byte) {
	if len(frame) != FrameBytes {
		return
	}
	p := append([]byte(nil), frame...)
	select {
	case r.audio <- p:
		r.mu.Lock()
		r.selfLevels.add(frame, time.Now())
		r.mu.Unlock()
	default:
		r.dropped.Add(1)
	}
}
func (r *Room) Playback(out []byte) { r.mixer.Read(out) }
func (r *Room) PlaybackSelected(out, selected []byte, peer [16]byte) {
	r.mixer.ReadSelected(out, selected, peer)
}
func (r *Room) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Status{SelfLevel: r.selfLevels.snapshot(time.Now()), Listen: r.conn.LocalAddr().String(), ID: hex.EncodeToString(r.id[:]), Peers: []Peer{}, Sent: r.sent.Load(), Received: r.received.Load(), Rejected: r.rejected.Load(), Dropped: r.dropped.Load()}
	for _, p := range r.peers {
		if time.Since(p.LastSeen) < 6*time.Second {
			p.Level = p.levels.snapshot(time.Now())
			s.Peers = append(s.Peers, p)
		}
	}
	sort.Slice(s.Peers, func(i, j int) bool { return s.Peers[i].ID < s.Peers[j].ID })
	return s
}
func (r *Room) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			r.conn.Close()
		case <-done:
		}
	}()
	defer close(done)
	sendDone := make(chan struct{})
	child, cancel := context.WithCancel(ctx)
	go func() { defer close(sendDone); r.sendLoop(child) }()
	defer func() { cancel(); <-sendDone }()
	buf := make([]byte, maxPacket+1)
	for {
		n, from, err := r.conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		now := time.Now()
		r.codec.prune(now)
		p, err := r.codec.open(buf[:n], now)
		if err != nil {
			r.rejected.Add(1)
			continue
		}
		if p.Sender == r.id {
			continue
		}
		r.mu.Lock()
		for id, peer := range r.peers {
			if now.Sub(peer.LastSeen) > 30*time.Second {
				delete(r.peers, id)
			}
		}
		_, known := r.peers[p.Sender]
		if !known && len(r.peers) >= MaxPeers {
			r.mu.Unlock()
			continue
		}
		peer := r.peers[p.Sender]
		peer.ID, peer.Address, peer.LastSeen = hex.EncodeToString(p.Sender[:]), from.String(), now
		if p.Kind == voice {
			peer.levels.add(p.Body, now)
		}
		r.peers[p.Sender] = peer
		r.mu.Unlock()
		if p.Kind == voice {
			r.received.Add(1)
			r.mixer.Push(p.Sender, p.Seq, p.Body)
		}
		if p.Kind == hello {
			var candidates []string
			if json.Unmarshal(p.Body, &candidates) == nil && len(candidates) <= MaxPeers {
				r.mu.Lock()
				for _, s := range candidates {
					a, e := netip.ParseAddrPort(s)
					if e != nil || !validAddress(a) {
						continue
					}
					found := false
					for _, v := range r.seeds {
						if v == a {
							found = true
						}
					}
					if !found && len(r.seeds) < MaxPeers {
						r.seeds = append(r.seeds, a)
					}
				}
				r.mu.Unlock()
			}
		}
	}
}
func (r *Room) sendLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var seq uint64
	send := func(kind byte, body []byte) {
		seq++
		b, err := r.codec.seal(packet{Kind: kind, Sender: r.id, Seq: seq, Body: body}, time.Now())
		if err != nil {
			return
		}
		destinations := map[netip.AddrPort]bool{}
		r.mu.Lock()
		for _, p := range r.seeds {
			destinations[p] = true
		}
		for _, p := range r.peers {
			if time.Since(p.LastSeen) < 6*time.Second {
				a, e := netip.ParseAddrPort(p.Address)
				if e == nil {
					destinations[a] = true
				}
			}
		}
		r.mu.Unlock()
		for addr := range destinations {
			if _, err := r.conn.WriteToUDPAddrPort(b, addr); err == nil && kind == voice {
				r.sent.Add(1)
			}
		}
	}
	announce := func() {
		s := r.Status()
		addresses := []string{}
		for _, p := range s.Peers {
			addresses = append(addresses, p.Address)
		}
		body, _ := json.Marshal(addresses)
		if len(body) > maxPacket-65 {
			body = []byte("[]")
		}
		send(hello, body)
	}
	announce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			announce()
		case frame := <-r.audio:
			send(voice, frame)
		}
	}
}

// PlaybackSelectedMonitor preserves received recording and meters before gain.
func (r *Room) PlaybackSelectedMonitor(out, selected []byte, peer [16]byte, monitor []float64, gains map[string]float64) {
	factors := make(map[[16]byte]float64, len(gains))
	r.mu.Lock()
	for id, p := range r.peers {
		address, _ := volume.Address(p.Address)
		if db, ok := gains[address]; ok {
			factors[id] = volume.Factor(db)
		}
	}
	r.mu.Unlock()
	r.mixer.ReadSelectedMonitor(out, selected, peer, monitor, factors)
}
