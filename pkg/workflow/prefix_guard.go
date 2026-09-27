package workflow

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Every recursive delete of a Bellum prefix goes through removeBellumPrefix.
// It deletes nothing until it has shown that the directory is a Bellum
// prefix this user owns: the path is safe, the directory is real and owned
// by us, Bellum's ownership manifest names exactly this path, the contents
// match what the caller expects, and nothing inside is another filesystem.

// prefixProof is what a caller must see before removing a prefix.
type prefixProof int

const (
	// proofInstalled: an uninstall. The manifest plus either a Wine prefix
	// (system.reg, drive_c) or the install-incomplete marker.
	proofInstalled prefixProof = iota
	// proofIncomplete: replacing an unfinished install. The manifest plus
	// the install-incomplete marker.
	proofIncomplete
	// proofCreatedThisRun: rolling back a prefix this run created. The
	// manifest; without one only an empty directory is removed.
	proofCreatedThisRun
)

// Directories whose contents are never a Bellum prefix.
var forbiddenRoots = []string{"/proc", "/sys", "/dev", "/run", "/boot", "/etc", "/usr", "/bin", "/sbin", "/lib", "/lib64"}

// checkPrefixPath rejects paths that can't be a Bellum prefix, before
// anything is read from disk beyond resolving symlinks.
func checkPrefixPath(prefix string) (string, error) {
	if prefix == "" || !filepath.IsAbs(prefix) {
		return "", fmt.Errorf("refusing to remove %q: the Bellum folder must be an absolute path", prefix)
	}
	clean := filepath.Clean(prefix)
	if clean == "/" {
		return "", fmt.Errorf("refusing to remove %q: that is the root of the filesystem", clean)
	}
	if filepath.Base(clean) != "Bellum" {
		return "", fmt.Errorf("refusing to remove %q: a Bellum install folder is always named Bellum", clean)
	}
	for _, root := range forbiddenRoots {
		if within(clean, root) {
			return "", fmt.Errorf("refusing to remove %q: it is inside the system folder %s", clean, root)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if within(filepath.Clean(home), clean) {
			return "", fmt.Errorf("refusing to remove %q: it contains your home folder", clean)
		}
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot resolve %q: %w", clean, err)
	}
	if err == nil && resolved != clean {
		return "", fmt.Errorf("refusing to remove %q: it is, or sits under, a symlink (to %s)", clean, resolved)
	}
	return clean, nil
}

// within reports whether path is dir or inside it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// verifyBellumPrefix proves that prefix is a Bellum prefix owned by this user
// and holding what proof requires. It changes nothing.
func verifyBellumPrefix(prefix string, proof prefixProof, files FileStore) (string, error) {
	clean, err := checkPrefixPath(prefix)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("refusing to remove %q: it is not a real directory", clean)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return "", fmt.Errorf("refusing to remove %q: it belongs to another user", clean)
	}
	manifest := filepath.Join(clean, manifestName)
	if mi, err := os.Lstat(manifest); err != nil || !mi.Mode().IsRegular() {
		return "", fmt.Errorf("refusing to remove %q: no Bellum ownership manifest (%s), so it was not created by the Bellum installer", clean, manifestName)
	}
	if _, err := readManifest(clean, files); err != nil {
		return "", fmt.Errorf("refusing to remove %q: %w", clean, err)
	}
	incomplete := isRegular(filepath.Join(clean, incompleteMarkerName))
	winePrefix := isRegular(filepath.Join(clean, "system.reg")) && isRealDir(filepath.Join(clean, "drive_c"))
	switch proof {
	case proofInstalled:
		if !incomplete && !winePrefix {
			return "", fmt.Errorf("refusing to remove %q: it has a Bellum manifest but no Wine prefix (system.reg, drive_c)", clean)
		}
	case proofIncomplete:
		if !incomplete {
			return "", fmt.Errorf("refusing to remove %q: it is not an unfinished Bellum install", clean)
		}
	}
	return clean, nil
}

// removeBellumPrefix deletes a verified Bellum prefix. It refuses, before
// deleting anything, when a directory inside is on another filesystem
// (a mounted drive or bind mount), because removing it would delete data
// that lives outside the prefix. Symlinks inside are removed, never followed.
func removeBellumPrefix(prefix string, proof prefixProof, files FileStore) error {
	if proof == proofCreatedThisRun {
		clean, err := checkPrefixPath(prefix)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(clean); os.IsNotExist(err) {
			return nil
		}
		// Failed before the manifest was written: only an empty directory
		// can be ours, so remove it without recursion.
		if !isRegular(filepath.Join(clean, manifestName)) {
			if err := files.Remove(clean); err != nil {
				return fmt.Errorf("remove new prefix %q (left in place, not empty and has no Bellum manifest): %w", clean, err)
			}
			return nil
		}
	}
	clean, err := verifyBellumPrefix(prefix, proof, files)
	if err != nil {
		return err
	}
	if err := checkSingleFilesystem(clean); err != nil {
		return err
	}
	return files.RemoveAll(clean)
}

// checkSingleFilesystem walks root without following symlinks and fails if
// any directory is on a different device than root.
func checkSingleFilesystem(root string) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	rootStat, ok := rootInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Dev != rootStat.Dev {
			return fmt.Errorf("refusing to remove %q: %s is a mount point for another filesystem; unmount it first", root, path)
		}
		return nil
	})
}

func isRegular(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func isRealDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}
