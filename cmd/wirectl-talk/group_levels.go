package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/k0ngk0ng/wire-talk/internal/audio"
	"github.com/k0ngk0ng/wire-talk/internal/groups"
	"github.com/k0ngk0ng/wire-talk/internal/room"
)

func groupLevels(ctx context.Context, dir string, args []string) error {
	fs := flag.NewFlagSet("group levels", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "output JSON snapshots")
	if err := fs.Parse(positionalLast(args)); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: group levels [ROOM_ID|NAME] [--json]")
	}
	ref := ""
	if fs.NArg() == 1 {
		ref = fs.Arg(0)
	}
	info, _ := os.Stdout.Stat()
	interactive := !*asJSON && info != nil && info.Mode()&os.ModeCharDevice != 0
	if interactive {
		fmt.Print("\x1b[?1049h\x1b[?25l")
		defer fmt.Print("\x1b[?25h\x1b[?1049l")
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		s, err := readGroups(ctx, dir)
		if err != nil {
			return err
		}
		g, err := chooseGroup(s, ref)
		if err != nil {
			return err
		}
		if *asJSON {
			b, _ := json.Marshal(g)
			fmt.Println(string(b))
		} else {
			if interactive {
				fmt.Print("\x1b[H\x1b[J")
			}
			fmt.Print(formatGroupLevels(g))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func memberLevel(l room.Level) string {
	return formatDeviceLevel(&audio.DeviceState{Online: true, Level: audio.Level{RMS: l.RMS, Peak: l.Peak, Clipped: l.Clipped}}, false)
}
func formatGroupLevels(g groups.Status) string {
	state := "online"
	if !g.Online {
		state = "offline"
	}
	text := fmt.Sprintf("Room %s (%s) · %s\nMember audio levels · 100ms · Ctrl+C exits; talk stays online.\nSelf: room input. Others: received audio, before mixing / speaker mute.\n\n", cleanText(g.Name), g.RoomID, state)
	self := "Self (sending)"
	if !g.Current {
		self = "Self (not speaking here)"
	}
	text += fmt.Sprintf("%s\n  %s\n", self, memberLevel(g.SelfLevel))
	for _, p := range g.Peers {
		text += fmt.Sprintf("%s · %s\n  %s\n", cleanText(p.Address), cleanText(p.ID), memberLevel(p.Level))
	}
	if len(g.Peers) == 0 {
		text += "No other members online.\n"
	}
	return text
}
