package core

import (
	"context"
	"regexp"
	"strings"
)

// NVIDIA loaded-state detection. Kernel flavor and userspace are separate
// axes: the loaded proc banner distinguishes NVIDIA proprietary from the
// NVIDIA Open Kernel Module (version alone does not), and open NVIDIA kernel
// modules use NVIDIA userspace and must never be classified as nouveau/NVK.
// A manifest or module observed somewhere on a hybrid system never proves it
// drives the selected renderer.

// NVIDIA evidence paths (logical absolute Linux paths).
const (
	nvidiaVersionPath = "/sys/module/nvidia/version"
	nvidiaBannerPath  = "/proc/driver/nvidia/version"
	nouveauModulePath = "/sys/module/nouveau"
	nvidiaModulePath  = "/sys/module/nvidia"
	nvidiaModesetPath = "/sys/module/nvidia_drm/parameters/modeset"
)

var driverVersionRe = regexp.MustCompile(`\d+(?:\.\d+)*`)

// ParseDriverVersion extracts the first numeric dotted version without
// lexicographic comparison. Unparseable input keeps ok=false while the raw
// text is retained by the caller.
func ParseDriverVersion(raw string) (string, bool) {
	m := driverVersionRe.FindString(raw)
	if m == "" {
		return "", false
	}
	return m, true
}

// DetectNVIDIA reads loaded module evidence, the numeric driver version, the
// tri-state modeset flag and the inventory-only Vulkan ICD scan. renderer is
// the active renderer string used to judge device association; icdLibs are
// library paths from the ICD scan used for userspace corroboration.
func DetectNVIDIA(ctx context.Context, fs DetectionFS, renderer string, icdLibs []string) (NVIDIAState, []DetectionDiagnostic) {
	st := NVIDIAState{
		Kernel:      NVIDIAKernelNotObserved,
		Userspace:   NVIDIAUserspaceUnknown,
		Association: TriUnknown,
		Modeset:     TriUnknown,
	}
	var diags []DetectionDiagnostic
	add := func(code, source string, outcome DiagnosticOutcome, detail string) {
		diags = append(diags, DetectionDiagnostic{Code: code, Subsystem: "nvidia", Source: source, Outcome: outcome, Detail: SanitizeField(detail, MaxDetailLen)})
	}

	if err := ctx.Err(); err != nil {
		add("nvidia/cancelled", "nvidia", OutcomeTimeout, "detection cancelled")
		return st, diags
	}

	nvidiaLoaded := pathExists(fs, nvidiaModulePath)
	nouveauLoaded := pathExists(fs, nouveauModulePath)

	versionRaw, versionSrc := "", ""
	if raw, err := readBounded(fs, nvidiaVersionPath, ReadLimitSmall); err == nil {
		versionRaw, versionSrc = strings.TrimSpace(string(raw)), nvidiaVersionPath
	} else if !isMissing(err) {
		add("nvidia/version-unreadable", nvidiaVersionPath, outcomeOf(err), "NVIDIA module version unreadable")
	}
	banner, bannerErr := readBounded(fs, nvidiaBannerPath, ReadLimitSmall)
	bannerText := ""
	if bannerErr == nil {
		bannerText = string(banner)
		if versionRaw == "" {
			if v, ok := ParseDriverVersion(bannerText); ok {
				versionRaw, versionSrc = v, nvidiaBannerPath
			} else {
				versionRaw, versionSrc = SanitizeField(bannerText, 128), nvidiaBannerPath
			}
		}
	} else if !isMissing(bannerErr) {
		add("nvidia/banner-unreadable", nvidiaBannerPath, outcomeOf(bannerErr), "NVIDIA proc banner unreadable")
	}

	// Kernel flavor comes from the loaded banner, never from the version
	// alone. A loaded nvidia module without a banner leaves flavor unknown.
	lowerBanner := strings.ToLower(bannerText)
	switch {
	case !nvidiaLoaded && !nouveauLoaded:
		st.Kernel = NVIDIAKernelNotObserved
	case nvidiaLoaded && nouveauLoaded:
		st.Kernel = NVIDIAKernelUnknown
		add("nvidia/modules-conflict", "sys:module", OutcomeConflict, "both nvidia and nouveau modules observed; loaded evidence is ambiguous")
	case nvidiaLoaded && bannerErr == nil:
		switch {
		case strings.Contains(lowerBanner, "open kernel module"):
			st.Kernel = NVIDIAKernelOpen
		case strings.Contains(lowerBanner, "kernel module"):
			st.Kernel = NVIDIAKernelProprietary
		default:
			st.Kernel = NVIDIAKernelUnknown
			add("nvidia/flavor-unknown", nvidiaBannerPath, OutcomeMalformed, "loaded nvidia module without a recognized banner; flavor unknown")
		}
	case nvidiaLoaded:
		st.Kernel = NVIDIAKernelUnknown
		add("nvidia/flavor-unknown", nvidiaBannerPath, OutcomeMissing, "loaded nvidia module without a readable banner; flavor unknown")
	case nouveauLoaded:
		st.Kernel = NVIDIAKernelNouveau
	}

	st.RawVersion = SanitizeField(versionRaw, 128)
	st.VersionSrc = versionSrc
	if versionRaw != "" {
		if v, ok := ParseDriverVersion(versionRaw); ok {
			st.Version = v
		} else {
			add("nvidia/version-unparseable", versionSrc, OutcomeMalformed, "retaining unparseable driver version with unknown parsed value")
		}
	}

	// Modeset: Y/1 means yes, N/0 means no; anything else (or a read
	// failure) is unknown.
	if raw, err := readBounded(fs, nvidiaModesetPath, ReadLimitSmall); err == nil {
		switch strings.ToLower(strings.TrimSpace(string(raw))) {
		case "y", "1":
			st.Modeset = TriYes
		case "n", "0":
			st.Modeset = TriNo
		default:
			st.Modeset = TriUnknown
			add("nvidia/modeset-unrecognized", nvidiaModesetPath, OutcomeMalformed, "unrecognized modeset value; unknown")
		}
	} else if !isMissing(err) {
		add("nvidia/modeset-unreadable", nvidiaModesetPath, outcomeOf(err), "modeset flag unreadable; unknown")
	}

	st.Userspace = classifyNVIDIAUserspace(renderer, icdLibs, st.Kernel)
	st.Association = classifyNVIDIAAssociation(renderer, st.Kernel)

	return st, diags
}

// classifyNVIDIAUserspace treats NVK/nouveau markers independently from GPU
// architecture. Open NVIDIA kernel modules resolve to NVIDIA userspace.
func classifyNVIDIAUserspace(renderer string, icdLibs []string, kernel NVIDIAKernel) NVIDIAUserspace {
	lower := strings.ToLower(renderer)
	if kernel == NVIDIAKernelOpen || kernel == NVIDIAKernelProprietary {
		if strings.Contains(lower, "nouveau") || containsLib(icdLibs, "nouveau") {
			return NVIDIAUserspaceUnknown
		}
		return NVIDIAUserspaceNVIDIA
	}
	switch {
	case strings.Contains(lower, "nouveau") || containsLib(icdLibs, "nouveau"):
		return NVIDIAUserspaceNouveau
	case strings.Contains(lower, "nvk") || containsLib(icdLibs, "nvk"):
		return NVIDIAUserspaceNVK
	case strings.Contains(lower, "nvidia") || strings.Contains(lower, "geforce") || strings.Contains(lower, "quadro") || containsLib(icdLibs, "nvidia"):
		return NVIDIAUserspaceNVIDIA
	default:
		return NVIDIAUserspaceUnknown
	}
}

func containsLib(libs []string, token string) bool {
	for _, lib := range libs {
		if strings.Contains(strings.ToLower(lib), token) {
			return true
		}
	}
	return false
}

// classifyNVIDIAAssociation keeps multi-GPU uncertainty: a module loaded
// somewhere else on a hybrid system does not disable the active adapter.
func classifyNVIDIAAssociation(renderer string, kernel NVIDIAKernel) TriState {
	lower := strings.ToLower(renderer)
	rendererIsNVIDIA := strings.Contains(lower, "nvidia") || strings.Contains(lower, "geforce") || strings.Contains(lower, "quadro")
	nvidiaPresent := kernel == NVIDIAKernelProprietary || kernel == NVIDIAKernelOpen
	switch {
	case rendererIsNVIDIA && nvidiaPresent:
		return TriYes
	case rendererIsNVIDIA:
		return TriUnknown
	case nvidiaPresent:
		return TriNo
	default:
		return TriUnknown
	}
}
