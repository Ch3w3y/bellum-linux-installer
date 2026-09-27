package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/config"
)

// VerifyLauncherAuthenticode requires a valid signature and the approved signer.
func VerifyLauncherAuthenticode(path string) error {
	signer := config.DefaultVersions.LauncherSigner
	if signer == "" {
		return fmt.Errorf("missing AstarteLauncher Authenticode signer pin")
	}
	if _, err := exec.LookPath("osslsigncode"); err != nil {
		return fmt.Errorf("osslsigncode is required for launcher Authenticode verification: %w", err)
	}
	output, err := exec.Command("osslsigncode", "verify", "-in", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("AstarteLauncher Authenticode verification failed: %w", err)
	}
	if !hasApprovedLeafSubject(string(output), signer) {
		return fmt.Errorf("AstarteLauncher signer does not match approved signer")
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

func hasApprovedLeafSubject(output, approvedCN string) bool {
	lines := strings.Split(output, "\n")
	inLeaf, foundLeaf, match := false, false, false
	subjectCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Signer #") {
			if inLeaf {
				break
			}
			inLeaf = line == "Signer #0:"
			foundLeaf = inLeaf
			continue
		}
		if inLeaf && strings.HasPrefix(line, "Subject:") {
			subjectCount++
			subject := strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
			cn, count := subjectCommonNames(subject)
			match = count == 1 && cn == approvedCN
		}
	}
	return foundLeaf && subjectCount == 1 && match
}

// subjectCommonNames parses both RFC 2253 (comma-separated) and the older
// OpenSSL X509_NAME_oneline slash-separated subject format. Escaped separators
// stay inside their RDN value, so a fake CN embedded in another attribute is
// never treated as an actual common name.
func subjectCommonNames(subject string) (string, int) {
	separator := byte(',')
	plusSeparators := true
	if strings.HasPrefix(subject, "/") {
		separator = '/'
		plusSeparators = false
		subject = subject[1:]
	}
	var rdns []string
	start, escaped := 0, false
	for i := 0; i < len(subject); i++ {
		if escaped {
			escaped = false
			continue
		}
		if subject[i] == '\\' {
			escaped = true
			continue
		}
		if subject[i] == separator || (plusSeparators && subject[i] == '+') {
			rdns = append(rdns, subject[start:i])
			start = i + 1
		}
	}
	rdns = append(rdns, subject[start:])
	var commonName string
	count := 0
	for _, rdn := range rdns {
		key, value, ok := strings.Cut(rdn, "=")
		if !ok || strings.TrimSpace(key) != "CN" {
			continue
		}
		count++
		commonName = unescapeSubjectValue(strings.TrimSpace(value))
	}
	return commonName, count
}

func unescapeSubjectValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			i++
		}
		b.WriteByte(value[i])
	}
	return strings.TrimSpace(b.String())
}
