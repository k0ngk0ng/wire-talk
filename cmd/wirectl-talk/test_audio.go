package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/k0ngk0ng/wire-talk/internal/audio"
)

func testAudioCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: wirectl talk test input|output [--device ID] [--seconds 10]")
		fmt.Println("Local test only. Input: live microphone level. Output: quiet 440 Hz tone; confirm audibility by listening. Ctrl+C stops.")
		return nil
	}
	kind := args[0]
	if kind != "input" && kind != "output" {
		return fmt.Errorf("choose test input or test output")
	}
	fs := flag.NewFlagSet("test "+kind, flag.ContinueOnError)
	id := fs.String("device", "", "device ID from talk devices (default: system default)")
	seconds := fs.Int("seconds", 10, "test duration, 1–300 seconds")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *seconds < 1 || *seconds > 300 {
		return fmt.Errorf("use --device ID and --seconds 1..300")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*seconds)*time.Second)
	defer cancel()
	if kind == "output" {
		fmt.Println("Playing a quiet test tone locally. The meter shows generated audio; confirm that your speaker is audible.")
	} else {
		fmt.Println("Speak into the microphone. This test does not transmit or play captured audio.")
	}
	fmt.Println("Ctrl+C stops. Meter: RMS dBFS, peak, clipping.")
	err := audio.TestDevice(ctx, kind, *id, func(name string, l audio.Level) {
		db := -60.0
		if l.RMS > 0 {
			db = math.Max(-60, 20*math.Log10(l.RMS))
		}
		bars := int((db + 60) / 60 * 30)
		bars = max(0, min(30, bars))
		clip := ""
		if l.Clipped {
			clip = " CLIP"
		}
		fmt.Printf("\r%-35s [%s%s] %6.1f dBFS peak %5.1f%%%s   ", cleanText(name), strings.Repeat("#", bars), strings.Repeat("-", 30-bars), db, l.Peak*100, clip)
	})
	fmt.Println()
	return err
}
