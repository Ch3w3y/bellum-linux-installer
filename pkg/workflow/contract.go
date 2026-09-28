package workflow

import (
	"regexp"
	"sort"
)

// ProtonRequirement is one thing the installer relies on inside a Proton
// build: Needle must appear in the file at Path (relative to the archive's
// top directory). tools/pinupdate checks every candidate Proton release
// against this list before proposing it, so an upstream change that would
// silently turn one of our settings into a no-op blocks the update instead.
type ProtonRequirement struct {
	Path, Needle, Why string
}

// ProtonContract lists everything the installer and launch_vars.env depend on.
func ProtonContract() []ProtonRequirement {
	reqs := []ProtonRequirement{
		{"proton", "PROTON_FSR4_UPGRADE", "RDNA4 launch setting"},
		{"proton", "PROTON_DLSS_UPGRADE", "kept off in every configuration"},
		{"proton", "PROTON_NVIDIA_LIBS", "NVIDIA launch setting"},
		{"proton", "PROTON_DXVK_D3D8", "launch setting"},
		{"proton", "nvngx.dll", "DLSS through the NVIDIA driver"},
		{"protonfixes/upscalers.py", "PROTON_FSR4_UPGRADE", "FSR4 driver component handling"},
		{"files/lib/wine/x86_64-unix/ntdll.so", "PROTON_EAC_RUNTIME", "Easy Anti-Cheat runtime bridge"},
		{"files/lib/wine/x86_64-windows/amdxc64.dll", "FSR4_UPGRADE", "FSR4 provider for AMD GPUs"},
		{"files/lib/wine/vkd3d-proton/x86_64-windows/d3d12core.dll", "descriptor_heap", "VKD3D_CONFIG option"},
		{"files/lib/wine/dxvk/x86_64-windows/dxgi.dll", "DXVK_ENABLE_NVAPI", "NVIDIA launch setting"},
		// minNVIDIADriver is DXVK 3.x's documented minimum; a new DXVK major
		// needs that minimum reviewed before the pin moves.
		{"files/lib/wine/dxvk/version", "dxvk (v3.", "NVIDIA minimum driver check (" + minNVIDIADriver + ")"},
	}
	for _, verb := range WinetricksVerbs() {
		reqs = append(reqs, ProtonRequirement{"protonfixes/winetricks", "w_metadata " + verb + " ", "winetricks verb used during install"})
	}
	return reqs
}

// WinetricksVerbs lists every winetricks verb and setting the installer runs.
func WinetricksVerbs() []string {
	return append(append([]string{}, requiredWinetricksVerbs...), "win11", "remove_mono", "grabfullscreen=y", "windowmanagerdecorated=n", "mwo=disable")
}

var protonVarRe = regexp.MustCompile(`\bPROTON_[A-Z0-9_]+`)

// protonVarsIn returns the PROTON_* names in a launch_vars.env body.
func protonVarsIn(content string) []string {
	seen := map[string]bool{}
	for _, v := range protonVarRe.FindAllString(content, -1) {
		seen[v] = true
	}
	var out []string
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
