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
	Vendor          GPUVendor
	Renderer        string
	Generation      string
	Ambiguous       bool
	DLSS            bool
	NVAPI           bool
	FSR             bool
	FSR41           bool
	FrameGeneration bool
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

// readRendererWithFallbackWith makes each probe injectable for tests. Any
// glxinfo failure (missing binary, no display over SSH or on a TTY) falls
// through to lspci and then DRM sysfs, as does a successful run that printed
// no renderer at all.
func readRendererWithFallbackWith(
	runGlxinfo func() (string, error),
	runLspci func() (string, bool),
	runDRM func() (GPUCapabilities, error),
) (string, error) {
	renderer, err := runGlxinfo()
	if err == nil && strings.TrimSpace(renderer) != "" {
		return renderer, nil
	}
	if err == nil {
		// glxinfo ran but printed no "OpenGL renderer string" line. Classifying
		// "" would report a confident Unknown GPU; treat it as a failed probe
		// so lspci and DRM sysfs still get a chance.
		err = fmt.Errorf("glxinfo output did not contain an OpenGL renderer")
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

// DetectGPUCapabilities never fails: when no probe can identify the GPU (VMs,
// containers, unusual hardware) it reports GPUUnknown, which gets generic
// Proton settings.
func DetectGPUCapabilities() (GPUCapabilities, error) {
	caps, err := DetectGPUCapabilitiesWith(readRendererWithFallback)
	if err != nil {
		return GPUCapabilities{Vendor: GPUUnknown, Renderer: "undetected: " + err.Error()}, nil
	}
	return caps, nil
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
		// The vendor name goes into the renderer string so the string alone
		// still classifies: callers pass it back through
		// ClassifyGPUCapabilities, which only reads renderer text.
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
	// -nn keeps the machine-readable [vendor:device] IDs next to the names.
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

// parseLspciRendererLine inspects one `lspci -nn` line. Both the class and the
// vendor come from the bracketed hex IDs rather than the names, so the check
// survives localized class labels like "Affichage" and vendor spellings like
// "ATI Technologies" or "Advanced Micro Devices, Inc. [AMD/ATI]".
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

// pciVendorID matches the `[vendor:device]` pair that `lspci -nn` appends to
// each device name. The class code is `[0300]` and has no colon, so it cannot
// be mistaken for a vendor pair.
var pciVendorID = regexp.MustCompile(`\[([[:xdigit:]]{4}):[[:xdigit:]]{4}\]`)

// gpuVendorFromPCIID maps a PCI vendor ID, with or without the sysfs `0x`
// prefix, to a known GPU vendor. Anything else stays Unknown.
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

// Renderer model patterns. Matching the whole model number stops a prefix from
// claiming the wrong generation: "gfx10" covered every RDNA1 and RDNA2 chip
// alike, and the patterns also accept an unspaced "RTX5080".
var (
	// The workstation "RTX 4000/5000/6000 Ada Generation" parts reuse GeForce
	// RTX 40/50 numbering, so "RTX 5000 Ada Generation" would otherwise read as
	// Blackwell. The "Ada Generation" suffix settles it.
	rendererAdaWorkstation = regexp.MustCompile(`\bada generation\b`)
	// The Turing-era "Quadro RTX 4000/5000/6000/8000" reuse the numbers too.
	rendererQuadroRTX = regexp.MustCompile(`\bquadro\s+rtx\s*\d{4}\b`)

	rendererRTX50 = regexp.MustCompile(`\brtx\s*50\d{2}\b`)
	rendererRTX40 = regexp.MustCompile(`\brtx\s*40\d{2}\b`)
	rendererRTX30 = regexp.MustCompile(`\brtx\s*30\d{2}\b`)
	rendererRTX20 = regexp.MustCompile(`\brtx\s*20\d{2}\b`)
	rendererGTX16 = regexp.MustCompile(`\bgtx\s*16\d{2}\b`)

	// Mesa's gfx chip IDs, optionally with a revision suffix such as the
	// Phoenix APUs' "gfx1103_r1".
	rendererGFX12   = regexp.MustCompile(`\bgfx12\d{2}(?:_r\d+)?\b`)
	rendererGFX11   = regexp.MustCompile(`\bgfx11\d{2}(?:_r\d+)?\b`)
	rendererGFX103x = regexp.MustCompile(`\bgfx103[[:xdigit:]](?:_r\d+)?\b`)
	rendererGFX101x = regexp.MustCompile(`\bgfx101[[:xdigit:]](?:_r\d+)?\b`)
	rendererGFX9    = regexp.MustCompile(`\bgfx9[[:xdigit:]]{2}(?:_r\d+)?\b`)
	// Marketing names, including laptop parts such as "RX 7600S" and
	// "RX 6800M".
	rendererRX9000 = regexp.MustCompile(`\brx\s*9\d{3}[ms]?\b`)
	rendererRX7000 = regexp.MustCompile(`\brx\s*7\d{3}[ms]?\b`)
	rendererRX6000 = regexp.MustCompile(`\brx\s*6\d{3}[ms]?\b`)
	rendererRX5000 = regexp.MustCompile(`\brx\s*5\d{3}[ms]?\b`)
	// Mesa before 24.1 reported chip codenames instead of gfx IDs
	// ("radeonsi, navi21"); Ubuntu LTS and Debian stable still ship such
	// versions. Only RDNA codenames are mapped.
	rendererNavi4x = regexp.MustCompile(`\bnavi4\d\b`)
	rendererNavi3x = regexp.MustCompile(`\b(?:navi3\d|phoenix\d?|hawk_?point\d?|strix(?:_halo)?)\b`)
	rendererNavi2x = regexp.MustCompile(`\b(?:navi2\d|vangogh|rembrandt|raphael|mendocino|beige_goby|dimgrey_cavefish|navy_flounder|sienna_cichlid)\b`)
	rendererNavi1x = regexp.MustCompile(`\bnavi1\d\b`)
)

// ClassifyGPUCapabilities uses explicit renderer identifiers only. Generation
// and optional features are enabled only where the renderer provides evidence.
func ClassifyGPUCapabilities(renderer string) GPUCapabilities {
	r := strings.ToLower(renderer)
	c := GPUCapabilities{Vendor: GPUUnknown, Renderer: renderer}
	if strings.Contains(r, "nvidia") || strings.Contains(r, "geforce") || strings.Contains(r, "quadro") {
		c.Vendor = GPUNVIDIA
		// RTX names encode architecture. GTX 16xx is Turing too, but without
		// the tensor cores DLSS needs, so it gets NVAPI only.
		switch {
		case rendererQuadroRTX.MatchString(r):
			c.Generation = "Turing"
			c.DLSS, c.NVAPI = true, true
		case rendererAdaWorkstation.MatchString(r):
			c.Generation = "Ada"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		case rendererRTX50.MatchString(r):
			c.Generation = "Blackwell"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		case rendererRTX40.MatchString(r):
			c.Generation = "Ada"
			c.DLSS, c.NVAPI, c.FrameGeneration = true, true, true
		case rendererRTX30.MatchString(r):
			c.Generation = "Ampere"
			c.DLSS, c.NVAPI = true, true
		case rendererRTX20.MatchString(r):
			c.Generation = "Turing"
			c.DLSS, c.NVAPI = true, true
		case rendererGTX16.MatchString(r):
			c.Generation = "Turing"
			c.NVAPI = true
		}
		c.FSR = true
	} else if strings.Contains(r, "amd") || strings.Contains(r, "radeon") || strings.Contains(r, "advanced micro devices") {
		c.Vendor = GPUAMD
		// Mesa's gfx chip ID is the reliable signal; the marketing name is the
		// fallback. An AMD GPU whose generation matches nothing below keeps an
		// empty Generation and the conservative settings that go with it.
		switch {
		case rendererGFX12.MatchString(r) || rendererRX9000.MatchString(r) || rendererNavi4x.MatchString(r):
			c.Generation = "RDNA4"
			// FSR4's FP8 path is native on RDNA4 only (#21).
			c.FSR41, c.FrameGeneration = true, true
		case rendererGFX11.MatchString(r) || rendererRX7000.MatchString(r) || rendererNavi3x.MatchString(r):
			c.Generation = "RDNA3"
		case rendererGFX103x.MatchString(r) || rendererRX6000.MatchString(r) || rendererNavi2x.MatchString(r):
			// gfx103x is RDNA2, including the Steam Deck's gfx1033.
			c.Generation = "RDNA2"
		case rendererGFX101x.MatchString(r) || rendererRX5000.MatchString(r) || rendererNavi1x.MatchString(r):
			// gfx101x is RDNA1; the old gfx10 prefix match called it RDNA2.
			c.Generation = "RDNA1"
		case rendererGFX9.MatchString(r):
			// Vega and CDNA share the gfx9 family; neither is RDNA.
			c.Generation = "GCN/CDNA"
		}
		c.FSR = true
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
		c.DLSS, c.NVAPI, c.FSR41, c.FrameGeneration = false, false, false, false
	}
	return c
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
