package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// Bounded, cancellable platform detection assembly.

// errMalformed marks content that exists but cannot be parsed.
var errMalformed = errors.New("malformed content")

// errTooLarge marks reads refused by the byte bound before allocation.
var errTooLarge = errors.New("content exceeds read bound")

// errLinkCycle marks symlink loops inside the injected filesystem model.
var errLinkCycle = errors.New("symlink cycle")

// errLinkEscape marks a fixture symlink that would leave the fixture root.
// The fixture adapter never follows such links onto the real host.
var errLinkEscape = errors.New("symlink escapes fixture root")

// PartialError carries a collected partial snapshot alongside the error.
// Cancellation or invalid dependency configuration returns an error and any
// collected partial snapshot through this type.
type PartialError struct {
	Snapshot Platform
	Err      error
}

func (e *PartialError) Error() string { return e.Err.Error() }
func (e *PartialError) Unwrap() error { return e.Err }

// isMissing reports not-exist errors from either the host or fixtures.
func isMissing(err error) bool {
	return err != nil && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist))
}

// isNotDir reports non-directory errors when a directory was expected.
func isNotDir(err error) bool {
	if err == nil {
		return false
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return strings.Contains(strings.ToLower(pathErr.Err.Error()), "not a directory")
	}
	return strings.Contains(strings.ToLower(err.Error()), "not a directory")
}

// outcomeOf maps read failures to the stable diagnostic vocabulary.
func outcomeOf(err error) DiagnosticOutcome {
	switch {
	case err == nil:
		return OutcomeMissing
	case errors.Is(err, errTooLarge):
		return OutcomeUnsupported
	case errors.Is(err, errMalformed):
		return OutcomeMalformed
	case errors.Is(err, errLinkCycle), errors.Is(err, errLinkEscape):
		return OutcomeConflict
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return OutcomeTimeout
	case errors.Is(err, fs.ErrPermission) || errors.Is(err, os.ErrPermission) ||
		strings.Contains(strings.ToLower(err.Error()), "permission denied"):
		return OutcomePermission
	case isMissing(err):
		return OutcomeMissing
	default:
		return OutcomeUnsupported
	}
}

// readBounded reads one file through the injected filesystem with a byte
// cap. Callers pass the design's limits (1 MiB for text/JSON, 64 KiB for
// os-release, small caps for sysfs/proc single values).
func readBounded(fsys DetectionFS, name string, limit int) ([]byte, error) {
	if limit <= 0 || limit > 8<<20 {
		limit = ReadLimitText
	}
	if fsys == nil {
		return nil, fmt.Errorf("no filesystem dependency: %w", fs.ErrInvalid)
	}
	// Refuse oversized files before unbounded allocation when size metadata
	// is available.
	if info, serr := fsys.Stat(name); serr == nil && info != nil && info.Size() > int64(limit) {
		return nil, fmt.Errorf("%s: %w", name, errTooLarge)
	}
	raw, err := fsys.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		return nil, fmt.Errorf("%s: %w", name, errTooLarge)
	}
	return raw, nil
}

// readDirNames lists one directory through the injected filesystem with
// sorted, bounded results.
func readDirNames(fsys DetectionFS, dir string) ([]string, error) {
	if fsys == nil {
		return nil, fmt.Errorf("no filesystem dependency: %w", fs.ErrInvalid)
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for i, e := range entries {
		if i >= MaxScanEntries {
			return nil, fmt.Errorf("%s: truncated at %d entries: %w", dir, MaxScanEntries, errTooLarge)
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func pathExists(fsys DetectionFS, name string) bool {
	if fsys == nil {
		return false
	}
	_, err := fsys.Stat(name)
	return err == nil
}

func isDir(fsys DetectionFS, name string) (bool, error) {
	if fsys == nil {
		return false, fmt.Errorf("no filesystem dependency: %w", fs.ErrInvalid)
	}
	info, err := fsys.Stat(name)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// resolveLink resolves absolute and relative symlinks lexically inside the
// injected filesystem (at most 32 hops). It never calls filepath.EvalSymlinks
// on the host, never uses the test process's HOME, and never executes a
// probe. Cycles and escaping links are explicit errors.
func resolveLink(fsys DetectionFS, name string) (string, error) {
	if fsys == nil {
		return "", fmt.Errorf("no filesystem dependency: %w", fs.ErrInvalid)
	}
	current := path.Clean("/" + strings.TrimPrefix(path.Clean(name), "/"))
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		if seen[current] {
			return "", fmt.Errorf("%s: %w", name, errLinkCycle)
		}
		seen[current] = true
		target, err := fsys.Readlink(current)
		if err != nil {
			if isMissing(err) {
				return current, nil
			}
			return "", err
		}
		if strings.TrimSpace(target) == "" {
			return "", fmt.Errorf("%s: %w", name, errMalformed)
		}
		if strings.HasPrefix(target, "/") {
			current = path.Clean(target)
		} else {
			current = path.Join(path.Dir(current), target)
		}
		if !strings.HasPrefix(current, "/") {
			return "", fmt.Errorf("%s: %w", name, errLinkEscape)
		}
	}
	return "", fmt.Errorf("%s: %w", name, errLinkCycle)
}

// osDetectionFS is the production adapter. It enforces the design's byte
// limits in the adapter itself (size check plus a capped reader), not only
// after os.ReadFile has allocated unbounded data.
type osDetectionFS struct{}

func (osDetectionFS) ReadFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, serr := f.Stat(); serr == nil && info.Size() > ReadLimitText {
		return nil, fmt.Errorf("%s: %w", name, errTooLarge)
	}
	raw, err := io.ReadAll(io.LimitReader(f, ReadLimitText+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > ReadLimitText {
		return nil, fmt.Errorf("%s: %w", name, errTooLarge)
	}
	return raw, nil
}

func (osDetectionFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (osDetectionFS) Stat(name string) (fs.FileInfo, error)      { return os.Stat(name) }
func (osDetectionFS) Readlink(name string) (string, error) {
	target, err := os.Readlink(name)
	if err != nil {
		// Not a link (or missing): resolveLink treats this as terminal.
		if os.IsNotExist(err) {
			return "", err
		}
		if linkErr, ok := err.(*os.LinkError); ok && linkErr != nil {
			return "", err
		}
		return "", err
	}
	return target, nil
}

// DetectPlatform is the production wrapper: it supplies the OS-backed
// filesystem, process environment, home directory, owner UID and the
// cancellable bounded step-1 GPU probe. Only this wrapper reaches the host.
func DetectPlatform(ctx context.Context) (Platform, error) {
	home, _ := os.UserHomeDir()
	return DetectPlatformWith(ctx, DetectionSources{
		Files:    osDetectionFS{},
		Env:      os.Getenv,
		Home:     home,
		GPU:      ProductionGPUProbe,
		EUID:     os.Geteuid(),
		HaveEUID: true,
	})
}

// DetectPlatformWith assembles the snapshot from explicit dependencies only.
// Missing optional host evidence yields a usable partial Platform plus
// diagnostics, never an installation failure. Cancellation or invalid
// dependency configuration returns a *PartialError with any collected
// partial snapshot.
func DetectPlatformWith(ctx context.Context, sources DetectionSources) (Platform, error) {
	var platform Platform
	if sources.Files == nil || sources.Env == nil || sources.GPU == nil {
		return platform, &PartialError{Snapshot: platform, Err: fmt.Errorf("DetectPlatformWith requires Files, Env and GPU dependencies")}
	}
	if err := ctx.Err(); err != nil {
		return platform, &PartialError{Snapshot: platform, Err: err}
	}

	var diags []DetectionDiagnostic

	osInfo, osDiags := detectOS(ctx, sources)
	platform.OS = osInfo
	diags = append(diags, osDiags...)

	caps, probe, gerr := sources.GPU(ctx)
	if gerr != nil {
		if ctx.Err() != nil {
			platform.Diagnostics = finalizeDiagnostics(diags)
			return platform, &PartialError{Snapshot: platform, Err: ctx.Err()}
		}
		caps = GPUCapabilities{Vendor: GPUUnknown, Renderer: "undetected: " + SanitizeField(gerr.Error(), 256)}
		probe = GPUProbeInfo{Source: ProbeUnknown, DeviceAssociation: "unknown", Provenance: "step1:error"}
		diags = append(diags, DetectionDiagnostic{Code: "gpu/probe-failed", Subsystem: "gpu", Source: "gpu:probe", Outcome: OutcomeUnsupported, Detail: SanitizeField("GPU probe failed; continuing with unknown GPU: "+gerr.Error(), MaxDetailLen)})
	}
	caps.Renderer = SanitizeField(caps.Renderer, MaxRendererLen)
	probe.Renderer = SanitizeField(probe.Renderer, MaxRendererLen)
	platform.GPU = caps
	platform.GPUProbe = probe

	hardware, hwDiags := ClassifyHardware(
		readDMIValue(sources.Files, "/sys/class/dmi/id/sys_vendor"),
		readDMIValue(sources.Files, "/sys/class/dmi/id/product_name"),
		caps, probe,
	)
	platform.Hardware = hardware
	diags = append(diags, hwDiags...)

	var icdLibs []string
	nvidia, nvDiags := DetectNVIDIA(ctx, sources.Files, caps.Renderer, nil)
	platform.NVIDIA = nvidia
	diags = append(diags, nvDiags...)

	vulkan, vkDiags := DetectVulkan(ctx, sources.Files, sources.Home, sources.Env)
	platform.NVIDIA.Vulkan = vulkan
	for _, m := range vulkan.Manifests {
		if m.ParseState == "ok" && m.LibraryPath != "" {
			icdLibs = append(icdLibs, m.LibraryPath)
		}
	}
	// Re-resolve userspace once ICD inventory is known.
	platform.NVIDIA.Userspace = classifyNVIDIAUserspace(caps.Renderer, icdLibs, platform.NVIDIA.Kernel)
	diags = append(diags, vkDiags...)

	platform.Session = ClassifySessionFunc(sources.Env)

	steam, steamDiags := DetectSteam(ctx, sources.Files, sources.Home, sources.EUID, sources.HaveEUID)
	platform.Steam = steam
	diags = append(diags, steamDiags...)

	controllers, ctlDiags := DetectControllers(ctx, sources.Files)
	platform.Controllers = controllers
	diags = append(diags, ctlDiags...)

	if err := ctx.Err(); err != nil {
		platform.Diagnostics = finalizeDiagnostics(diags)
		return platform, &PartialError{Snapshot: platform, Err: err}
	}
	if platform.Steam.Installs == nil {
		platform.Steam.Installs = []SteamInstall{}
	}
	if platform.Controllers.Devices == nil {
		platform.Controllers.Devices = []ControllerDevice{}
	}
	if platform.NVIDIA.Vulkan.Manifests == nil {
		platform.NVIDIA.Vulkan.Manifests = []VulkanManifest{}
	}
	if platform.OS.IDLike == nil {
		platform.OS.IDLike = []string{}
	}
	platform.Diagnostics = finalizeDiagnostics(diags)
	return platform, nil
}

func finalizeDiagnostics(diags []DetectionDiagnostic) []DetectionDiagnostic {
	if diags == nil {
		return []DetectionDiagnostic{}
	}
	SortDiagnostics(diags)
	return diags
}

// detectOS reads the preferred os-release file and falls back to
// /usr/lib/os-release only when the former does not exist. An unreadable or
// malformed preferred file produces a diagnostic rather than silently
// replacing its identity.
func detectOS(ctx context.Context, sources DetectionSources) (OSInfo, []DetectionDiagnostic) {
	info := OSInfo{Family: OSUnknown, Immutable: TriUnknown}
	var diags []DetectionDiagnostic
	if err := ctx.Err(); err != nil {
		return info, diags
	}
	data, preferredErr := readBounded(sources.Files, osReleasePaths[0], ReadLimitOSRelease)
	info.SourcePath = osReleasePaths[0]
	if preferredErr != nil {
		if isMissing(preferredErr) {
			data, preferredErr = readBounded(sources.Files, osReleasePaths[1], ReadLimitOSRelease)
			info.SourcePath = osReleasePaths[1]
		}
		if preferredErr != nil {
			diags = append(diags, DetectionDiagnostic{Code: "os/os-release-unreadable", Subsystem: "os", Source: info.SourcePath, Outcome: outcomeOf(preferredErr), Detail: "os-release unreadable; OS identity unknown"})
			info.ImmutableEvidence = "no distro identity evidence"
			return info, diags
		}
	}
	id, _, variant, version, pretty, tokens, parseDiags := ParseOSRelease(data)
	for i := range parseDiags {
		parseDiags[i].Source = info.SourcePath
	}
	diags = append(diags, parseDiags...)
	if strings.TrimSpace(id) == "" {
		diags = append(diags, DetectionDiagnostic{Code: "os/os-release-malformed", Subsystem: "os", Source: info.SourcePath, Outcome: OutcomeMalformed, Detail: "os-release has no usable ID; OS identity unknown"})
	}
	ostreeBooted := pathExists(sources.Files, "/run/ostree-booted")
	family := ClassifyOSFamily(id, tokens)
	immutable, evidence := ClassifyImmutable(id, variant, family, ostreeBooted)
	info.ID = id
	info.IDLike = tokens
	if info.IDLike == nil {
		info.IDLike = []string{}
	}
	info.VariantID = variant
	info.VersionID = version
	info.PrettyName = SanitizeField(pretty, MaxRawFieldLen)
	info.Family = family
	info.Immutable = immutable
	info.ImmutableEvidence = evidence
	return info, diags
}

// readDMIValue trims NUL/whitespace and sanitizes DMI evidence. Failures
// yield an empty string; ClassifyHardware records the missing evidence.
func readDMIValue(fsys DetectionFS, name string) string {
	raw, err := readBounded(fsys, name, ReadLimitSmall)
	if err != nil {
		return ""
	}
	return SanitizeDMI(string(raw))
}
