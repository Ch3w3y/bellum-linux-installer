package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogFileRejectsSymlinkAndPrivateDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "logs")
	path := filepath.Join(dir, "installer.log")
	f, err := OpenLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("log dir mode %o", info.Mode().Perm())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "target"), path); err != nil {
		t.Fatal(err)
	}
	if f, err := OpenLogFile(path); err == nil {
		f.Close()
		t.Fatal("log symlink accepted")
	}
}
