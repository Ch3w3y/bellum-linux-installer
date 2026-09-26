package packages

import (
	"fmt"
	"os/exec"
	"strings"

	"bellum-installer/pkg/config"
)

// VerifyLauncherAuthenticode requires a valid signature and the approved signer.
func VerifyLauncherAuthenticode(path string) error {
	signer := config.DefaultVersions.LauncherSigner
	if signer == "" {
		return fmt.Errorf("missing AstarteLauncher Authenticode signer pin")
	}
	output, err := exec.Command("osslsigncode", "verify", "-in", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("AstarteLauncher Authenticode verification failed: %w", err)
	}
	if !strings.Contains(string(output), signer) {
		return fmt.Errorf("AstarteLauncher signer does not match approved signer")
	}
	return nil
}

func VerifyLauncherInstaller(path string) error {
	if config.DefaultVersions.LauncherSHA256 != "" {
		if err := VerifySHA256(path, config.DefaultVersions.LauncherSHA256); err != nil {
			return err
		}
	}
	return VerifyLauncherAuthenticode(path)
}
