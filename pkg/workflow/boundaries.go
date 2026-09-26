package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/core"
	"bellum-installer/pkg/launchers"
	"bellum-installer/pkg/packages"
)

// CommandRunner isolates host command execution for workflow tests.
type CommandRunner interface {
	Run(core.RunMode, []string, *core.Logger, string) error
	Output([]string) (string, error)
	LookPath(string) string
}

// FileStore isolates filesystem discovery and mutations.
type FileStore interface {
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.DirEntry, error)
	MkdirAll(string, os.FileMode) error
	RemoveAll(string) error
	Remove(string) error
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, os.FileMode) error
}

// WorkflowBoundaries groups effects that used to be reached directly by each phase.
// Tests can supply fakes; production workflows use DefaultBoundaries.
type WorkflowBoundaries struct {
	Commands         CommandRunner
	Files            FileStore
	AcquirePackage   func(string, *core.Logger) (*packages.LauncherInstallerState, error)
	MutatePrefix     func(core.RunMode, []string, *core.Logger, string) error
	GenerateLauncher func(launchers.LauncherConfig) error
	VerifyFile       func(string, string) error
	ExtractPackage   func(string, string) (string, error)
	CleanupPackage   func(string)
}

type systemCommands struct{}

func (systemCommands) Run(m core.RunMode, a []string, l *core.Logger, p string) error {
	return core.RunCommand(m, a, l, p)
}
func (systemCommands) Output(a []string) (string, error) { return core.RunCommandWithOutput(a) }
func (systemCommands) LookPath(s string) string          { return core.LookPath(s) }

type osFiles struct{}

func (osFiles) Stat(p string) (os.FileInfo, error)                { return os.Stat(p) }
func (osFiles) ReadDir(p string) ([]os.DirEntry, error)           { return os.ReadDir(p) }
func (osFiles) MkdirAll(p string, m os.FileMode) error            { return os.MkdirAll(p, m) }
func (osFiles) RemoveAll(p string) error                          { return os.RemoveAll(p) }
func (osFiles) Remove(p string) error                             { return os.Remove(p) }
func (osFiles) ReadFile(p string) ([]byte, error)                 { return os.ReadFile(p) }
func (osFiles) WriteFile(p string, b []byte, m os.FileMode) error { return os.WriteFile(p, b, m) }

// GuardGameTreeWrites rejects filesystem mutations aimed inside Bellum's game
// install tree. Prefix files and user-level launcher files remain writable.
// The optional game path is explicit so callers cannot accidentally infer the
// game directory from the Wine prefix.
type GuardGameTreeWrites struct {
	FileStore
	GameInstallDir string
}

func (g GuardGameTreeWrites) check(path string) error {
	root, err := filepath.Abs(filepath.Clean(g.GameInstallDir))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("refusing to mutate Bellum game install tree: %s", path)
	}
	return nil
}

func (g GuardGameTreeWrites) MkdirAll(p string, m os.FileMode) error {
	if err := g.check(p); err != nil {
		return err
	}
	return g.FileStore.MkdirAll(p, m)
}
func (g GuardGameTreeWrites) RemoveAll(p string) error {
	if err := g.check(p); err != nil {
		return err
	}
	return g.FileStore.RemoveAll(p)
}
func (g GuardGameTreeWrites) Remove(p string) error {
	if err := g.check(p); err != nil {
		return err
	}
	return g.FileStore.Remove(p)
}
func (g GuardGameTreeWrites) WriteFile(p string, b []byte, m os.FileMode) error {
	if err := g.check(p); err != nil {
		return err
	}
	return g.FileStore.WriteFile(p, b, m)
}

var DefaultBoundaries = WorkflowBoundaries{
	Commands: systemCommands{}, Files: osFiles{},
	AcquirePackage:   packages.DownloadLauncherInstaller,
	MutatePrefix:     core.RunCommand,
	GenerateLauncher: launchers.GenerateLauncher,
	VerifyFile:       packages.VerifySHA256,
	ExtractPackage:   packages.ExtractPackage,
	CleanupPackage:   packages.CleanupTempDir,
}

// DiscoverExecutable is a small host-discovery seam shared by prechecks.
func DiscoverExecutable(name string, commands CommandRunner) string {
	if commands == nil {
		commands = DefaultBoundaries.Commands
	}
	return commands.LookPath(name)
}
