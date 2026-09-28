package core

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// This file defines the step-2 platform detection contracts accepted in SOF-4
// (design document `design`, revision 1 d3d4cc24). The design text is the
// complete API/semantic/fixture contract; the comments below restate the
// binding semantics so reviewers can check the code against them without
// re-reading the issue thread.
//
// Convention: Platform is a read-only-by-convention snapshot. Profiles
// describe the machine; they are never a user-selected preset.

// TriState is a yes/no/unknown value. The zero value ("") normalizes to
// unknown so unset observations can never masquerade as absence or presence.
type TriState string

const (
	TriYes     TriState = "yes"
	TriNo      TriState = "no"
	TriUnknown TriState = "unknown"
)

// Normalize maps the zero value (and anything unrecognized) to unknown.
func (t TriState) Normalize() TriState {
	switch t {
	case TriYes, TriNo, TriUnknown:
		return t
	default:
		return TriUnknown
	}
}

// OSFamily is the package-manager family derived from os-release identity.
// It is separate from the distro ID, which is preserved verbatim.
type OSFamily string

const (
	OSArch     OSFamily = "arch"
	OSFedora   OSFamily = "fedora"
	OSDebian   OSFamily = "debian"
	OSOpenSUSE OSFamily = "opensuse"
	OSUnknown  OSFamily = "unknown"
)

// Normalize maps unset or unrecognized families to unknown.
func (f OSFamily) Normalize() OSFamily {
	switch f {
	case OSArch, OSFedora, OSDebian, OSOpenSUSE, OSUnknown:
		return f
	default:
		return OSUnknown
	}
}

// HardwareSKU is the Valve hardware classification. Anything that is not an
// exact Deck mapping or a provisional Steam Machine candidate stays generic
// (readable non-Valve DMI) or unknown (unreadable DMI).
type HardwareSKU string

const (
	SKUGeneric               HardwareSKU = "generic"
	SKUDeckLCD               HardwareSKU = "deck-lcd"
	SKUDeckOLED              HardwareSKU = "deck-oled"
	SKUSteamMachineCandidate HardwareSKU = "steam-machine-candidate"
	SKUUnknown               HardwareSKU = "unknown"
)

// MatchKind records how a hardware SKU was reached.
type MatchKind string

const (
	MatchExact     MatchKind = "exact"
	MatchHeuristic MatchKind = "heuristic"
	MatchUnknown   MatchKind = "unknown"
)

// GPUProbeSource distinguishes active renderer evidence from fallback
// inventory. A vendor-only DRM fallback never satisfies rules that require
// actual renderer evidence.
type GPUProbeSource string

const (
	ProbeRenderer GPUProbeSource = "renderer"
	ProbeLspci    GPUProbeSource = "lspci"
	ProbeDRM      GPUProbeSource = "drm"
	ProbeUnknown  GPUProbeSource = "unknown"
)

// NVIDIAKernel is the loaded kernel module flavor. The NVIDIA open kernel
// module is distinct from nouveau/NVK; a loaded module without a readable
// banner leaves the flavor unknown.
type NVIDIAKernel string

const (
	NVIDIAKernelProprietary NVIDIAKernel = "proprietary"
	NVIDIAKernelOpen        NVIDIAKernel = "open"
	NVIDIAKernelNouveau     NVIDIAKernel = "nouveau"
	NVIDIAKernelNotObserved NVIDIAKernel = "not-observed"
	NVIDIAKernelUnknown     NVIDIAKernel = "unknown"
)

// NVIDIAUserspace is the userspace driver family. Kernel and userspace are
// separate axes; open NVIDIA kernel modules use NVIDIA userspace.
type NVIDIAUserspace string

const (
	NVIDIAUserspaceNVIDIA  NVIDIAUserspace = "nvidia"
	NVIDIAUserspaceNVK     NVIDIAUserspace = "nvk"
	NVIDIAUserspaceNouveau NVIDIAUserspace = "nouveau"
	NVIDIAUserspaceUnknown NVIDIAUserspace = "unknown"
)

// DiscoveryState records whether an inventory scan saw everything.
type DiscoveryState string

const (
	DiscoveryComplete DiscoveryState = "complete"
	DiscoveryPartial  DiscoveryState = "partial"
	DiscoveryUnknown  DiscoveryState = "unknown"
)

// NVIDIACandidate records whether a Vulkan ICD inventory references NVIDIA.
// Presence is inventory only: it never claims working Vulkan or an active
// device association.
type NVIDIACandidate string

const (
	NVIDIAPresent     NVIDIACandidate = "present"
	NVIDIANotObserved NVIDIACandidate = "not-observed"
	NVIDIAUnknown     NVIDIACandidate = "unknown"
)

// SessionKind is the graphical session type.
type SessionKind string

const (
	SessionGamescope SessionKind = "gamescope"
	SessionWayland   SessionKind = "wayland"
	SessionX11       SessionKind = "x11"
	SessionHeadless  SessionKind = "headless"
	SessionUnknown   SessionKind = "unknown"
)

// SteamInstallKind is the Steam package kind. Several kinds may coexist.
type SteamInstallKind string

const (
	SteamNative  SteamInstallKind = "native"
	SteamFlatpak SteamInstallKind = "flatpak"
	SteamSnap    SteamInstallKind = "snap"
)

// DiagnosticOutcome is the stable outcome vocabulary for detection issues.
type DiagnosticOutcome string

const (
	OutcomeMissing     DiagnosticOutcome = "missing"
	OutcomePermission  DiagnosticOutcome = "permission"
	OutcomeMalformed   DiagnosticOutcome = "malformed"
	OutcomeConflict    DiagnosticOutcome = "conflict"
	OutcomeTimeout     DiagnosticOutcome = "timeout"
	OutcomeUnsupported DiagnosticOutcome = "unsupported"
)

// IdentityConfidence distinguishes credible pads from uncertain matches.
// Uncertain matches stay labeled candidates; keyboards and mice are excluded.
type IdentityConfidence string

const (
	ControllerPad       IdentityConfidence = "controller"
	ControllerCandidate IdentityConfidence = "candidate"
)

// OSInfo preserves distro identity separately from the package family.
type OSInfo struct {
	ID                string   `json:"id"`
	IDLike            []string `json:"idLike"`
	VariantID         string   `json:"variantId"`
	VersionID         string   `json:"versionId"`
	PrettyName        string   `json:"prettyName"`
	Family            OSFamily `json:"family"`
	Immutable         TriState `json:"immutable"`
	ImmutableEvidence string   `json:"immutableEvidence"`
	SourcePath        string   `json:"sourcePath"`
}

// HardwareInfo holds sanitized DMI evidence and the SKU classification.
// The raw product string is retained so provisional candidates stay
// follow-up-able.
type HardwareInfo struct {
	SysVendor   string      `json:"sysVendor"`
	ProductName string      `json:"productName"`
	SKU         HardwareSKU `json:"sku"`
	Match       MatchKind   `json:"match"`
	Evidence    string      `json:"evidence"`
}

// GPUProbeInfo records how the GPU evidence was acquired. Renderer carries
// the active renderer string; lspci carries the selected PCI identity when
// actually known; drm is vendor-only inventory with unknown association.
type GPUProbeInfo struct {
	Source            GPUProbeSource `json:"source"`
	Renderer          string         `json:"renderer"`
	PCIVendor         string         `json:"pciVendor"`
	PCIDevice         string         `json:"pciDevice"`
	DeviceAssociation string         `json:"deviceAssociation"`
	Provenance        string         `json:"provenance"`
}

// NVIDIAState keeps loaded kernel flavor, userspace family and device
// association on separate axes, plus the numeric driver version, tri-state
// modeset and the inventory-only Vulkan ICD scan.
type NVIDIAState struct {
	Kernel      NVIDIAKernel    `json:"kernel"`
	Userspace   NVIDIAUserspace `json:"userspace"`
	Association TriState        `json:"association"`
	RawVersion  string          `json:"rawVersion"`
	Version     string          `json:"version"`
	VersionSrc  string          `json:"versionSource"`
	Modeset     TriState        `json:"modeset"`
	Vulkan      VulkanInfo      `json:"vulkan"`
}

// VulkanManifest is one parsed ICD manifest: inventory only, never proof of
// a working runtime, selected device, library availability or 32-bit
// coverage.
type VulkanManifest struct {
	Path        string `json:"path"`
	LibraryPath string `json:"libraryPath"`
	APIVersion  string `json:"apiVersion"`
	VendorHint  string `json:"vendorHint"`
	ParseState  string `json:"parseState"`
}

// VulkanInfo is the inventory-only ICD scan result.
type VulkanInfo struct {
	Manifests       []VulkanManifest `json:"manifests"`
	ScanState       DiscoveryState   `json:"scanState"`
	NVIDIACandidate NVIDIACandidate  `json:"nvidiaCandidate"`
	Evidence        string           `json:"evidence"`
}

// SessionInfo is the typed session classification. Gamescope hardware
// signals never identify a hardware SKU, and neither Deck DMI nor SteamOS
// alone proves Game Mode.
type SessionInfo struct {
	Kind     SessionKind `json:"kind"`
	Desktop  string      `json:"desktop"`
	GameMode TriState    `json:"gameMode"`
	Evidence string      `json:"evidence"`
}

// SteamInstall is one detected Steam root. With no proven active install
// there is no Selected field to guess.
type SteamInstall struct {
	Kind     SteamInstallKind `json:"kind"`
	Root     string           `json:"root"`
	Evidence string           `json:"evidence"`
}

// SteamInfo reports every detected install kind. Simultaneous native,
// Flatpak and Snap installations are all represented.
type SteamInfo struct {
	Installs  []SteamInstall `json:"installs"`
	Discovery DiscoveryState `json:"discovery"`
	Running   TriState       `json:"running"`
}

// ControllerDevice is one observed input device. Serial numbers, Bluetooth
// addresses and unique physical paths are never stored.
type ControllerDevice struct {
	Name               string             `json:"name"`
	Bus                string             `json:"bus"`
	VendorID           string             `json:"vendorId"`
	ProductID          string             `json:"productId"`
	Handlers           []string           `json:"handlers"`
	IdentityConfidence IdentityConfidence `json:"confidence"`
}

// ControllerInfo is the informational controller inventory. Presence never
// changes the GPU profile and proves nothing about Steam Input routing.
type ControllerInfo struct {
	Devices   []ControllerDevice `json:"devices"`
	Discovery DiscoveryState     `json:"discovery"`
}

// DetectionDiagnostic is one stable, deterministically ordered observation
// about missing, unreadable, malformed or conflicting evidence. Details are
// sanitized and never contain machine-specific absolute home paths.
type DetectionDiagnostic struct {
	Code      string            `json:"code"`
	Subsystem string            `json:"subsystem"`
	Source    string            `json:"source"`
	Outcome   DiagnosticOutcome `json:"outcome"`
	Detail    string            `json:"detail"`
}

// Platform is the read-only-by-convention detection snapshot. GPU reuses the
// accepted step-1 type with unchanged capability meaning.
type Platform struct {
	OS          OSInfo                `json:"os"`
	Hardware    HardwareInfo          `json:"hardware"`
	GPU         GPUCapabilities       `json:"gpu"`
	GPUProbe    GPUProbeInfo          `json:"gpuProbe"`
	NVIDIA      NVIDIAState           `json:"nvidia"`
	Session     SessionInfo           `json:"session"`
	Steam       SteamInfo             `json:"steam"`
	Controllers ControllerInfo        `json:"controllers"`
	Diagnostics []DetectionDiagnostic `json:"diagnostics"`
}

// Profile is the deterministic derivative of a Platform snapshot. Driver
// state and Steam/controllers are orthogonal facts, not presets, so they do
// not appear here.
type Profile struct {
	Version       int         `json:"version"`
	OSID          string      `json:"osId"`
	OSVariant     string      `json:"osVariant"`
	OSFamily      OSFamily    `json:"osFamily"`
	SKU           HardwareSKU `json:"sku"`
	GPUVendor     GPUVendor   `json:"gpuVendor"`
	GPUGeneration string      `json:"gpuGeneration"`
	GPUAmbiguous  bool        `json:"gpuAmbiguous"`
	Session       SessionKind `json:"session"`
	GameMode      TriState    `json:"gameMode"`
	HardwareMatch MatchKind   `json:"hardwareMatch"`
}

// ProfileVersion is the identity contract version.
const ProfileVersion = 1

// DetectionFS is the injected read-only filesystem. Paths are logical
// absolute Linux paths; the production adapter wraps OS reads while the
// fixture adapter translates them into fixture-relative entries. The fixture
// adapter must never follow a fixture symlink onto the real host.
type DetectionFS interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Stat(string) (fs.FileInfo, error)
	Readlink(string) (string, error)
}

// GPUProbeFunc adapts the accepted step-1 detector (and its
// renderer/lspci/DRM fallback) with provenance. Fixture callbacks feed
// captured outputs through the real step-1 classifiers and must not return
// a hard-coded expected generation.
type GPUProbeFunc func(context.Context) (GPUCapabilities, GPUProbeInfo, error)

// DetectionSources carries every host dependency DetectPlatformWith may
// touch. A nil Files/Env/GPU dependency is a programmer error, never
// permission to reach the host. EUID carries the process owner for the
// best-effort same-user Steam process check; HaveEUID must be set when EUID
// is meaningful (the production wrapper fills it from os.Geteuid).
type DetectionSources struct {
	Files    DetectionFS
	Env      func(string) string
	Home     string
	GPU      GPUProbeFunc
	EUID     int
	HaveEUID bool
}

// Read bounds from the accepted design. The production adapter enforces byte
// limits before unbounded allocation; scans are capped and marked partial on
// truncation; listings are sorted before classification.
const (
	ReadLimitText      = 1 << 20
	ReadLimitOSRelease = 64 << 10
	ReadLimitSmall     = 8 << 10
	MaxScanEntries     = 4096
	MaxRawFieldLen     = 256
	MaxRendererLen     = 1024
	MaxDetailLen       = 512
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// SanitizeField caps an untrusted display string, strips terminal escapes
// and control characters, and trims surrounding whitespace. Raw strings are
// never interpolated into shell commands.
func SanitizeField(s string, maxLen int) string {
	if maxLen <= 0 || maxLen > 65536 {
		maxLen = MaxRawFieldLen
	}
	s = ansiEscape.ReplaceAllString(s, "")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\x00' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			if r == '\t' {
				b.WriteRune(' ')
			}
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > maxLen {
		out = strings.TrimSpace(out[:maxLen])
	}
	return out
}

// SortDiagnostics orders diagnostics deterministically.
func SortDiagnostics(ds []DetectionDiagnostic) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].Subsystem != ds[j].Subsystem {
			return ds[i].Subsystem < ds[j].Subsystem
		}
		if ds[i].Source != ds[j].Source {
			return ds[i].Source < ds[j].Source
		}
		if ds[i].Code != ds[j].Code {
			return ds[i].Code < ds[j].Code
		}
		if ds[i].Outcome != ds[j].Outcome {
			return ds[i].Outcome < ds[j].Outcome
		}
		return ds[i].Detail < ds[j].Detail
	})
}

// ProfileFor derives one profile deterministically from a snapshot. It is
// pure: no environment, filesystem, probes or runtime settings. Unknown GPU
// and ambiguity never unlock extra capabilities.
func ProfileFor(p Platform) Profile {
	vendor := p.GPU.Vendor
	if vendor == "" {
		vendor = GPUUnknown
	}
	gen := p.GPU.Generation
	if strings.TrimSpace(gen) == "" {
		gen = "unknown"
	}
	return Profile{
		Version:       ProfileVersion,
		OSID:          p.OS.ID,
		OSVariant:     p.OS.VariantID,
		OSFamily:      p.OS.Family.Normalize(),
		SKU:           p.Hardware.SKU,
		GPUVendor:     vendor,
		GPUGeneration: gen,
		GPUAmbiguous:  p.GPU.Ambiguous,
		Session:       p.Session.Kind,
		GameMode:      p.Session.GameMode.Normalize(),
		HardwareMatch: p.Hardware.Match,
	}
}

// Key returns the stable serialization of the profile identity. It includes
// the contract version and canonical fields, never renderer text, user
// paths, controller lists, driver versions or translated display labels.
func (p Profile) Key() string {
	amb := "stable"
	if p.GPUAmbiguous {
		amb = "ambiguous"
	}
	return fmt.Sprintf("v%d|os=%s|variant=%s|family=%s|sku=%s|gpu=%s|gen=%s|%s|session=%s|gamemode=%s|match=%s",
		p.Version, p.OSID, p.OSVariant, p.OSFamily.Normalize(), p.SKU,
		p.GPUVendor, p.GPUGeneration, amb, p.Session, p.GameMode.Normalize(), p.HardwareMatch)
}
