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
