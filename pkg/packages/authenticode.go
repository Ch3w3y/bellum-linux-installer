package packages

import (
	"fmt"
	"os/exec"
	"regexp"
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

func VerifyLauncherInstaller(path string) error {
	if len(config.DefaultVersions.LauncherSHA256Allowlist) == 0 {
		return fmt.Errorf("approved AstarteLauncher SHA-256 pin is required")
	}
	var accepted bool
	for _, digest := range config.DefaultVersions.LauncherSHA256Allowlist {
		if len(digest) == 64 && VerifySHA256(path, digest) == nil {
			accepted = true
			break
		}
	}
	if !accepted {
		return fmt.Errorf("AstarteLauncher SHA-256 is not in the approved allowlist")
	}
	return VerifyLauncherAuthenticode(path)
}

var commonNameRE = regexp.MustCompile(`(?:^|,)\s*CN\s*=\s*([^,]+)`)

func hasApprovedLeafSubject(output, approvedCN string) bool {
	lines := strings.Split(output, "\n")
	inLeaf, foundLeaf, match := false, false, false
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
			subject := strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
			cn := commonNameRE.FindStringSubmatch(subject)
			match = len(cn) == 2 && strings.EqualFold(strings.TrimSpace(cn[1]), approvedCN)
		}
	}
	return foundLeaf && match
}
