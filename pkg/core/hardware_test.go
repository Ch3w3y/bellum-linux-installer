package core

import (
	"strings"
	"testing"
)

func deckCaps() (GPUCapabilities, GPUProbeInfo) {
	renderer := "AMD Radeon Graphics (radeonsi, gfx1033, ACO, DRM 3.57)"
	caps := ClassifyGPUCapabilities(renderer)
	caps.Renderer = renderer
	return caps, GPUProbeInfo{Source: ProbeRenderer, Renderer: renderer, DeviceAssociation: "known", Provenance: "test"}
}

func TestClassifyHardwareExactDeck(t *testing.T) {
	caps, probe := deckCaps()
	for product, sku := range map[string]HardwareSKU{"Jupiter": SKUDeckLCD, "Galileo": SKUDeckOLED} {
		info, diags := ClassifyHardware("Valve", product, caps, probe)
		if info.SKU != sku || info.Match != MatchExact {
			t.Errorf("%s: got %+v diags=%v", product, info, diags)
		}
	}
}

func TestClassifyHardwareValveRequiresExactVendor(t *testing.T) {
	caps, probe := deckCaps()
	info, _ := ClassifyHardware("Valve Corporation", "Jupiter", caps, probe)
	if info.SKU != SKUGeneric {
		t.Fatalf("non-exact vendor must not match Valve: %+v", info)
	}
}

func TestClassifyHardwareTrimsNULAndCase(t *testing.T) {
	caps, probe := deckCaps()
	info, _ := ClassifyHardware("VALVE\x00", "  galileo\x00 ", caps, probe)
	if info.SKU != SKUDeckOLED {
		t.Fatalf("trim/case: %+v", info)
	}
}

func TestClassifyHardwareMissingDMIIsUnknown(t *testing.T) {
	caps, probe := deckCaps()
	info, diags := ClassifyHardware("", "", caps, probe)
	if info.SKU != SKUUnknown || info.Match != MatchUnknown {
		t.Fatalf("got %+v", info)
	}
	if len(diags) == 0 {
		t.Fatal("expected a missing-DMI diagnostic")
	}
}

func TestClassifyHardwareGeneric(t *testing.T) {
	caps, probe := deckCaps()
	info, diags := ClassifyHardware("LENOVO", "82JM", caps, probe)
	if info.SKU != SKUGeneric || len(diags) != 0 {
		t.Fatalf("got %+v diags=%v", info, diags)
	}
}

func TestDeckOutranksHeuristicWithExternalRDNA3(t *testing.T) {
	renderer := "AMD Radeon RX 7900 XTX gfx1100"
	caps := ClassifyGPUCapabilities(renderer)
	caps.Renderer = renderer
	probe := GPUProbeInfo{Source: ProbeRenderer, Renderer: renderer, DeviceAssociation: "known"}
	info, diags := ClassifyHardware("Valve", "Galileo", caps, probe)
	if info.SKU != SKUDeckOLED || info.Match != MatchExact {
		t.Fatalf("Deck mapping must win: %+v", info)
	}
	if caps.Generation != "RDNA3" {
		t.Fatalf("DMI must not overwrite GPU generation: %+v", caps)
	}
	found := false
	for _, d := range diags {
		if d.Code == "hardware/gpu-conflict" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected gpu-conflict diagnostic, got %v", diags)
	}
}

func TestSteamMachineCandidateRule(t *testing.T) {
	renderer := "AMD Radeon RX 7600M XT gfx1102"
	caps := ClassifyGPUCapabilities(renderer)
	caps.Renderer = renderer
	probe := GPUProbeInfo{Source: ProbeRenderer, Renderer: renderer, DeviceAssociation: "known"}
	info, _ := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", caps, probe)
	if info.SKU != SKUSteamMachineCandidate || info.Match != MatchHeuristic {
		t.Fatalf("candidate: %+v", info)
	}
	if info.ProductName != "SYNTHETIC-MACHINE-01" {
		t.Fatalf("unknown product string must be retained: %+v", info)
	}
}

func TestSteamMachineNegatives(t *testing.T) {
	rdna3 := "AMD Radeon RX 7900 XTX gfx1100"
	caps := ClassifyGPUCapabilities(rdna3)
	caps.Renderer = rdna3
	// Non-Valve machine with gfx11xx can never satisfy the rule.
	if info, _ := ClassifyHardware("ASUS", "ROG", caps, GPUProbeInfo{Source: ProbeRenderer, Renderer: rdna3}); info.SKU == SKUSteamMachineCandidate {
		t.Fatal("non-Valve gfx11xx must not be a candidate")
	}
	// Vendor-only DRM fallback is not actual renderer evidence.
	drmCaps := GPUCapabilities{Vendor: GPUAMD, Generation: "RDNA3", Renderer: "DRM device card0 (AMD, vendor 0x1002, no GL renderer)"}
	if info, _ := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", drmCaps, GPUProbeInfo{Source: ProbeDRM, Renderer: drmCaps.Renderer}); info.SKU == SKUSteamMachineCandidate {
		t.Fatal("DRM fallback must not satisfy the rule")
	}
	// RX marketing name without the gfx token is insufficient.
	rxOnly := "AMD Radeon RX 7900 XTX"
	rxCaps := ClassifyGPUCapabilities(rxOnly)
	rxCaps.Renderer = rxOnly
	if info, _ := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", rxCaps, GPUProbeInfo{Source: ProbeRenderer, Renderer: rxOnly}); info.SKU == SKUSteamMachineCandidate {
		t.Fatal("marketing name without gfx11xx must not satisfy the rule")
	}
	// Ambiguous hybrid evidence is insufficient.
	amb := ClassifyGPUCapabilities(rdna3 + " PRIME hybrid")
	amb.Renderer = rdna3
	if info, _ := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", amb, GPUProbeInfo{Source: ProbeRenderer, Renderer: rdna3}); info.SKU == SKUSteamMachineCandidate {
		t.Fatal("hybrid ambiguity must not satisfy the rule")
	}
	// Missing GPU evidence is insufficient.
	if info, _ := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", GPUCapabilities{Vendor: GPUUnknown}, GPUProbeInfo{Source: ProbeUnknown}); info.SKU == SKUSteamMachineCandidate {
		t.Fatal("missing GPU must not satisfy the rule")
	}
	// Unmatched Valve without evidence stays unknown with a diagnostic.
	info, diags := ClassifyHardware("Valve", "SYNTHETIC-MACHINE-01", GPUCapabilities{Vendor: GPUUnknown}, GPUProbeInfo{Source: ProbeUnknown})
	if info.SKU != SKUUnknown {
		t.Fatalf("unmatched Valve: %+v", info)
	}
	found := false
	for _, d := range diags {
		if d.Code == "hardware/valve-unmatched" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected valve-unmatched diagnostic: %v", diags)
	}
}

func TestHasCompleteGFX11Token(t *testing.T) {
	if !HasCompleteGFX11Token("radeonsi, gfx1100") || !HasCompleteGFX11Token("GFX1151") {
		t.Fatal("complete tokens must match")
	}
	if HasCompleteGFX11Token("gfx11") || HasCompleteGFX11Token("RX 7900 XTX") {
		t.Fatal("incomplete/marketing text must not match")
	}
}

func TestProfileForPureAndKey(t *testing.T) {
	caps, _ := deckCaps()
	p := Platform{
		OS:       OSInfo{ID: "steamos", VariantID: "", Family: OSArch},
		Hardware: HardwareInfo{SKU: SKUDeckOLED, Match: MatchExact},
		GPU:      caps,
		Session:  SessionInfo{Kind: SessionGamescope, GameMode: TriYes},
	}
	prof := ProfileFor(p)
	if prof.Version != ProfileVersion || prof.GPUVendor != GPUAMD || prof.GPUGeneration != "RDNA2" {
		t.Fatalf("profile: %+v", prof)
	}
	key := prof.Key()
	for _, want := range []string{"v1", "steamos", "deck-oled", "RDNA2", "gamescope"} {
		if !strings.Contains(key, want) {
			t.Fatalf("key %q lacks %q", key, want)
		}
	}
	// Unknown generation normalizes to "unknown", never empty.
	p.GPU = GPUCapabilities{Vendor: GPUUnknown}
	if got := ProfileFor(p).GPUGeneration; got != "unknown" {
		t.Fatalf("unknown generation: %q", got)
	}
	if got := ProfileFor(p).Key(); !strings.Contains(got, "unknown") {
		t.Fatalf("key must carry unknown: %q", got)
	}
}
