package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalBinPathHint returns "" when ~/.local/bin (where the Bellum command is
// installed) is on PATH, and otherwise the exact command that adds it for the
// user's shell.
func LocalBinPathHint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return localBinPathHint(home, os.Getenv("PATH"), os.Getenv("SHELL"))
}

func localBinPathHint(home, path, shell string) string {
	bin := filepath.Join(home, ".local", "bin")
	for _, dir := range filepath.SplitList(path) {
		if filepath.Clean(dir) == bin {
			return ""
		}
	}
	switch filepath.Base(shell) {
	case "fish":
		return "fish_add_path ~/.local/bin"
	case "zsh":
		return `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && exec zsh`
	default:
		return fmt.Sprintf(`echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.%src && exec %s`, shellName(shell), shellName(shell))
	}
}

func shellName(shell string) string {
	name := filepath.Base(shell)
	if name == "" || name == "." || name == "/" || strings.ContainsAny(name, " '\"") {
		return "bash"
	}
	return name
}
