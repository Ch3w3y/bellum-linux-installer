package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// GPUVendor is the recognized GPU manufacturer. Unknown is deliberately the
// zero value so failed or incomplete identification grants no capabilities.
type GPUVendor string

const (
	GPUUnknown GPUVendor = "Unknown"
	GPUNVIDIA  GPUVendor = "NVIDIA"
	GPUAMD     GPUVendor = "AMD"
	GPUIntel   GPUVendor = "Intel"
)

// GPUCapabilities describes features the installer can safely configure.
// Detection is based on the active OpenGL renderer; hybrid systems may expose
// only one adapter; Ambiguous is set only when the renderer explicitly includes
// a PRIME/hybrid/mux marker. Absence of this flag does not prove there is one GPU.
type GPUCapabilities struct {
	Vendor               GPUVendor
	Renderer             string
	Generation           string
	Ambiguous            bool
	DLSS                 bool
	NVAPI                bool
	FSR                  bool
	FSR4                 bool
	FrameGeneration      bool
	MultiFrameGeneration bool
	MLFrameGeneration    bool
}

// DetectGPUCapabilitiesWith makes renderer acquisition injectable for callers
// and tests. Empty/unrecognized renderers remain unknown with no enabled flags.
func DetectGPUCapabilitiesWith(readRenderer func() (string, error)) (GPUCapabilities, error) {
	renderer, err := readRenderer()
	if err != nil {
		return GPUCapabilities{}, err
	}
	return ClassifyGPUCapabilities(renderer), nil
}

// glxinfoMissingError reports whether the renderer probe failed because the
// glxinfo binary itself is absent (exec.LookPath "executable file not found").
func glxinfoMissingError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*exec.Error); ok {
		return true
	}
	return strings.Contains(err.Error(), "executable file not found in $PATH")
}

// readRendererWithFallback is the production renderer acquisition chain:
// glxinfo first (rich renderer strings), then lspci, then DRM sysfs vendor
// IDs. It only errors when every probe fails, so prechecks degrade
// gracefully on hosts without glxinfo instead of hard-failing.
func readRendererWithFallback() (string, error) {
	return readRendererWithFallbackWith(
		func() (string, error) {
			output, err := RunCommandWithOutput([]string{"glxinfo", "-B"})
			if err != nil {
				return "", err
			}
			return parseRenderer(output), nil
		},
		readRendererFromLspci,
		detectGPUCapabilitiesFromDRM,
	)
}

// readRendererWithFallbackWith makes each probe injectable for tests.
// glxinfo probe errors that are not "binary missing" abort the chain.
func readRendererWithFallbackWith(
	runGlxinfo func() (string, error),
	runLspci func() (string, bool),
	runDRM func() (GPUCapabilities, error),
) (string, error) {
	renderer, err := runGlxinfo()
	if err == nil {
		if strings.TrimSpace(renderer) == "" {
			return "", fmt.Errorf("glxinfo output did not contain an OpenGL renderer")
		}
		return renderer, nil
	}
	if !glxinfoMissingError(err) {
		return "", err
	}
	if lspciRenderer, ok := runLspci(); ok {
		return lspciRenderer, nil
	}
	if caps, drmErr := runDRM(); drmErr == nil {
		return caps.Renderer, nil
	}
	// Surface the original, most actionable lookup failure.
	return "", err
}

func DetectGPUCapabilities() (GPUCapabilities, error) {
	return DetectGPUCapabilitiesWith(readRendererWithFallback)
}

// detectGPUCapabilitiesFromDRM identifies the GPU vendor directly from sysfs
// DRM card entries when no GL renderer source is available. It matches the
// known PCI vendor IDs of NVIDIA, AMD/ATI, and Intel adapters.
func detectGPUCapabilitiesFromDRM() (GPUCapabilities, error) {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return GPUCapabilities{}, err
	}
	for _, e := range entries {
		name := e.Name()
		// Top-level cardN nodes only; cardN-DP-1 style connectors are skipped.
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		vendorID, readErr := os.ReadFile(filepath.Join("/sys/class/drm", name, "device", "vendor"))
		if readErr != nil {
			continue
		}
		vid := strings.TrimSpace(string(vendorID))
		vendor := gpuVendorFromPCIID(vid)
		if vendor == GPUUnknown {
			continue
		}
		return GPUCapabilities{Vendor: vendor, Renderer: fmt.Sprintf("DRM device %s (%s, vendor %s, no GL renderer)", name, vendor, vid)}, nil
	}
	return GPUCapabilities{}, fmt.Errorf("no supported DRM GPU device found")
}

// readRendererFromLspci runs lspci when available and maps the first
// VGA/Display/3D device to a renderer string. ok=false when lspci is absent,
// fails, or no GPU-class device exists.
func readRendererFromLspci() (string, bool) {
	if LookPath("lspci") == "" {
		return "", false
	}
	output, err := RunCommandWithOutput([]string{"lspci", "-nn"})
	if err != nil {
		return "", false
	}
	return readRendererFromLspciOutput(output)
}

func readRendererFromLspciOutput(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		if renderer, ok := parseLspciRendererLine(line); ok {
			return renderer, true
		}
	}
	return "", false
}

// parseLspciRendererLine inspects one `lspci -nn` line. It uses the PCI class
// and vendor IDs, not localized vendor or class names.
func parseLspciRendererLine(line string) (string, bool) {
	classCode := lspciClassCode(line)
	if classCode == "" || (classCode != "0300" && classCode != "0302" && classCode != "0380") {
		return "", false
	}
	for _, match := range pciVendorID.FindAllStringSubmatch(line, -1) {
		if vendor := gpuVendorFromPCIID(match[1]); vendor != GPUUnknown {
			return string(vendor) + " " + strings.TrimSpace(line), true
		}
	}
	return "", false
}

var pciVendorID = regexp.MustCompile(`\[([[:xdigit:]]{4}):[[:xdigit:]]{4}\]`)

func gpuVendorFromPCIID(id string) GPUVendor {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "0x") {
	case "10de":
		return GPUNVIDIA
	case "1002", "1022":
		return GPUAMD
	case "8086":
		return GPUIntel
	default:
		return GPUUnknown
	}
}

// lspciClassCode extracts the bracketed hex class code from an lspci -nn
// line, e.g. `[0300]` -> `0300`. Returns "" when absent or malformed.
func lspciClassCode(line string) string {
	open := strings.Index(line, "[0")
	if open < 0 {
		return ""
	}
	rest := line[open+1:]
	end := strings.Index(rest, "]")
	if end != 4 {
		return ""
	}
	code := rest[:end]
	for _, ch := range code {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return ""
		}
	}
	return code
}

// DetectGPU preserves the historical vendor-string API for cleanup code.
func DetectGPU() (string, error) {
	caps, err := DetectGPUCapabilities()
	if err != nil {
		return "", err
	}
	if caps.Vendor == GPUUnknown {
		return "", nil
	}
	return string(caps.Vendor), nil
}

func parseRenderer(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), "opengl renderer") {
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// ClassifyGPUCapabilities uses explicit renderer identifiers only. Generation
// and optional features are enabled only where the renderer provides evidence.
func ClassifyGPUCapabilities(renderer string) GPUCapabilities {
	r := strings.ToLower(renderer)
	c := GPUCapabilities{Vendor: GPUUnknown, Renderer: renderer}
	if strings.Contains(r, "nvidia") || strings.Contains(r, "geforce") || strings.Contains(r, "quadro") {
		c.Vendor = GPUNVIDIA
		// RTX names encode architecture; GTX 16xx is Turing, GTX 10xx is Pascal.
		if modelMatch(r, `\brtx\s*50\d{2}\b`) {
			c.Generation = "Blackwell"
			c.DLSS, c.NVAPI, c.FrameGeneration, c.MultiFrameGeneration = true, true, true, true
		} else if modelMatch(r, `\brtx\s*40\d{2}\b`) {
			c.Generation = "Ada"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		} else if modelMatch(r, `\brtx\s*30\d{2}\b`) {
			c.Generation = "Ampere"
			c.DLSS, c.NVAPI = true, true
		} else if modelMatch(r, `\brtx\s*20\d{2}\b`) {
			c.Generation = "Turing"
			c.DLSS, c.NVAPI = true, true
		} else if modelMatch(r, `\bgtx\s*16\d{2}\b`) {
			c.Generation = "Turing"
			c.NVAPI = true
		}
		c.FSR = true
	} else if strings.Contains(r, "amd") || strings.Contains(r, "radeon") || strings.Contains(r, "advanced micro devices") {
		c.Vendor = GPUAMD
		if modelMatch(r, `\bgfx12\d{2}\b`) || modelMatch(r, `\brx\s*9\d{3}\b`) {
			c.Generation = "RDNA4"
			c.FSR, c.FSR4, c.FrameGeneration, c.MLFrameGeneration = true, true, true, true
		} else if modelMatch(r, `\bgfx11\d{2}\b`) || modelMatch(r, `\brx\s*7\d{3}\b`) {
			c.Generation = "RDNA3"
			c.FSR, c.FSR4, c.FrameGeneration, c.MLFrameGeneration = true, true, true, true
		} else if modelMatch(r, `\bgfx10\d{2}\b`) {
			if modelMatch(r, `\bgfx10[0-2]\d\b`) {
				c.Generation = "RDNA1"
			} else if modelMatch(r, `\bgfx103\d\b`) {
				c.Generation = "RDNA2"
				c.FSR4 = true
			}
			c.FSR = true
		} else if modelMatch(r, `\brx\s*6\d{3}\b`) {
			c.Generation = "RDNA2"
			c.FSR, c.FSR4 = true, true
		} else if modelMatch(r, `\brx\s*5\d{3}\b`) {
			c.Generation = "RDNA1"
			c.FSR = true
		} else {
			c.FSR = true
		}
	} else if strings.Contains(r, "intel") || strings.Contains(r, "arc ") {
		c.Vendor = GPUIntel
		c.FSR = true
	}
	// A renderer string cannot reliably identify a second, inactive adapter.
	// Report explicit hybrid/PRIME markers as ambiguity and avoid architecture claims;
	// ordinary glxinfo output cannot reveal an inactive adapter.
	if strings.Contains(r, "prime") || strings.Contains(r, "hybrid") || strings.Contains(r, "mux") {
		c.Ambiguous = true
		c.Generation = ""
		c.DLSS, c.NVAPI, c.FrameGeneration, c.MultiFrameGeneration, c.MLFrameGeneration = false, false, false, false, false
	}
	return c
}

func modelMatch(value, pattern string) bool {
	return regexp.MustCompile(pattern).MatchString(value)
}

func classifyGPU(renderer string) string {
	caps := ClassifyGPUCapabilities(renderer)
	if caps.Vendor == GPUUnknown {
		parts := strings.Fields(renderer)
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	}
	return string(caps.Vendor)
}
