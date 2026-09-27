package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"bellum-installer/pkg/core"
)

// recordingFiles records deletes instead of performing them.
type recordingFiles struct {
	osFiles
	removed []string
}

func (r *recordingFiles) RemoveAll(p string) error { r.removed = append(r.removed, p); return nil }
func (r *recordingFiles) Remove(p string) error    { r.removed = append(r.removed, p); return nil }

func makeBellumPrefix(t *testing.T, parent string) string {
	t.Helper()
	prefix := filepath.Join(parent, "Bellum")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), []byte("WINE REGISTRY Version 2"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(prefix, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	return prefix
}

func TestRemoveBellumPrefixRefusesDangerousPaths(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home", "user")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	// A home folder that happens to be called Bellum.
	bellumHome := filepath.Join(root, "Bellum")
	if err := os.MkdirAll(bellumHome, 0700); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"root":             "/",
		"empty":            "",
		"relative":         "Games/Bellum",
		"home":             home,
		"home parent":      filepath.Dir(home),
		"not named Bellum": filepath.Join(home, "Games"),
		"dot-dot to root":  filepath.Join(home, "..", "..", "..", "..", ".."),
		"system folder":    "/usr/Bellum",
		"proc":             "/proc/Bellum",
	}
	for name, path := range cases {
		for _, proof := range []prefixProof{proofInstalled, proofIncomplete, proofCreatedThisRun} {
			files := &recordingFiles{}
			if err := removeBellumPrefix(path, proof, files); err == nil {
				t.Errorf("%s (%q, proof %d): accepted", name, path, proof)
			}
			if len(files.removed) != 0 {
				t.Errorf("%s (%q): deleted %v", name, path, files.removed)
			}
		}
	}

	t.Setenv("HOME", filepath.Join(bellumHome, "me"))
	files := &recordingFiles{}
	if err := removeBellumPrefix(bellumHome, proofInstalled, files); err == nil || len(files.removed) != 0 {
		t.Fatalf("a Bellum folder containing home was accepted: %v %v", err, files.removed)
	}
}

func TestRemoveBellumPrefixRequiresOwnershipProof(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	// A folder named Bellum with the user's own files and no manifest.
	userFolder := filepath.Join(root, "docs", "Bellum")
	if err := os.MkdirAll(filepath.Join(userFolder, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userFolder, "system.reg"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, proof := range []prefixProof{proofInstalled, proofIncomplete, proofCreatedThisRun} {
		if proof == proofCreatedThisRun {
			// Rollback without a manifest may only rmdir, which fails on a
			// non-empty folder; never a recursive delete.
			if err := removeBellumPrefix(userFolder, proof, osFiles{}); err == nil {
				t.Fatal("rollback removed a non-empty folder without a manifest")
			}
			continue
		}
		files := &recordingFiles{}
		if err := removeBellumPrefix(userFolder, proof, files); err == nil || len(files.removed) != 0 {
			t.Fatalf("proof %d: folder without manifest accepted: %v %v", proof, err, files.removed)
		}
	}
	if _, err := os.Stat(filepath.Join(userFolder, "system.reg")); err != nil {
		t.Fatalf("user files were touched: %v", err)
	}

	// A manifest that names a different folder (copied prefix).
	copied := filepath.Join(root, "copy", "Bellum")
	if err := os.MkdirAll(filepath.Join(copied, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	orig := makeBellumPrefix(t, filepath.Join(root, "orig"))
	data, _ := os.ReadFile(filepath.Join(orig, manifestName))
	_ = os.WriteFile(filepath.Join(copied, manifestName), data, 0600)
	_ = os.WriteFile(filepath.Join(copied, "system.reg"), []byte("x"), 0600)
	if err := removeBellumPrefix(copied, proofInstalled, &recordingFiles{}); err == nil {
		t.Fatal("manifest for another folder accepted")
	}

	// A manifest that is a symlink.
	linked := filepath.Join(root, "linked", "Bellum")
	if err := os.MkdirAll(filepath.Join(linked, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(linked, "system.reg"), []byte("x"), 0600)
	if err := os.Symlink(filepath.Join(orig, manifestName), filepath.Join(linked, manifestName)); err != nil {
		t.Fatal(err)
	}
	if err := removeBellumPrefix(linked, proofInstalled, &recordingFiles{}); err == nil {
		t.Fatal("symlinked manifest accepted")
	}

	// Manifest but no Wine prefix and no install marker.
	bare := filepath.Join(root, "bare", "Bellum")
	if err := os.MkdirAll(bare, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(bare, DefaultBoundaries.Files); err != nil {
		t.Fatal(err)
	}
	if err := removeBellumPrefix(bare, proofInstalled, &recordingFiles{}); err == nil {
		t.Fatal("manifest without Wine prefix accepted for uninstall")
	}
	// A finished install is not an unfinished one.
	if err := removeBellumPrefix(orig, proofIncomplete, &recordingFiles{}); err == nil {
		t.Fatal("finished install accepted as unfinished")
	}
}

func TestRemoveBellumPrefixRefusesSymlinks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	prefix := makeBellumPrefix(t, filepath.Join(root, "real"))

	linkParent := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "real"), linkParent); err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(root, "other", "Bellum")
	_ = os.MkdirAll(filepath.Dir(direct), 0700)
	if err := os.Symlink(prefix, direct); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(linkParent, "Bellum"), direct} {
		files := &recordingFiles{}
		if err := removeBellumPrefix(path, proofInstalled, files); err == nil || len(files.removed) != 0 {
			t.Fatalf("%s: symlinked path accepted: %v %v", path, err, files.removed)
		}
	}
}

func TestRemoveBellumPrefixRemovesRealPrefixButNotLinkTargets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	prefix := makeBellumPrefix(t, filepath.Join(root, "games"))
	outside := filepath.Join(root, "precious")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "keep"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	// Wine's dosdevices/z: points at /; links must be removed, not followed.
	if err := os.MkdirAll(filepath.Join(prefix, "dosdevices"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(prefix, "dosdevices", "z:")); err != nil {
		t.Fatal(err)
	}
	if err := removeBellumPrefix(prefix, proofInstalled, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(prefix); !os.IsNotExist(err) {
		t.Fatalf("prefix remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "keep")); err != nil {
		t.Fatalf("symlink target was deleted: %v", err)
	}
}

func TestRollbackOfNewPrefixWithoutManifestOnlyRemovesEmptyDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	prefix := filepath.Join(root, "Bellum")
	if err := os.Mkdir(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := rollbackNewPrefix(prefix, true, osFiles{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(prefix); !os.IsNotExist(err) {
		t.Fatal("empty new prefix not removed")
	}
	if err := os.MkdirAll(filepath.Join(prefix, "stuff"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := rollbackNewPrefix(prefix, true, osFiles{}); err == nil {
		t.Fatal("non-empty prefix without manifest removed")
	}
	if _, err := os.Stat(filepath.Join(prefix, "stuff")); err != nil {
		t.Fatal("contents touched")
	}
}

func TestCheckSingleFilesystemDetectsMountPoints(t *testing.T) {
	var dev, pts syscall.Stat_t
	if syscall.Lstat("/dev", &dev) != nil || syscall.Lstat("/dev/pts", &pts) != nil || dev.Dev == pts.Dev {
		t.Skip("no mount point below /dev to test against")
	}
	err := checkSingleFilesystem("/dev")
	if err == nil || !strings.Contains(err.Error(), "mount point") {
		t.Fatalf("mount point not detected: %v", err)
	}
	if err := checkSingleFilesystem(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallRefusesRootAndHome(t *testing.T) {
	logger, err := core.NewLogger("")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, path := range []string{"/", root, filepath.Join(root, "..")} {
		files := &recordingFiles{}
		err := RunUninstallationWithBoundaries(UninstallConfig{WINEPREFIX: path}, logger, WorkflowBoundaries{Files: files, Commands: fakeCommands{}})
		if err == nil || len(files.removed) != 0 {
			t.Fatalf("%q: uninstall accepted: %v, removed %v", path, err, files.removed)
		}
	}
}
