package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/k0ngk0ng/wire-talk/internal/audio"
	"github.com/k0ngk0ng/wire-talk/internal/config"
	"github.com/k0ngk0ng/wire-talk/internal/control"
	"github.com/k0ngk0ng/wire-talk/internal/room"
	"github.com/k0ngk0ng/wire-talk/internal/update"
	"github.com/k0ngk0ng/wirectl/cli"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "talk:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	dir, err := config.DefaultDir()
	if err != nil {
		return err
	}
	// --state-dir is accepted anywhere, including after daemon subcommands.
	for i := 0; i < len(args); i++ {
		if args[i] == "--state-dir" {
			if i+1 >= len(args) {
				return errors.New("--state-dir requires a path")
			}
			dir, err = filepath.Abs(args[i+1])
			if err != nil {
				return err
			}
			args = append(args[:i], args[i+2:]...)
			i--
		}
	}
	cmd := func(summary string, f func(context.Context, []string) error) cli.Command {
		return cli.Command{Summary: summary, Run: f}
	}
	app := cli.App{Name: "wirectl talk", Description: "direct encrypted microphone and speaker conversations", Commands: map[string]cli.Command{
		"update":  cmd("Verify and install latest release", func(c context.Context, a []string) error { return updateCommand(c, dir, a) }),
		"version": cmd("Print version", func(context.Context, []string) error { fmt.Println(version); return nil }),
		"init":    cmd("Create room config (never overwrites)", func(_ context.Context, a []string) error { return initConfig(dir, a) }),
		"devices": cmd("List audio input/output IDs", func(_ context.Context, a []string) error {
			if len(a) != 0 {
				return errors.New("usage: devices")
			}
			ds, e := audio.List()
			if e != nil {
				return e
			}
			return json.NewEncoder(os.Stdout).Encode(ds)
		}),
		"join": cmd("Run microphone and speaker in foreground; Ctrl+C stops", func(c context.Context, a []string) error {
			if len(a) > 0 {
				return errors.New("usage: join [--state-dir DIR]; set devices/peers in config.json")
			}
			return join(c, dir)
		}),
		"daemon": cmd("start | stop | status (background audio)", func(c context.Context, a []string) error { return daemon(c, dir, a) }),
		"status": cmd("Show current session status", func(c context.Context, a []string) error { return status(c, dir, false, a) }),
		"watch":  cmd("Watch status; Ctrl+C only exits watch", func(c context.Context, a []string) error { return status(c, dir, true, a) }),
		"mute":   cmd("Mute local microphone", func(c context.Context, a []string) error { _, e := control.Request(c, dir, "POST", "/mute"); return e }),
		"unmute": cmd("Unmute local microphone", func(c context.Context, a []string) error {
			_, e := control.Request(c, dir, "POST", "/unmute")
			return e
		}),
	}}
	return app.Run(ctx, args)
}
func initConfig(dir string, args []string) error {
	c, err := config.New()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.StringVar(&c.Listen, "listen", c.Listen, "UDP listen address")
	fs.StringVar(&c.Input, "input", "", "input device ID (default device when empty)")
	fs.StringVar(&c.Output, "output", "", "output device ID (default device when empty)")
	fs.BoolVar(&c.Headphones, "headphones", false, "allow full duplex; disable speaker feedback guard when using headphones")
	keyFile := fs.String("key-file", "", "read another member's room key from a private file")
	peers := fs.String("peers", "", "comma-separated reachable UDP host:port addresses")
	if err = fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected init arguments")
	}
	if *keyFile != "" {
		b, e := os.ReadFile(*keyFile)
		if e != nil {
			return e
		}
		c.Key = strings.TrimSpace(string(b))
	}
	if *peers != "" {
		for _, s := range strings.Split(*peers, ",") {
			c.Peers = append(c.Peers, strings.TrimSpace(s))
		}
	}
	if err = config.Save(dir, c); err != nil {
		return err
	}
	fmt.Println("Created", filepath.Join(dir, "config.json"))
	fmt.Println("Share the key field privately with room members. Start with: wirectl talk join")
	return nil
}
func join(ctx context.Context, dir string) error {
	unlock, err := control.Lock(dir)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := config.Load(dir)
	if err != nil {
		return err
	}
	key, _ := c.KeyBytes()
	r, err := room.Open(c.Listen, c.Peers, key)
	if err != nil {
		return err
	}
	defer r.Close()
	a, err := audio.Open(c.Input, c.Output, c.Headphones, r.Capture, r.Playback)
	if err != nil {
		return err
	}
	defer a.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := time.Now()
	api, err := control.Start(dir, func() any {
		return struct {
			room.Status
			Input   string    `json:"input"`
			Output  string    `json:"output"`
			Muted   bool      `json:"muted"`
			Started time.Time `json:"started"`
			Version string    `json:"version"`
		}{r.Status(), a.Input, a.Output, a.Muted.Load(), started, version}
	}, cancel, func(m bool) { a.Muted.Store(m) })
	if err != nil {
		return err
	}
	defer api.Close()
	fmt.Printf("Online at %s · input: %s · output: %s\n", r.Status().Listen, a.Input, a.Output)
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	select {
	case err := <-done:
		return err
	case <-a.Stopped:
		cancel()
		<-done
		return errors.New("audio device stopped; reconnect the device and restart talk")
	}
}
func status(ctx context.Context, dir string, watch bool, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: status | watch [--state-dir DIR]")
	}
	for {
		b, err := control.Request(ctx, dir, "GET", "/status")
		if err != nil {
			if !watch {
				return err
			}
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Print(string(b))
		}
		if !watch {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}
func daemon(ctx context.Context, dir string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: daemon start | stop | status")
	}
	switch args[0] {
	case "status":
		return status(ctx, dir, false, nil)
	case "stop":
		if _, err := control.Request(ctx, dir, "POST", "/stop"); err != nil {
			return err
		}
		for i := 0; i < 100; i++ {
			unlock, err := control.Lock(dir)
			if err == nil {
				unlock()
				fmt.Println("Stopped")
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		return errors.New("session did not stop within 10 seconds")
	case "start":
		if _, err := control.Request(ctx, dir, "GET", "/status"); err == nil {
			return errors.New("talk is already running")
		}
		if _, err := config.Load(dir); err != nil {
			return err
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		log, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer log.Close()
		child := exec.Command(self, "join", "--state-dir", dir)
		child.Stdout = log
		child.Stderr = log
		detach(child)
		if err = child.Start(); err != nil {
			return err
		}
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		for i := 0; i < 150; i++ {
			if _, err = control.Request(ctx, dir, "GET", "/status"); err == nil {
				fmt.Println("Background audio online. Watch: wirectl talk watch · Stop: wirectl talk daemon stop")
				return nil
			}
			select {
			case e := <-done:
				return fmt.Errorf("background startup failed (%v); see %s", e, log.Name())
			case <-ctx.Done():
				child.Process.Kill()
				<-done
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		child.Process.Kill()
		<-done
		return fmt.Errorf("audio startup timed out; see %s", log.Name())
	default:
		return errors.New("usage: daemon start | stop | status")
	}
}

func updateCommand(ctx context.Context, dir string, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	var o update.Options
	fs.StringVar(&o.Version, "version", "", "release version (default latest)")
	fs.StringVar(&o.Archive, "archive", "", "offline release archive")
	fs.StringVar(&o.Checksums, "checksums", "", "offline SHA256SUMS")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected update arguments")
	}
	// Do not replace the binary while this profile still captures audio. The
	// same OS lock prevents a concurrent daemon start during replacement.
	unlock, err := control.Lock(dir)
	if err != nil {
		return fmt.Errorf("stop talk before updating: %w", err)
	}
	defer unlock()
	v, err := update.Run(ctx, o)
	if err != nil {
		return err
	}
	fmt.Println("Installed", v, "· restart with wirectl talk daemon start")
	return nil
}
