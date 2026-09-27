package packages

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bellum-installer/pkg/core"
)

func umuArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, h := range []*tar.Header{
		{Name: "umu/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "umu/umu-run", Typeflag: tar.TypeReg, Mode: 0744, Size: 22},
		{Name: "umu/umu_run.py", Typeflag: tar.TypeSymlink, Linkname: "umu-run"},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			_, _ = tw.Write([]byte("#!/usr/bin/env python3\n"[:22]))
		}
	}
	_ = tw.Close()
	return buf.Bytes()
}

func TestEnsureUMUInstallsPinnedZipappOnce(t *testing.T) {
	archive := umuArchive(t)
	sum := sha256.Sum256(archive)
	pin := hex.EncodeToString(sum[:])
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	old := downloadClient
	downloadClient = srv.Client()
	defer func() { downloadClient = old }()
	logger, _ := core.NewLogger("")
	dir := filepath.Join(t.TempDir(), "umu", "1.4.4")

	path, err := ensureUMUFrom(dir, srv.URL+"/umu.tar", pin, logger)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("umu-run not installed executable: %v %v", info, err)
	}
	if _, err := ensureUMUFrom(dir, srv.URL+"/umu.tar", pin, logger); err != nil || hits != 1 {
		t.Fatalf("second call re-downloaded (hits=%d): %v", hits, err)
	}
	// A wrong pin must fail closed and leave the installed copy alone.
	if _, err := ensureUMUFrom(filepath.Join(t.TempDir(), "other"), srv.URL+"/umu.tar", "0000000000000000000000000000000000000000000000000000000000000000", logger); err == nil {
		t.Fatal("archive with the wrong digest accepted")
	}
}
