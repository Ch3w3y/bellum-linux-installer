package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func emptySources() DetectionSources {
	return DetectionSources{
		Files:    newFixtureFS(),
		Env:      fixtureEnv(nil),
		Home:     "/home/tester",
		GPU:      fixtureGPU("", "unknown", "", "", "test:empty", false),
		EUID:     1000,
		HaveEUID: true,
	}
}

func TestDetectRequiresDependencies(t *testing.T) {
	for _, src := range []DetectionSources{
		{Env: fixtureEnv(nil), Home: "/home/tester", GPU: emptySources().GPU},
		{Files: newFixtureFS(), Home: "/home/tester", GPU: emptySources().GPU},
		{Files: newFixtureFS(), Env: fixtureEnv(nil), Home: "/home/tester"},
	} {
		_, err := DetectPlatformWith(context.Background(), src)
		if err == nil {
			t.Fatal("missing dependency must be a programmer error")
		}
		var partial *PartialError
		if !errors.As(err, &partial) {
			t.Fatalf("error must carry a partial snapshot: %T", err)
		}
	}
}

func TestDetectCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DetectPlatformWith(ctx, emptySources())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	var partial *PartialError
	if !errors.As(err, &partial) {
		t.Fatalf("cancelled error must carry a partial snapshot: %T", err)
	}
}

func TestDetectTotalDiscoveryFailure(t *testing.T) {
	platform, err := DetectPlatformWith(context.Background(), emptySources())
	if err != nil {
		t.Fatalf("missing evidence is partial success, not failure: %v", err)
	}
	if platform.OS.Family != OSUnknown || platform.Hardware.SKU != SKUUnknown {
		t.Fatalf("unknowns: %+v %+v", platform.OS, platform.Hardware)
	}
	if platform.Session.Kind != SessionHeadless {
		t.Fatalf("session: %+v", platform.Session)
	}
	if len(platform.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for missing evidence")
	}
	prof := ProfileFor(platform)
	if !strings.HasPrefix(prof.Key(), "v1|") {
		t.Fatalf("key: %q", prof.Key())
	}
}

func TestDetectOversizedOSRelease(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile("/etc/os-release", make([]byte, ReadLimitOSRelease+1))
	src := emptySources()
	src.Files = fsys
	platform, err := DetectPlatformWith(context.Background(), src)
	if err != nil {
		t.Fatalf("oversized is partial success: %v", err)
	}
	if platform.OS.ID != "" {
		t.Fatalf("oversized os-release must not parse: %+v", platform.OS)
	}
	found := false
	for _, d := range platform.Diagnostics {
		if d.Subsystem == "os" && d.Outcome == OutcomeUnsupported {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected oversized diagnostic: %v", platform.Diagnostics)
	}
}

func TestDetectDiagnosticsDeterministic(t *testing.T) {
	first, err := DetectPlatformWith(context.Background(), emptySources())
	if err != nil {
		t.Fatal(err)
	}
	second, err := DetectPlatformWith(context.Background(), emptySources())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Diagnostics) != len(second.Diagnostics) {
		t.Fatal("diagnostic counts differ")
	}
	for i := range first.Diagnostics {
		if first.Diagnostics[i] != second.Diagnostics[i] {
			t.Fatalf("diagnostics not deterministic at %d", i)
		}
	}
}

func TestDetectNeverTouchesHost(t *testing.T) {
	canary := "HOST-CANARY-9f2c41"
	fsys := newFixtureFS()
	// An escaping link pointed at a host-looking path must resolve to
	// missing, never to host content.
	fsys.addLink("/home/tester/.steam/steam", "HOST:/etc/passwd")
	fsys.addFile("/etc/os-release", []byte("ID="+canary+"\n"))
	src := DetectionSources{
		Files:    fsys,
		Env:      fixtureEnv(map[string]string{"HOME": "/tmp/" + canary, "XDG_CURRENT_DESKTOP": canary}),
		Home:     "/tmp/" + canary,
		GPU:      fixtureGPU("renderer "+canary, "renderer", "", "", "test:canary", false),
		EUID:     1000,
		HaveEUID: true,
	}
	platform, err := DetectPlatformWith(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	// The canary flows only through injected env/GPU/data, which is
	// expected; what must never happen is host-file content appearing.
	for _, in := range platform.Steam.Installs {
		if in.Root == "/etc/passwd" {
			t.Fatal("escaping link reached host content")
		}
	}
	if platform.OS.ID != strings.ToLower(canary) {
		t.Fatalf("injected os-release must be honored: %+v", platform.OS)
	}
}

func TestDetectGPUErrorIsPartial(t *testing.T) {
	src := emptySources()
	src.GPU = fixtureGPU("", "unknown", "", "", "test:fail", true)
	platform, err := DetectPlatformWith(context.Background(), src)
	if err != nil {
		t.Fatalf("GPU failure is partial success: %v", err)
	}
	if platform.GPU.Vendor != GPUUnknown {
		t.Fatalf("gpu: %+v", platform.GPU)
	}
	found := false
	for _, d := range platform.Diagnostics {
		if d.Code == "gpu/probe-failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected gpu/probe-failed: %v", platform.Diagnostics)
	}
}

func TestDetectOSFallbackOnlyWhenMissing(t *testing.T) {
	// Denied preferred file: diagnostic, no silent replacement.
	fsys := newFixtureFS()
	fsys.addDenied("/etc/os-release")
	fsys.addFile("/usr/lib/os-release", []byte("ID=arch\n"))
	src := emptySources()
	src.Files = fsys
	platform, _ := DetectPlatformWith(context.Background(), src)
	if platform.OS.ID != "" {
		t.Fatalf("denied preferred file must not fall back silently: %+v", platform.OS)
	}
	// Absent preferred file: fallback is used.
	fsys = newFixtureFS()
	fsys.addFile("/usr/lib/os-release", []byte("ID=arch\n"))
	src.Files = fsys
	platform, _ = DetectPlatformWith(context.Background(), src)
	if platform.OS.ID != "arch" || platform.OS.Family != OSArch {
		t.Fatalf("fallback: %+v", platform.OS)
	}
}
