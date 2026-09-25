package main

import (
	"github.com/k0ngk0ng/wire-talk/internal/groups"
	"github.com/k0ngk0ng/wire-talk/internal/room"
	"strings"
	"testing"
)

func TestGroupLevelsShowsEachMember(t *testing.T) {
	g := groups.Status{Name: "default", RoomID: "room", Current: true, Online: true, Status: room.Status{Peers: []room.Peer{{ID: "one", Address: "10.0.0.1:51830", Level: room.Level{RMS: 0.5, Peak: 0.5}}, {ID: "two", Address: "10.0.0.2:51830"}}}}
	s := formatGroupLevels(g)
	for _, want := range []string{"Self (sending)", "10.0.0.1:51830", "10.0.0.2:51830", "-6.0 dBFS", "-inf dBFS"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
}
