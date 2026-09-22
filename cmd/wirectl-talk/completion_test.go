package main

import (
	"strings"
	"testing"
)

func TestCompletionScripts(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   []string
	}{
		{name: "bash", script: bashCompletionScript, want: []string{"_wirectl_talk_completion", "complete -o", "daemon", "--state-dir"}},
		{name: "zsh", script: zshCompletionScript, want: []string{"_wirectl_talk_completion", "compdef", "_arguments", "daemon", "--state-dir"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, want := range test.want {
				if !strings.Contains(test.script, want) {
					t.Errorf("completion script does not contain %q", want)
				}
			}
		})
	}
}

func TestCompletionCommandRejectsUnsupportedShell(t *testing.T) {
	if err := completionCommand([]string{"fish"}); err == nil {
		t.Fatal("completionCommand(fish) returned nil")
	}
}
