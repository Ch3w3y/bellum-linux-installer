package workflow

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNoInstallerCodeWritesGameTree(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "../.."))
	if _, err := os.Stat(filepath.Join(root, "packages", "fsr4")); !os.IsNotExist(err) {
		t.Fatal("vendored FSR DLL directory must be absent")
	}
	for _, rel := range []string{"bellum-installer/main.go", "pkg/workflow/configure.go", "pkg/workflow/install.go", "pkg/launchers/generator.go"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		if strings.Contains(source, "UpgradeFSR") || strings.Contains(source, "--fsr41") || strings.Contains(source, `"packages", "fsr4"`) {
			t.Fatalf("%s contains removed game write or flag", rel)
		}
	}
}
