package workflow

import (
	"path/filepath"
	"testing"
)

func TestUpdateLauncherInPrefixRequiresABellumInstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	for _, prefix := range []string{"/", filepath.Join(root, "Bellum"), filepath.Join(root, "Other")} {
		if err := UpdateLauncherInPrefix(prefix, nil); err == nil {
			t.Fatalf("%q accepted", prefix)
		}
	}
}
