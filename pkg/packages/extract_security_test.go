package packages

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRejectsTraversalAndEscapingLinks(t *testing.T) {
	for _, tc := range []struct {
		name, link string
		kind       byte
	}{
		{name: "../escape", kind: tar.TypeReg},
		{name: "inside/link", link: "../../escape", kind: tar.TypeSymlink},
		{name: "inside/link", link: "/etc/passwd", kind: tar.TypeSymlink},
		{name: "inside/link", link: "../escape", kind: tar.TypeLink},
	} {
		t.Run(tc.name+tc.link, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "attack.tar")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			tw := tar.NewWriter(f)
			if err := tw.WriteHeader(&tar.Header{Name: tc.name, Linkname: tc.link, Typeflag: tc.kind, Mode: 0644}); err != nil {
				t.Fatal(err)
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			if err := ExtractPackageTo(archive, filepath.Join(root, "dest"), 0); err == nil {
				t.Fatal("unsafe tar accepted")
			}
		})
	}
}

func TestExtractRejectsSymlinkChainParentSegments(t *testing.T) {
	for _, ext := range []string{"tar", "tar.gz"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "chain."+ext)
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			var w *tar.Writer
			if ext == "tar.gz" {
				gz := gzip.NewWriter(f)
				w = tar.NewWriter(gz)
				defer gz.Close()
			} else {
				w = tar.NewWriter(f)
			}
			for _, h := range []*tar.Header{
				{Name: "root/b", Linkname: ".", Typeflag: tar.TypeSymlink, Mode: 0777},
				{Name: "root/a", Linkname: "b/c/../..", Typeflag: tar.TypeSymlink, Mode: 0777},
			} {
				if err := w.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			if err := ExtractPackageTo(archive, filepath.Join(root, "dest"), 1); err == nil {
				t.Fatal("symlink chain with parent segments accepted")
			}
		})
	}
}

func TestGZStripSkipsEntriesWithoutRootComponent(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "strip.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	w := tar.NewWriter(gz)
	if err := w.WriteHeader(&tar.Header{Name: "orphan", Typeflag: tar.TypeReg, Mode: 0644, Size: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "dest")
	if err := ExtractPackageTo(archive, dest, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "orphan")); !os.IsNotExist(err) {
		t.Fatalf("stripped orphan was extracted, err=%v", err)
	}
}

func writeTar(t *testing.T, path string, headers []*tar.Header) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := tar.NewWriter(f)
	for _, h := range headers {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := w.Write(make([]byte, h.Size)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// Proton-CachyOS links sibling protonfixes directories with "../" targets.
func TestExtractAllowsLeadingParentSymlinksInsideRoot(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "proton.tar")
	writeTar(t, archive, []*tar.Header{
		{Name: "proton/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "proton/protonfixes/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "proton/protonfixes/gamefixes-steam/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "proton/protonfixes/gamefixes-steam/61500.py", Typeflag: tar.TypeReg, Mode: 0644, Size: 3},
		{Name: "proton/protonfixes/gamefixes-umu/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "proton/protonfixes/gamefixes-umu/61500.py", Linkname: "../gamefixes-steam/61500.py", Typeflag: tar.TypeSymlink, Mode: 0777},
	})
	dest := filepath.Join(root, "dest")
	if err := ExtractPackageTo(archive, dest, 1); err != nil {
		t.Fatalf("Proton-style relative symlink rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "protonfixes", "gamefixes-umu", "61500.py")); err != nil {
		t.Fatalf("link does not resolve: %v", err)
	}
}

func TestExtractRejectsUnsafeParentSymlinks(t *testing.T) {
	cases := map[string][]*tar.Header{
		"escapes root": {
			{Name: "proton/a/", Typeflag: tar.TypeDir, Mode: 0755},
			{Name: "proton/a/link", Linkname: "../../outside", Typeflag: tar.TypeSymlink, Mode: 0777},
		},
		"parent after segment": {
			{Name: "proton/a/", Typeflag: tar.TypeDir, Mode: 0755},
			{Name: "proton/a/link", Linkname: "x/../../outside", Typeflag: tar.TypeSymlink, Mode: 0777},
		},
		"under symlinked dir": {
			{Name: "proton/real/", Typeflag: tar.TypeDir, Mode: 0755},
			{Name: "proton/sym", Linkname: "real", Typeflag: tar.TypeSymlink, Mode: 0777},
			{Name: "proton/sym/link", Linkname: "../x", Typeflag: tar.TypeSymlink, Mode: 0777},
		},
		"symlinked dir listed later": {
			{Name: "proton/sym/link", Linkname: "../x", Typeflag: tar.TypeSymlink, Mode: 0777},
			{Name: "proton/sym", Linkname: "/etc", Typeflag: tar.TypeSymlink, Mode: 0777},
		},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "bad.tar")
			writeTar(t, archive, headers)
			if err := ExtractPackageTo(archive, filepath.Join(root, "dest"), 1); err == nil {
				t.Fatal("unsafe symlink accepted")
			}
		})
	}
}
