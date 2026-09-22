package service

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNativeServiceDefinitions(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		p, err := Build(goos, "/home/A B", "/config", "1234", "/app/a & b/talk", "/state/a & b")
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Start) == 0 || len(p.Stop) == 0 {
			t.Fatal("missing lifecycle commands")
		}
		if !strings.Contains(p.Content, "__serve") {
			t.Fatal("service does not run audio")
		}
		if goos != "linux" {
			d := xml.NewDecoder(strings.NewReader(p.Content))
			for {
				_, e := d.Token()
				if e != nil {
					if e.Error() != "EOF" {
						t.Fatal(e)
					}
					break
				}
			}
		}
		if goos == "windows" && !strings.Contains(p.Content, "InteractiveToken") {
			t.Fatal("audio cannot run in session 0")
		}
	}
}
func TestServiceRejectsLineInjection(t *testing.T) {
	if _, err := Build("linux", "/home", "/config", "1", "/bin/talk", "/state\nExecStart=bad"); err == nil {
		t.Fatal("accepted injected unit")
	}
}
