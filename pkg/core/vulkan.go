package core

import (
	"context"
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// Inventory-only Vulkan ICD detection. A manifest proves inventory: never a
// successful load, a selected device, library availability, 32-bit coverage
// or working Vulkan. Step 4 may say "NVIDIA ICD not found in the searched
// locations"; it may not claim a Vulkan runtime failure from this scan.

// Vulkan loader override variables captured as evidence. Explicit overrides
// follow loader documentation order; filters and unsupported settings make
// effective selection unknown.
var vulkanOverrideVars = []string{
	"VK_DRIVER_FILES",
	"VK_ICD_FILENAMES",
	"VK_ADD_DRIVER_FILES",
	"VK_LOADER_DRIVERS_SELECT",
	"VK_LOADER_DRIVERS_DISABLE",
	"VK_LOADER_LAYERS_ENABLE",
	"VK_LOADER_LAYERS_DISABLE",
}

func vulkanSearchDirs(home string, env func(string) string) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		d = path.Clean("/" + strings.TrimPrefix(path.Clean(d), "/"))
		if d == "/" || seen[d] {
			return
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	configHome := env("XDG_CONFIG_HOME")
	if strings.TrimSpace(configHome) == "" {
		configHome = path.Join(home, ".config")
	}
	dataHome := env("XDG_DATA_HOME")
	if strings.TrimSpace(dataHome) == "" {
		dataHome = path.Join(home, ".local", "share")
	}
	add(path.Join(configHome, "vulkan", "icd.d"))
	add(path.Join(dataHome, "vulkan", "icd.d"))
	configDirs := env("XDG_CONFIG_DIRS")
	if strings.TrimSpace(configDirs) == "" {
		configDirs = "/etc/xdg"
	}
	for _, dir := range strings.Split(configDirs, ":") {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		add(path.Join(dir, "vulkan", "icd.d"))
	}
	add("/etc/vulkan/icd.d")
	add("/usr/local/share/vulkan/icd.d")
	add("/usr/share/vulkan/icd.d")
	return dirs
}

type icdJSON struct {
	ICD *struct {
		LibraryPath string `json:"library_path"`
		APIVersion  string `json:"api_version"`
	} `json:"ICD"`
	LibraryPath string `json:"library_path"`
	APIVersion  string `json:"api_version"`
}

// DetectVulkan scans the documented loader directories through the injected
// filesystem. JSON contents are parsed, not just filenames. Permission and
// parse failures make the result partial; loader overrides make effective
// selection unknown.
func DetectVulkan(ctx context.Context, fs DetectionFS, home string, env func(string) string) (VulkanInfo, []DetectionDiagnostic) {
	info := VulkanInfo{ScanState: DiscoveryComplete, NVIDIACandidate: NVIDIANotObserved}
	var diags []DetectionDiagnostic
	add := func(code, source string, outcome DiagnosticOutcome, detail string) {
		diags = append(diags, DetectionDiagnostic{Code: code, Subsystem: "vulkan", Source: source, Outcome: outcome, Detail: SanitizeField(detail, MaxDetailLen)})
	}

	var overrideEvidence []string
	for _, v := range vulkanOverrideVars {
		if val := strings.TrimSpace(env(v)); val != "" {
			overrideEvidence = append(overrideEvidence, v+"="+SanitizeField(val, 160))
		}
	}
	if len(overrideEvidence) > 0 {
		info.Evidence = "loader-overrides: " + strings.Join(overrideEvidence, " ")
	}

	entries := 0
	for _, dir := range vulkanSearchDirs(home, env) {
		if err := ctx.Err(); err != nil {
			add("vulkan/cancelled", dir, OutcomeTimeout, "scan cancelled")
			info.ScanState = DiscoveryPartial
			break
		}
		names, derr := readDirNames(fs, dir)
		if derr != nil {
			if isMissing(derr) {
				continue
			}
			info.ScanState = DiscoveryPartial
			add("vulkan/dir-unreadable", dir, outcomeOf(derr), "ICD directory unreadable")
			continue
		}
		for _, name := range names {
			if !strings.HasSuffix(strings.ToLower(name), ".json") {
				continue
			}
			full := path.Join(dir, name)
			entries++
			if entries > MaxScanEntries {
				info.ScanState = DiscoveryPartial
				add("vulkan/scan-truncated", dir, OutcomeUnsupported, "ICD scan hit the entry bound")
				break
			}
			raw, rerr := readBounded(fs, full, ReadLimitText)
			if rerr != nil {
				info.ScanState = DiscoveryPartial
				info.Manifests = append(info.Manifests, VulkanManifest{Path: full, ParseState: "unreadable"})
				add("vulkan/manifest-unreadable", full, outcomeOf(rerr), "ICD manifest unreadable")
				continue
			}
			manifest := VulkanManifest{Path: full, ParseState: "ok"}
			var parsed icdJSON
			if jerr := json.Unmarshal(raw, &parsed); jerr != nil {
				info.ScanState = DiscoveryPartial
				manifest.ParseState = "malformed"
				info.Manifests = append(info.Manifests, manifest)
				add("vulkan/manifest-malformed", full, OutcomeMalformed, "ICD manifest is not valid JSON")
				continue
			}
			lib, api := parsed.LibraryPath, parsed.APIVersion
			if parsed.ICD != nil {
				if lib == "" {
					lib = parsed.ICD.LibraryPath
				}
				if api == "" {
					api = parsed.ICD.APIVersion
				}
			}
			manifest.LibraryPath = SanitizeField(lib, 256)
			manifest.APIVersion = SanitizeField(api, 64)
			manifest.VendorHint = vulkanVendorHint(name, lib)
			info.Manifests = append(info.Manifests, manifest)
		}
	}

	sort.Slice(info.Manifests, func(i, j int) bool { return info.Manifests[i].Path < info.Manifests[j].Path })
	for _, m := range info.Manifests {
		if m.ParseState == "ok" && (m.VendorHint == "nvidia" || strings.Contains(strings.ToLower(m.LibraryPath), "nvidia")) {
			info.NVIDIACandidate = NVIDIAPresent
			break
		}
	}
	if info.NVIDIACandidate != NVIDIAPresent && info.ScanState != DiscoveryComplete {
		info.NVIDIACandidate = NVIDIAUnknown
	}
	if len(overrideEvidence) > 0 && info.NVIDIACandidate == NVIDIANotObserved {
		info.NVIDIACandidate = NVIDIAUnknown
	}
	return info, diags
}

func vulkanVendorHint(filename, lib string) string {
	joined := strings.ToLower(filename + " " + lib)
	switch {
	case strings.Contains(joined, "nvidia"):
		return "nvidia"
	case strings.Contains(joined, "intel"):
		return "intel"
	case strings.Contains(joined, "amd") || strings.Contains(joined, "radeon") || strings.Contains(joined, "radv"):
		return "amd"
	default:
		return ""
	}
}
