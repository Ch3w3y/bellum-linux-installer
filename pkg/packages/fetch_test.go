package packages

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"bellum-installer/pkg/core"
)

func TestDownloadFileOverHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("payload"))
		case "/short":
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("only a bit"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := downloadClient
	downloadClient = srv.Client()
	defer func() { downloadClient = old }()
	logger, _ := core.NewLogger("")
	dir := t.TempDir()

	dest := filepath.Join(dir, "ok")
	if err := downloadFile(dest, srv.URL+"/ok", "", logger); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "payload" {
		t.Fatalf("got %q", got)
	}
	if err := downloadFile(dest, srv.URL+"/ok", "", logger); err == nil {
		t.Fatal("overwrote an existing file")
	}
	for _, path := range []string{"/missing", "/short"} {
		dest := filepath.Join(dir, path[1:])
		if err := downloadFile(dest, srv.URL+path, "", logger); err == nil {
			t.Errorf("%s: expected an error", path)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Errorf("%s: partial file left behind", path)
		}
	}
	if err := downloadFile(filepath.Join(dir, "plain"), "http://example.com/x", "", logger); err == nil {
		t.Error("plain HTTP accepted")
	}
}
