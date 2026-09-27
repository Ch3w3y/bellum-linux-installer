package packages

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bellum-installer/pkg/config"
)

// VerifyLauncherAuthenticode requires a valid Authenticode signature, chained
// to the system's trusted roots, whose leaf certificate has exactly one
// common name equal to the approved signer (case-sensitive).
func VerifyLauncherAuthenticode(path string) error {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("load the system's trusted certificates: %w", err)
	}
	return verifyLauncherAuthenticodeWith(path, config.DefaultVersions.LauncherSigner, roots, time.Now())
}

func verifyLauncherAuthenticodeWith(path, approvedSigner string, roots *x509.CertPool, now time.Time) error {
	if approvedSigner == "" {
		return fmt.Errorf("missing AstarteLauncher Authenticode signer pin")
	}
	image, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := VerifyAuthenticode(image, roots, now)
	if err != nil {
		return fmt.Errorf("AstarteLauncher Authenticode verification failed: %w", err)
	}
	cn, err := SignerCommonName(sig.Signer)
	if err != nil || cn != approvedSigner {
		return fmt.Errorf("AstarteLauncher signer %q does not match approved signer %q", sig.Signer.Subject.String(), approvedSigner)
	}
	return nil
}

// LauncherCheck reports what launcher verification established. The
// Authenticode signer is the mandatory control; the SHA-256 allowlist only
// records builds that were inspected, because Astarte's download URL is not
// versioned and always serves the current build.
type LauncherCheck struct {
	Digest      string
	KnownDigest bool
}

// Warning returns the message to log for a signed build that is not in the
// allowlist, or "" when the digest is known.
func (c LauncherCheck) Warning() string {
	if c.KnownDigest {
		return ""
	}
	return fmt.Sprintf("AstarteLauncher installer SHA-256 %s is not in the inspected allowlist; accepting it because its Authenticode signature from %q is valid. Add it to LauncherSHA256Allowlist after review.", c.Digest, config.DefaultVersions.LauncherSigner)
}

func digestAllowed(digest string, allowlist []string) bool {
	for _, pin := range allowlist {
		if len(pin) == 64 && strings.EqualFold(digest, pin) {
			return true
		}
	}
	return false
}

// VerifyLauncherInstaller requires a valid Authenticode signature from the
// approved signer and reports whether the digest is a known build.
func VerifyLauncherInstaller(path string) (LauncherCheck, error) {
	return verifyLauncherInstaller(path, config.DefaultVersions.LauncherSHA256Allowlist, VerifyLauncherAuthenticode)
}

func verifyLauncherInstaller(path string, allowlist []string, verify func(string) error) (LauncherCheck, error) {
	f, err := os.Open(path)
	if err != nil {
		return LauncherCheck{}, err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if err := firstErr(copyErr, closeErr); err != nil {
		return LauncherCheck{}, err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if err := verify(path); err != nil {
		return LauncherCheck{}, err
	}
	return LauncherCheck{Digest: digest, KnownDigest: digestAllowed(digest, allowlist)}, nil
}

// StageLauncherInstaller copies an untrusted source into a private directory,
// computes its digest during that copy, and verifies only the staged file.
func StageLauncherInstaller(source string) (string, string, LauncherCheck, error) {
	return stageLauncherInstaller(source, config.DefaultVersions.LauncherSHA256Allowlist, VerifyLauncherAuthenticode)
}

func stageLauncherInstaller(source string, pins []string, verify func(string) error) (string, string, LauncherCheck, error) {
	privateDir, err := os.MkdirTemp("", "bellum-launcher-*")
	if err != nil {
		return "", "", LauncherCheck{}, err
	}
	if err := os.Chmod(privateDir, 0700); err != nil {
		os.RemoveAll(privateDir)
		return "", "", LauncherCheck{}, err
	}
	staged := filepath.Join(privateDir, "launcher-installer.exe")
	in, err := os.Open(source)
	if err != nil {
		os.RemoveAll(privateDir)
		return "", "", LauncherCheck{}, err
	}
	out, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		in.Close()
		os.RemoveAll(privateDir)
		return "", "", LauncherCheck{}, err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), in)
	closeOutErr, closeInErr := out.Close(), in.Close()
	if copyErr != nil || closeOutErr != nil || closeInErr != nil {
		os.RemoveAll(privateDir)
		return "", "", LauncherCheck{}, fmt.Errorf("copy launcher installer: %w", firstErr(copyErr, closeOutErr, closeInErr))
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if err := verify(staged); err != nil {
		os.RemoveAll(privateDir)
		return "", "", LauncherCheck{}, err
	}
	return staged, privateDir, LauncherCheck{Digest: digest, KnownDigest: digestAllowed(digest, pins)}, nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
