package core

import (
	"strings"
	"testing"
)

func TestParseOSReleaseQuoting(t *testing.T) {
	data := "# comment\nID=arch\nPRETTY_NAME=\"Arch \\\"Linux\\\"\"\nEMPTY=\nSINGLE='a b'\n"
	id, _, variant, _, pretty, _, diags := ParseOSRelease([]byte(data))
	if id != "arch" || variant != "" || pretty != `Arch "Linux"` {
		t.Fatalf("got id=%q variant=%q pretty=%q diags=%v", id, variant, pretty, diags)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
}

func TestParseOSReleaseDuplicateKeyKeepsFinal(t *testing.T) {
	data := "ID=arch\nID=cachyos\nUNKNOWN_KEY=whatever\nID_LIKE=\"arch\"\n"
	id, idLike, _, _, _, tokens, diags := ParseOSRelease([]byte(data))
	if id != "cachyos" {
		t.Fatalf("got id=%q", id)
	}
	if idLike != "arch" || len(tokens) != 1 || tokens[0] != "arch" {
		t.Fatalf("got idLike=%q tokens=%v", idLike, tokens)
	}
	found := false
	for _, d := range diags {
		if d.Code == "os-release/duplicate-key" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate-key diagnostic, got %v", diags)
	}
}

func TestParseOSReleaseMalformedValue(t *testing.T) {
	data := "ID=\"unterminated\nID_LIKE=arch\n"
	id, _, _, _, _, _, diags := ParseOSRelease([]byte(data))
	if id != "" {
		t.Fatalf("malformed ID must be ignored, got %q", id)
	}
	found := false
	for _, d := range diags {
		if d.Code == "os-release/malformed-value" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected malformed-value diagnostic, got %v", diags)
	}
}

func TestClassifyOSFamilyAliases(t *testing.T) {
	cases := map[string]OSFamily{
		"steamos": OSArch, "arch": OSArch, "cachyos": OSArch, "endeavouros": OSArch, "manjaro": OSArch,
		"bazzite": OSFedora, "fedora": OSFedora, "nobara": OSFedora, "rhel": OSFedora, "centos": OSFedora,
		"debian": OSDebian, "ubuntu": OSDebian, "linuxmint": OSDebian, "pop": OSDebian,
		"opensuse-tumbleweed": OSOpenSUSE, "opensuse-leap": OSOpenSUSE, "suse": OSOpenSUSE,
		"void": OSUnknown, "": OSUnknown,
	}
	for id, want := range cases {
		if got := ClassifyOSFamily(id, nil); got != want {
			t.Errorf("ClassifyOSFamily(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestClassifyOSFamilyIDLikeFallback(t *testing.T) {
	if got := ClassifyOSFamily("void", []string{"debian", "ubuntu"}); got != OSDebian {
		t.Fatalf("expected debian fallback, got %q", got)
	}
	if got := ClassifyOSFamily("something", []string{"unknown-token", "arch"}); got != OSArch {
		t.Fatalf("expected ordered arch fallback, got %q", got)
	}
	// No substring matching: "myarchfork" must not resolve to arch.
	if got := ClassifyOSFamily("myarchfork", nil); got != OSUnknown {
		t.Fatalf("substring IDs must stay unknown, got %q", got)
	}
	// Explicit ID wins over ID_LIKE.
	if got := ClassifyOSFamily("ubuntu", []string{"arch"}); got != OSDebian {
		t.Fatalf("explicit ID must outrank ID_LIKE, got %q", got)
	}
}

func TestClassifyImmutable(t *testing.T) {
	if v, _ := ClassifyImmutable("steamos", "", OSArch, false); v != TriYes {
		t.Error("steamos must be immutable")
	}
	if v, _ := ClassifyImmutable("bazzite", "", OSFedora, false); v != TriYes {
		t.Error("bazzite must be immutable")
	}
	if v, _ := ClassifyImmutable("fedora", "silverblue", OSFedora, false); v != TriYes {
		t.Error("fedora silverblue must be immutable")
	}
	if v, _ := ClassifyImmutable("fedora", "", OSFedora, true); v != TriYes {
		t.Error("ostree marker must mean immutable")
	}
	if v, _ := ClassifyImmutable("arch", "", OSArch, false); v != TriNo {
		t.Error("plain arch must be mutable")
	}
	if v, _ := ClassifyImmutable("fedora", "some-custom-spin", OSFedora, false); v != TriUnknown {
		t.Error("unrecognized variant must stay unknown")
	}
	if v, _ := ClassifyImmutable("", "", OSUnknown, false); v != TriUnknown {
		t.Error("missing identity must stay unknown")
	}
	// Bazzite keeps its own ID (never rewritten to fedora); only family maps.
	id, _, _, _, _, tokens, _ := ParseOSRelease([]byte("ID=bazzite\n"))
	if id != "bazzite" || ClassifyOSFamily(id, tokens) != OSFedora {
		t.Fatalf("bazzite identity/family: id=%q family=%q", id, ClassifyOSFamily(id, tokens))
	}
}

func TestSanitizeField(t *testing.T) {
	if got := SanitizeField("Valve\x00  ", 0); got != "Valve" {
		t.Fatalf("NUL/space trim: %q", got)
	}
	if got := SanitizeField("\x1b[31mred\x1b[0m", 0); got != "red" {
		t.Fatalf("ANSI strip: %q", got)
	}
	if got := SanitizeField("a\tb\x07c", 0); strings.ContainsAny(got, "\x07") {
		t.Fatalf("controls stripped: %q", got)
	}
	if got := SanitizeField(strings.Repeat("x", 500), 10); len(got) > 10 {
		t.Fatalf("length cap: %d", len(got))
	}
}
