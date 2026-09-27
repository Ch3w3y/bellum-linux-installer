package workflow

import (
	"os"
	"testing"

	"bellum-installer/pkg/core"
)

// Install and update tests must not download the real Astarte Launcher.
func TestMain(m *testing.M) {
	launcherUpdater = func(string, *core.Logger) error { return nil }
	os.Exit(m.Run())
}
