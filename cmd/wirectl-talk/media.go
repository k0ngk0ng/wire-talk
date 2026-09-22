package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/k0ngk0ng/wire-talk/internal/control"
	"github.com/k0ngk0ng/wire-talk/internal/media"
)

func mediaCommand(ctx context.Context, dir, kind string, args []string) error {
	usage := func() {
		if kind == "record" {
			fmt.Println("Usage: wirectl talk record start FILE.wav [--peer ID|HOST:PORT] | stop | status [--json]")
			fmt.Println("Records received audio before output mute; default: all peers, no local microphone. Existing files are never overwritten.")
		} else {
			fmt.Println("Usage: wirectl talk input start FILE [--mode replace|mix] [--loop] | pause | resume | stop | status [--json]")
			fmt.Println("Sends WAV/MP3/FLAC to peers. Default: replace microphone. At EOF/stop, microphone resumes; pause in replace mode sends silence.")
		}
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		usage()
		return nil
	}
	if args[0] == "status" {
		fs := flag.NewFlagSet(kind+" status", flag.ContinueOnError)
		asJSON := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected status arguments")
		}
		b, err := control.Request(ctx, dir, "GET", "/status")
		if err != nil {
			return err
		}
		var snapshot struct {
			Media *media.Status `json:"media"`
		}
		if err = json.Unmarshal(b, &snapshot); err != nil {
			return err
		}
		if snapshot.Media == nil {
			return fmt.Errorf("restart the daemon with the updated version to use media controls")
		}
		if *asJSON {
			var value any = snapshot.Media.Input
			if kind == "record" {
				value = snapshot.Media.Recording
			}
			b, _ = json.MarshalIndent(value, "", "  ")
			fmt.Println(string(b))
		} else {
			printMedia(*snapshot.Media)
		}
		return nil
	}
	c := media.Command{Action: kind + "-" + args[0]}
	if args[0] == "start" {
		if len(args) < 2 || args[1] == "" {
			return fmt.Errorf("%s start requires a file", kind)
		}
		var err error
		c.File, err = filepath.Abs(args[1])
		if err != nil {
			return err
		}
		fs := flag.NewFlagSet(kind+" start", flag.ContinueOnError)
		if kind == "record" {
			fs.StringVar(&c.Peer, "peer", "all", "peer ID/address (default: all)")
		} else {
			fs.StringVar(&c.Mode, "mode", "replace", "replace or mix microphone")
			fs.BoolVar(&c.Loop, "loop", false, "repeat until stopped")
		}
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected %s arguments", kind)
		}
	} else {
		if len(args) != 1 || (args[0] != "stop" && !(kind == "input" && (args[0] == "pause" || args[0] == "resume"))) {
			return fmt.Errorf("invalid %s action; use %s --help", kind, kind)
		}
	}
	data, _ := json.Marshal(c)
	b, err := control.RequestBody(ctx, dir, "POST", "/media", data)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	var snapshot media.Status
	if err = json.Unmarshal(b, &snapshot); err != nil {
		return err
	}
	printMedia(snapshot)
	return nil
}
func printMedia(s media.Status) {
	fmt.Printf("Recording:   %s", s.Recording.State)
	if s.Recording.File != "" {
		fmt.Printf(" · %s · peer: %s · %d bytes", cleanText(s.Recording.File), cleanText(s.Recording.Peer), s.Recording.Bytes)
	}
	if s.Recording.Error != "" {
		fmt.Printf(" · error: %s", cleanText(s.Recording.Error))
	}
	fmt.Println()
	fmt.Printf("File input:  %s", s.Input.State)
	if s.Input.File != "" {
		fmt.Printf(" · %s · mode: %s · loop: %t", cleanText(s.Input.File), s.Input.Mode, s.Input.Loop)
	}
	if s.Input.Error != "" {
		fmt.Printf(" · error: %s", cleanText(s.Input.Error))
	}
	fmt.Println()
}
