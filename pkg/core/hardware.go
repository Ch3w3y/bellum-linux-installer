package core

import (
	"regexp"
	"strings"
)

// Valve hardware classification. DMI values are trimmed of NUL/whitespace and
// compared case-insensitively; sys_vendor must read exactly Valve for any
// Valve match. Exact Deck mappings always outrank the Steam Machine
// heuristic, and DMI alone never sets the GPU generation.

var gfx11Token = regexp.MustCompile(`\bgfx11[0-9a-f]{2}\b`)

// SanitizeDMI trims NUL/whitespace and sanitizes an untrusted DMI string.
func SanitizeDMI(s string) string {
	s = strings.Trim(s, "\x00 \t\r\n")
	return SanitizeField(s, MaxRawFieldLen)
}

// HasCompleteGFX11Token reports whether renderer text carries a complete
// gfx11xx token (gfx1100-gfx11ff). Marketing names without the token and
// vendor-only fallbacks do not satisfy the Steam Machine rule.
func HasCompleteGFX11Token(renderer string) bool {
	return gfx11Token.MatchString(strings.ToLower(renderer))
}

// ClassifyHardware maps sanitized DMI evidence plus the step-1 GPU result to
// a hardware SKU. GPU generation always comes from the step-1 classifier;
// contradictory renderers stay visible with a diagnostic instead of being
// overwritten from the SKU.
func ClassifyHardware(sysVendor, productName string, caps GPUCapabilities, probe GPUProbeInfo) (HardwareInfo, []DetectionDiagnostic) {
	vendor := SanitizeDMI(sysVendor)
	product := SanitizeDMI(productName)
	lowerVendor := strings.ToLower(vendor)
	lowerProduct := strings.ToLower(product)
	var diags []DetectionDiagnostic

	if vendor == "" || product == "" {
		return HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUUnknown, Match: MatchUnknown, Evidence: "dmi:unreadable"}, append(diags, DetectionDiagnostic{
			Code:      "hardware/dmi-unreadable",
			Subsystem: "hardware",
			Source:    "dmi:sys_vendor,product_name",
			Outcome:   OutcomeMissing,
			Detail:    "DMI vendor or product unreadable; hardware unknown",
		})
	}

	if lowerVendor == "valve" {
		switch lowerProduct {
		case "jupiter":
			info := HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUDeckLCD, Match: MatchExact, Evidence: "dmi:sys_vendor,product_name"}
			return info, checkDeckRendererConflict(product, caps, probe)
		case "galileo":
			info := HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUDeckOLED, Match: MatchExact, Evidence: "dmi:sys_vendor,product_name"}
			return info, checkDeckRendererConflict(product, caps, probe)
		}
		// Otherwise-unmatched Valve hardware: the conservative provisional
		// Steam Machine rule needs unambiguous AMD RDNA3 plus a complete
		// gfx11xx token from actual renderer evidence.
		if IsUnambiguousRDNA3(caps, probe) {
			return HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUSteamMachineCandidate, Match: MatchHeuristic, Evidence: "dmi:sys_vendor,product_name+renderer:gfx11xx"}, diags
		}
		return HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUUnknown, Match: MatchUnknown, Evidence: "dmi:sys_vendor,product_name"}, append(diags, DetectionDiagnostic{
			Code:      "hardware/valve-unmatched",
			Subsystem: "hardware",
			Source:    "dmi:sys_vendor,product_name",
			Outcome:   OutcomeConflict,
			Detail:    "Valve DMI without an exact Deck mapping or unambiguous gfx11xx renderer evidence; hardware unknown, product retained for follow-up",
		})
	}

	return HardwareInfo{SysVendor: vendor, ProductName: product, SKU: SKUGeneric, Match: MatchUnknown, Evidence: "dmi:sys_vendor,product_name"}, diags
}

// checkDeckRendererConflict keeps a contradictory renderer visible. An exact
// Deck SKU is never overwritten from DMI and never upgrades the GPU result.
func checkDeckRendererConflict(product string, caps GPUCapabilities, probe GPUProbeInfo) []DetectionDiagnostic {
	if strings.TrimSpace(caps.Renderer) == "" || strings.HasPrefix(caps.Renderer, "undetected") {
		return nil
	}
	if caps.Ambiguous {
		return []DetectionDiagnostic{{
			Code:      "hardware/gpu-ambiguous",
			Subsystem: "hardware",
			Source:    "gpu:renderer",
			Outcome:   OutcomeConflict,
			Detail:    "Deck DMI with an explicitly hybrid renderer; GPU generation withheld",
		}}
	}
	if caps.Vendor == GPUAMD && (caps.Generation == "RDNA2" || caps.Generation == "") {
		return nil
	}
	if caps.Vendor == GPUAMD || caps.Vendor == GPUNVIDIA || caps.Vendor == GPUIntel {
		return []DetectionDiagnostic{{
			Code:      "hardware/gpu-conflict",
			Subsystem: "hardware",
			Source:    "gpu:renderer",
			Outcome:   OutcomeConflict,
			Detail:    SanitizeField("Deck DMI with a non-RDNA2 active renderer ("+string(caps.Vendor)+" "+caps.Generation+"); SKU mapping kept, GPU result unchanged", MaxDetailLen),
		}}
	}
	return nil
}

// IsUnambiguousRDNA3 reports whether the provisional Steam Machine predicate
// holds: AMD RDNA3 from the step-1 classifier, a complete gfx11xx token from
// actual renderer evidence (not a vendor-only fallback), and no hybrid
// ambiguity. Non-Valve machines and missing GPU evidence cannot satisfy it.
func IsUnambiguousRDNA3(caps GPUCapabilities, probe GPUProbeInfo) bool {
	if caps.Vendor != GPUAMD || caps.Generation != "RDNA3" || caps.Ambiguous {
		return false
	}
	if probe.Source != ProbeRenderer && probe.Source != ProbeLspci {
		return false
	}
	renderer := caps.Renderer
	if strings.TrimSpace(renderer) == "" {
		renderer = probe.Renderer
	}
	if strings.TrimSpace(renderer) == "" || strings.HasPrefix(renderer, "undetected") {
		return false
	}
	return HasCompleteGFX11Token(renderer)
}
