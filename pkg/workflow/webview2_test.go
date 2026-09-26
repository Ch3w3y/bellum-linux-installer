package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWebView2Runtime(t *testing.T) {
	prefix := t.TempDir()
	if err := checkWebView2Runtime(prefix); err == nil {
		t.Fatal("bootstrapper-only prefix should be rejected")
	}
	runtimeDir := filepath.Join(prefix, "drive_c", "Program Files (x86)", "Microsoft", "EdgeWebView", "Application", "123.0.0.0")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "msedgewebview2.exe"), []byte("runtime"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkWebView2Runtime(prefix); err != nil {
		t.Fatal(err)
	}
}
