package packages

import (
	"archive/tar"
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
