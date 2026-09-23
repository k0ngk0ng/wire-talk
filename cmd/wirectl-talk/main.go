package main

import (
	"context"
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
	"github.com/k0ngk0ng/wire-talk/internal/media"
	"github.com/k0ngk0ng/wire-talk/internal/room"
	"github.com/k0ngk0ng/wire-talk/internal/service"
	"github.com/k0ngk0ng/wire-talk/internal/update"
	"github.com/k0ngk0ng/wirectl/cli"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, flag.ErrHelp) {
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
	if len(args) == 1 && args[0] == "__serve" {
		f, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		os.Stdout = f
		os.Stderr = f
		for {
			err = join(ctx, dir)
			if errors.Is(err, errAudioShutdownTimeout) {
				return err
			}
			if err == nil || ctx.Err() != nil {
				return nil
			}
			if st, e := f.Stat(); e == nil && st.Size() > 5<<20 {
				_ = f.Truncate(0)
			}
			fmt.Fprintln(f, time.Now().Format(time.RFC3339), "talk:", err, "— retrying in 3s")
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(3 * time.Second):
			}
		}
	}
	cmd := func(summary string, f func(context.Context, []string) error) cli.Command {
		return cli.Command{Summary: summary, Run: f}
	}
	app := cli.App{Name: "wirectl talk", Description: "direct encrypted microphone and speaker conversations", Commands: map[string]cli.Command{
		"record":     cmd("Record all received audio or a selected peer", func(c context.Context, a []string) error { return mediaCommand(c, dir, "record", a) }),
		"input":      cmd("Send an audio file instead of/alongside microphone", func(c context.Context, a []string) error { return mediaCommand(c, dir, "input", a) }),
		"test":       cmd("Test local input/output with live audio meters", func(c context.Context, a []string) error { return testAudioCommand(c, dir, a) }),
		"completion": cmd("Print bash or zsh completion script", func(_ context.Context, a []string) error { return completionCommand(a) }),
		"update":     cmd("Verify and install latest release", func(c context.Context, a []string) error { return updateCommand(c, dir, a) }),
		"version": cmd("Print version", func(_ context.Context, a []string) error {
			if err := noArguments("version", a); err != nil {
				return err
			}
			fmt.Println(version)
			return nil
		}),
		"init":    cmd("Create room config (never overwrites)", func(_ context.Context, a []string) error { return initConfig(dir, a) }),
		"invite":  cmd("Generate a one-use pairing code (5 minutes)", func(c context.Context, a []string) error { return inviteCommand(c, dir, a) }),
		"pair":    cmd("Save a room using HOST:PORT --code CODE, without starting audio", func(c context.Context, a []string) error { return pairCommand(c, dir, a) }),
		"devices": cmd("List audio devices in a table; --json for scripts", func(_ context.Context, a []string) error { return devicesCommand(a) }),
		"join": cmd("Join HOST:PORT --code CODE, or start saved room audio; Ctrl+C stops", func(c context.Context, a []string) error {
			if len(a) > 0 {
				if err := pairCommand(c, dir, a); err != nil {
					return err
				}
			}
			return join(c, dir)
		}),
		"daemon": cmd("start | install | stop | status (background audio)", func(c context.Context, a []string) error { return daemon(c, dir, a) }),
		"status": cmd("Show readable session status; --json for scripts", func(c context.Context, a []string) error { return status(c, dir, false, a) }),
		"watch":  cmd("Watch readable status; --json for scripts; Ctrl+C only exits watch", func(c context.Context, a []string) error { return status(c, dir, true, a) }),
		"mute":   cmd("Mute local input or output", func(c context.Context, a []string) error { return muteCommand(c, dir, true, a) }),
		"unmute": cmd("Unmute local input or output", func(c context.Context, a []string) error { return muteCommand(c, dir, false, a) }),
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
	fs.StringVar(&c.Input, "input", "", "input device ID, or none for file-only input (default device when empty)")
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
	fmt.Println("Invite a member: wirectl talk invite. Start audio: wirectl talk join")
	return nil
}
func join(ctx context.Context, dir string) (result error) {
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
	m := media.New(r)
	defer m.Close()
	a, err := audio.Open(c.Input, c.Output, c.Headphones, m.Capture, m.Playback)
	if err != nil {
		return err
	}
	defer func() {
		if err := closeAudioWithin(a.Close, 3*time.Second); err != nil {
			result = err
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := time.Now()
	api, err := control.Start(dir, func() any {
		input, output := a.Devices()
		return sessionStatus{Status: r.Status(), Input: input.Name, Output: output.Name, InputDevice: &input, OutputDevice: &output, Muted: a.Muted.Load(), OutputMuted: a.OutputMuted.Load(), Started: started, Version: version, Media: m.Status()}
	}, cancel, func(m bool) { a.Muted.Store(m) }, func(m bool) { a.OutputMuted.Store(m) }, m)
	if err != nil {
		return err
	}
	defer api.Close()
	fmt.Printf("Online at %s · audio devices reconnect automatically\n", r.Status().Listen)
	return r.Run(ctx)
}
func daemon(ctx context.Context, dir string, args []string) error {
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") && (args[0] == "start" || args[0] == "install" || args[0] == "stop") {
		fmt.Printf("Usage: wirectl talk daemon %s [--state-dir DIR]\n", args[0])
		return nil
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("Usage: wirectl talk daemon start | install | stop | status [--json]")
		return nil
	}
	if len(args) > 0 && args[0] == "status" {
		return status(ctx, dir, false, args[1:])
	}
	if len(args) != 1 {
		return errors.New("usage: daemon start | install | stop | status")
	}
	switch args[0] {
	case "install":
		if _, err := config.Load(dir); err != nil {
			return err
		}
		if _, err := control.Request(ctx, dir, "GET", "/status"); err == nil {
			return errors.New("stop the current session before installing a native service")
		}
		if err := service.Install(dir); err != nil {
			return err
		}
		fmt.Println("Login service registered; it will start after login and retry failures.")
		if _, err := control.Request(ctx, dir, "GET", "/status"); err == nil {
			fmt.Println("Audio is online.")
		} else {
			fmt.Println("Audio is not online yet. Run wirectl talk watch to check startup and device readiness.")
		}
		return nil
	case "stop":
		removed, err := service.Remove(dir)
		if err != nil {
			return err
		}
		if _, err := control.Request(ctx, dir, "POST", "/stop"); err != nil && !removed {
			return err
		}
		// Native managers may close the control socket before audio teardown
		// releases ownership. Wait for the lock even when the API is gone.
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
				return fmt.Errorf("background startup failed (%v)\n%s\nLog: %s", e, lastDaemonLog(dir), log.Name())
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
		return errors.New("usage: daemon start | install | stop | status")
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
