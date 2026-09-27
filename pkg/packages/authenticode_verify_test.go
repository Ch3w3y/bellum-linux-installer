package packages

import (
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures come from testdata/authenticode/gen.sh: a throwaway test CA signs
// a 1 KiB PE with osslsigncode, and the same files verify with osslsigncode.

const fixtureSigner = "ASTARTE INDUSTRIES INC."

func fixtureRoots(t *testing.T) *x509.CertPool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "authenticode", "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "authenticode", name)
}

var fixtureNow = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

func TestAuthenticodeAcceptsValidSignatures(t *testing.T) {
	roots := fixtureRoots(t)
	for _, name := range []string{"signed.exe", "signed-ts.exe", "expired-ts.exe"} {
		if err := verifyLauncherAuthenticodeWith(fixture(t, name), fixtureSigner, roots, fixtureNow); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	data, _ := os.ReadFile(fixture(t, "expired-ts.exe"))
	sig, err := VerifyAuthenticode(data, roots, fixtureNow)
	if err != nil || !sig.Timestamp.Equal(time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("timestamp = %v, %v", sig, err)
	}
}

func TestAuthenticodeRejections(t *testing.T) {
	roots := fixtureRoots(t)
	for name, want := range map[string]string{
		"unsigned.exe": "no signature",
		"impostor.exe": "does not match approved signer",
		"noeku.exe":    "not trusted",
		"expired.exe":  "not trusted",
	} {
		err := verifyLauncherAuthenticodeWith(fixture(t, name), fixtureSigner, roots, fixtureNow)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want error containing %q", name, err, want)
		}
	}
	// A different trust store must not accept the test CA's signature.
	if err := verifyLauncherAuthenticodeWith(fixture(t, "signed.exe"), fixtureSigner, x509.NewCertPool(), fixtureNow); err == nil {
		t.Error("signature accepted without a trusted root")
	}
	// The signer check is exact and case-sensitive.
	for _, other := range []string{"ASTARTE", "astarte industries inc.", "ASTARTE INDUSTRIES INC. "} {
		if err := verifyLauncherAuthenticodeWith(fixture(t, "signed.exe"), other, roots, fixtureNow); err == nil {
			t.Errorf("signer %q accepted", other)
		}
	}
}

func TestAuthenticodeDetectsTampering(t *testing.T) {
	roots := fixtureRoots(t)
	orig, err := os.ReadFile(fixture(t, "signed-ts.exe"))
	if err != nil {
		t.Fatal(err)
	}
	certOff := int(binary.LittleEndian.Uint32(orig[0x44+20+112+32:]))
	cases := map[string]func([]byte) []byte{
		"code byte": func(b []byte) []byte { b[0x201] ^= 0xff; return b },
		"header":    func(b []byte) []byte { b[0x50] ^= 0x01; return b },
		"appended":  func(b []byte) []byte { return append(b, []byte("payload")...) },
		"signature": func(b []byte) []byte { b[len(b)-40] ^= 0xff; return b },
		"cert size": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[0x44+20+112+36:], uint32(len(b)-certOff-8))
			return b
		},
	}
	for name, mutate := range cases {
		data := mutate(append([]byte(nil), orig...))
		if _, err := VerifyAuthenticode(data, roots, fixtureNow); err == nil {
			t.Errorf("%s tampering accepted", name)
		}
	}
	// The PE checksum field is excluded from the digest by design.
	data := append([]byte(nil), orig...)
	data[0x44+20+64] ^= 0xff
	if _, err := VerifyAuthenticode(data, roots, fixtureNow); err != nil {
		t.Errorf("checksum change should not matter: %v", err)
	}
}

// TestAuthenticodeRealWorldSample verifies a real signed installer against the
// system trust store. Run it with, for example:
//
//	BELLUM_AUTHENTICODE_SAMPLE=rufus-4.6.exe BELLUM_AUTHENTICODE_SIGNER="Akeo Consulting" go test ./pkg/packages -run RealWorld
func TestAuthenticodeRealWorldSample(t *testing.T) {
	path := os.Getenv("BELLUM_AUTHENTICODE_SAMPLE")
	if path == "" {
		t.Skip("set BELLUM_AUTHENTICODE_SAMPLE to a signed .exe")
	}
	roots, err := launcherRoots()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyLauncherAuthenticodeWith(path, os.Getenv("BELLUM_AUTHENTICODE_SIGNER"), roots, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedRootsMatchPins(t *testing.T) {
	roots, err := embeddedRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != len(pinnedRoots) {
		t.Fatalf("%d embedded roots, %d pinned", len(roots), len(pinnedRoots))
	}
	for _, r := range roots {
		if !r.IsCA {
			t.Errorf("%s is not a CA certificate", r.Subject)
		}
	}
}
