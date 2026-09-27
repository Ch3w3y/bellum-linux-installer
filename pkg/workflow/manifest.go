package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"bellum-installer/pkg/core"
)

// Manifest records the instance-specific resources Bellum owns. The prefix is
// the only recursively removable resource; Proton is shared by all instances.
type Manifest struct {
	Version int      `json:"version"`
	Prefix  string   `json:"prefix"`
	Owned   []string `json:"owned"`
}

const manifestName = ".bellum-manifest.json"

func writeManifest(prefix string, files FileStore) error {
	canonical, err := filepath.Abs(prefix)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(Manifest{Version: 1, Prefix: canonical, Owned: []string{"prefix"}}, "", "  ")
	if err != nil {
		return err
	}
	return files.WriteFile(filepath.Join(prefix, manifestName), b, 0600)
}

func readManifest(prefix string, files FileStore) (Manifest, error) {
	b, err := files.ReadFile(filepath.Join(prefix, manifestName))
	if err != nil {
		return Manifest{}, fmt.Errorf("Bellum ownership manifest missing: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("invalid Bellum ownership manifest: %w", err)
	}
	abs, err := filepath.Abs(prefix)
	if err != nil {
		return Manifest{}, err
	}
	if m.Version != 1 || filepath.Clean(m.Prefix) != filepath.Clean(abs) || len(m.Owned) != 1 || m.Owned[0] != "prefix" {
		return Manifest{}, fmt.Errorf("ownership manifest does not authorize this prefix")
	}
	return m, nil
}

func rollbackNewPrefix(prefix string, created bool, files FileStore) error {
	if !created {
		return nil
	}
	if _, err := os.Lstat(prefix); os.IsNotExist(err) {
		return nil
	}
	return files.RemoveAll(prefix)
}

// incompleteMarkerName is written next to the manifest when an install starts
// and removed only once installation and configuration have both finished. A
// prefix carrying it can be replaced by the next installer run.
const incompleteMarkerName = ".bellum-install-incomplete"

func writeIncompleteMarker(prefix string, files FileStore) error {
	return files.WriteFile(filepath.Join(prefix, incompleteMarkerName), []byte("installation in progress\n"), 0600)
}

// MarkInstallComplete records that installation and configuration finished.
func MarkInstallComplete(prefix string) error {
	err := DefaultBoundaries.Files.Remove(filepath.Join(prefix, incompleteMarkerName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// DiscardIncompleteInstall removes a prefix only when it carries both a valid
// Bellum manifest and the install-incomplete marker, i.e. an install that this
// installer started and never finished.
// Launcher assets that point at the discarded prefix are removed with it.
func DiscardIncompleteInstall(prefix string, logger *core.Logger) error {
	if err := discardIncompleteInstallWith(prefix, DefaultBoundaries.Files); err != nil {
		return err
	}
	return removeOwnedLauncherAssets(prefix, logger, DefaultBoundaries.Files, DefaultBoundaries.Commands)
}

func discardIncompleteInstallWith(prefix string, files FileStore) error {
	state, err := inspectPrefix(prefix, files)
	if err != nil {
		return err
	}
	switch state {
	case prefixAbsent:
		return nil
	case prefixIncomplete:
		return files.RemoveAll(prefix)
	default:
		return fmt.Errorf("refusing to remove %s: it is not an unfinished Bellum install", prefix)
	}
}
