package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"bellum-installer/pkg/core"
)

func TestUninstallIsRepeatableAndPreservesOtherInstancesAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	parent := t.TempDir()
	prefix := filepath.Join(parent, "Bellum")
	other := filepath.Join(t.TempDir(), "Bellum")
	for _, p := range []string{prefix, other} {
		if err := os.MkdirAll(filepath.Join(p, "drive_c"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "system.reg"), []byte("wine"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := writeManifest(p, DefaultBoundaries.Files); err != nil {
			t.Fatal(err)
		}
	}
	launcher := filepath.Join(home, ".local", "bin", "Bellum")
	apps := filepath.Join(home, ".local", "share", "applications", "Bellum.desktop")
	desktop := filepath.Join(home, "Desktop", "Bellum.desktop")
	icon := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "bellum.png")
	for _, path := range []string{launcher, apps, desktop, icon} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n# "+prefix+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(apps, []byte("[Desktop Entry]\nPath="+prefix+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktop, []byte("[Desktop Entry]\nPath="+other+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("icon"), 0644); err != nil {
		t.Fatal(err)
	}
	logger, _ := core.NewLogger("")
	run := func() error {
		input, err := os.CreateTemp(t.TempDir(), "confirm")
		if err != nil {
			return err
		}
		defer input.Close()
		if _, err := input.WriteString("y\n"); err != nil {
			return err
		}
		if _, err := input.Seek(0, 0); err != nil {
			return err
		}
		old := os.Stdin
		os.Stdin = input
		defer func() { os.Stdin = old }()
		return RunUninstallationWithBoundaries(UninstallConfig{WINEPREFIX: prefix}, logger, WorkflowBoundaries{Files: DefaultBoundaries.Files, Commands: transactionCommands{}})
	}
	if err := run(); err != nil {
		t.Fatalf("first uninstall failed: %v", err)
	}
	if err := run(); err != nil {
		t.Fatalf("repeat uninstall failed: %v", err)
	}
	if _, err := os.Stat(prefix); !os.IsNotExist(err) {
		t.Fatalf("prefix still exists: %v", err)
	}
	if _, err := os.Stat(launcher); !os.IsNotExist(err) {
		t.Fatalf("owned launcher remains: %v", err)
	}
	if _, err := os.Stat(apps); !os.IsNotExist(err) {
		t.Fatalf("owned desktop entry remains: %v", err)
	}
	if _, err := os.Stat(desktop); err != nil {
		t.Fatalf("other instance desktop was removed: %v", err)
	}
	if _, err := os.Stat(icon); !os.IsNotExist(err) {
		t.Fatalf("owned icon remains: %v", err)
	}
}
