package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const MaxGroups = 16

type Group struct {
	Name   string   `json:"name"`
	Key    string   `json:"key"`
	Listen string   `json:"listen"`
	Peers  []string `json:"peers"`
	Muted  bool     `json:"listen_muted,omitempty"`
}

func (g Group) ID() string {
	key, _ := (Config{Key: g.Key}).KeyBytes()
	sum := sha256.Sum256(append([]byte("wire-talk room ID v1\x00"), key...))
	return hex.EncodeToString(sum[:8])
}
func (c Config) RoomConfigs() []Group {
	name := c.Name
	if name == "" {
		name = "default"
	}
	primary := Group{Name: name, Key: c.Key, Listen: c.Listen, Peers: c.Peers, Muted: c.ListenMuted}
	return append([]Group{primary}, c.Groups...)
}
func (c Config) CurrentID() string {
	if c.ActiveGroup != "" {
		return c.ActiveGroup
	}
	return c.RoomConfigs()[0].ID()
}
func (c Config) FindGroup(ref string) (Group, error) {
	if ref == "" {
		ref = c.CurrentID()
	}
	for _, g := range c.RoomConfigs() {
		if g.ID() == ref {
			return g, nil
		}
	}
	for _, g := range c.RoomConfigs() {
		if g.Name == ref {
			return g, nil
		}
	}
	return Group{}, fmt.Errorf("room %q not found; use group list", ref)
}
func (c Config) ValidateGroups() error {
	rooms := c.RoomConfigs()
	if len(rooms) > MaxGroups {
		return fmt.Errorf("at most %d rooms", MaxGroups)
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, g := range rooms {
		if _, err := (Config{Key: g.Key}).KeyBytes(); err != nil {
			return err
		}
		if g.Name == "" || len(g.Name) > 64 || strings.TrimSpace(g.Name) != g.Name || strings.IndexFunc(g.Name, unicode.IsControl) >= 0 {
			return fmt.Errorf("room name must be 1–64 bytes without control characters or surrounding spaces")
		}
		if _, _, err := net.SplitHostPort(g.Listen); err != nil {
			return fmt.Errorf("invalid listen address for room %s: %w", g.Name, err)
		}
		if ids[g.ID()] {
			return fmt.Errorf("already joined room %s", g.ID())
		}
		if names[g.Name] {
			return fmt.Errorf("room name %q already exists", g.Name)
		}
		ids[g.ID()] = true
		names[g.Name] = true
	}
	if !ids[c.CurrentID()] {
		return fmt.Errorf("active room does not exist")
	}
	return nil
}

type GroupChange struct {
	Action string `json:"action"`
	Room   string `json:"room,omitempty"`
	Name   string `json:"name,omitempty"`
	Group  *Group `json:"group,omitempty"`
}

// Changed returns a copy so failed writes never mutate live routing.
func (c Config) Changed(cmd GroupChange) (Config, error) {
	c.Groups = append([]Group(nil), c.Groups...)
	if cmd.Action == "add" {
		if cmd.Group == nil {
			return c, fmt.Errorf("room is required")
		}
		c.Groups = append(c.Groups, *cmd.Group)
	} else {
		g, err := c.FindGroup(cmd.Room)
		if err != nil {
			return c, err
		}
		id := g.ID()
		switch cmd.Action {
		case "use":
			c.ActiveGroup = id
		case "mute", "unmute", "rename":
			if cmd.Action == "rename" {
				if cmd.Name == "" {
					return c, fmt.Errorf("room name cannot be empty")
				}
				g.Name = cmd.Name
			} else {
				g.Muted = cmd.Action == "mute"
			}
			if id == c.RoomConfigs()[0].ID() {
				c.Name = g.Name
				c.ListenMuted = g.Muted
			} else {
				for i := range c.Groups {
					if c.Groups[i].ID() == id {
						c.Groups[i] = g
					}
				}
			}
		case "leave":
			if id == c.CurrentID() {
				return c, fmt.Errorf("select another room with group use before leaving the current room")
			}
			if id == c.RoomConfigs()[0].ID() {
				next := c.Groups[0]
				c.Key = next.Key
				c.Listen = next.Listen
				c.Peers = next.Peers
				c.Name = next.Name
				c.ListenMuted = next.Muted
				c.Groups = c.Groups[1:]
			} else {
				for i := range c.Groups {
					if c.Groups[i].ID() == id {
						c.Groups = append(c.Groups[:i], c.Groups[i+1:]...)
						break
					}
				}
			}
		default:
			return c, fmt.Errorf("unknown group action")
		}
	}
	return c, c.ValidateGroups()
}

// Rewrite is called while the profile lock is held, either by the daemon or an
// offline CLI. Rename commits a complete private file atomically.
func Rewrite(dir string, c Config) error {
	if err := c.ValidateGroups(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, filepath.Join(dir, "config.json"))
}
