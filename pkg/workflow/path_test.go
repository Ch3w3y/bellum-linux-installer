package workflow

import (
	"strings"
	"testing"
)

func TestLocalBinPathHint(t *testing.T) {
	if hint := localBinPathHint("/home/u", "/usr/bin:/home/u/.local/bin/", "/bin/bash"); hint != "" {
		t.Fatalf("on PATH, got hint %q", hint)
	}
	if hint := localBinPathHint("/home/u", "/usr/bin", "/bin/bash"); !strings.Contains(hint, "~/.bashrc") {
		t.Fatalf("bash hint: %q", hint)
	}
	if hint := localBinPathHint("/home/u", "/usr/bin", "/usr/bin/zsh"); !strings.Contains(hint, "~/.zshrc") {
		t.Fatalf("zsh hint: %q", hint)
	}
	if hint := localBinPathHint("/home/u", "/usr/bin", "/usr/bin/fish"); hint != "fish_add_path ~/.local/bin" {
		t.Fatalf("fish hint: %q", hint)
	}
	if hint := localBinPathHint("/home/u", "/usr/bin", ""); !strings.Contains(hint, "~/.bashrc") {
		t.Fatalf("default hint: %q", hint)
	}
}
