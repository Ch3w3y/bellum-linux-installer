package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickProtonTakesNewestStableSLR(t *testing.T) {
	releases := []release{
		{TagName: "cachyos-11.0-20260901-slr", Prerelease: true, Assets: []asset{{Name: "proton-cachyos-11.0-20260901-slr-x86_64.tar.xz"}}},
		{TagName: "cachyos-11.0-20260820", Assets: []asset{{Name: "proton-cachyos-11.0-20260820-x86_64.tar.xz"}}},
		{TagName: "cachyos-11.0-20260815-slr", Assets: []asset{{Name: "proton-cachyos-11.0-20260815-slr-x86_64_v3.tar.xz"}}},
		{TagName: "cachyos-11.0-20260810-slr", Assets: []asset{{Name: "proton-cachyos-11.0-20260810-slr-x86_64.tar.xz", Digest: "sha256:ab"}}},
		{TagName: "cachyos-11.0-20260703-slr", Assets: []asset{{Name: "proton-cachyos-11.0-20260703-slr-x86_64.tar.xz"}}},
	}
	rel, a, err := pickProton(releases, "")
	if err != nil || rel.TagName != "cachyos-11.0-20260810-slr" || a.Digest != "sha256:ab" {
		t.Fatalf("picked %q %+v, %v", rel.TagName, a, err)
	}
	if _, _, err := pickProton(releases[:3], ""); err == nil {
		t.Fatal("prerelease, non-SLR and v3-only releases must not be picked")
	}
}

func TestPickUMUNeedsZipapp(t *testing.T) {
	releases := []release{
		{TagName: "1.5.0", Draft: true, Assets: []asset{{Name: "umu-launcher-1.5.0-zipapp.tar"}}},
		{TagName: "1.4.5", Assets: []asset{{Name: "umu-launcher-1.4.5.tar.gz"}}},
		{TagName: "1.4.4", Assets: []asset{{Name: "umu-launcher-1.4.4-zipapp.tar"}}},
	}
	if rel, _, err := pickUMU(releases, ""); err != nil || rel.TagName != "1.4.4" {
		t.Fatalf("picked %q, %v", rel.TagName, err)
	}
}

func TestCheckDigest(t *testing.T) {
	if err := checkDigest(asset{Digest: "sha256:ABC"}, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := checkDigest(asset{Digest: "sha256:abc"}, "def"); err == nil {
		t.Fatal("digest mismatch accepted")
	}
	if err := checkDigest(asset{}, "def"); err != nil {
		t.Fatal("a missing published digest must not fail")
	}
}

func TestWinetricksVersion(t *testing.T) {
	if v := winetricksVersion([]byte("#!/bin/sh\nWINETRICKS_VERSION=20260125-next\n")); v != "20260125-next" {
		t.Fatalf("got %q", v)
	}
}

func TestWriteVersionsAndDoc(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "..", versionsFile))
	if err != nil {
		t.Fatal(err)
	}
	vf := filepath.Join(dir, "versions.go")
	if err := os.WriteFile(vf, src, 0644); err != nil {
		t.Fatal(err)
	}
	p := pins{
		ProtonVer: "proton-cachyos-11.0-20260810-slr-x86_64", ProtonSHA256: strings.Repeat("1", 64),
		WinetricksVer: "bundled with pinned Proton (20260801)", UMUVersion: "1.4.5", UMUZipappSHA256: strings.Repeat("2", 64),
		LauncherSHA256Allowlist: []string{strings.Repeat("3", 64), strings.Repeat("4", 64)},
	}
	if err := writeVersions(vf, p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(vf)
	for _, want := range []string{
		`ProtonVer:     "proton-cachyos-11.0-20260810-slr-x86_64"`,
		`"` + strings.Repeat("1", 64) + `"`,
		`UMUVersion:      "1.4.5"`,
		`[]string{"` + strings.Repeat("3", 64) + `", "` + strings.Repeat("4", 64) + `"}`,
		`WinetricksVer: "bundled with pinned Proton (20260801)"`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("versions.go lacks %s", want)
		}
	}

	doc := filepath.Join(dir, "pins.md")
	if err := os.WriteFile(doc, []byte("intro\n"+docBegin+"\nold\n"+docEnd+"\noutro\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writePinsDoc(doc, p); err != nil {
		t.Fatal(err)
	}
	d, _ := os.ReadFile(doc)
	if strings.Contains(string(d), "old") || !strings.Contains(string(d), "cachyos-11.0-20260810-slr") || !strings.HasSuffix(string(d), "outro\n") {
		t.Fatalf("doc not regenerated:\n%s", d)
	}
}
