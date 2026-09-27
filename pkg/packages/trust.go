package packages

import (
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"path"
	"strings"
)

//go:embed trust
var trustFS embed.FS

// pinnedRoots maps each embedded trust anchor to its SHA-256 fingerprint
// (of the DER certificate). A file that is missing here, or whose fingerprint
// differs, is rejected.
var pinnedRoots = map[string]string{
	// GlobalSign Code Signing Root R45; issues the Astarte Launcher's EV
	// code-signing CA (GlobalSign GCC R45 EV CodeSigning CA 2020).
	"globalsign-code-signing-root-r45.pem": "7b9d553e1c92cb6e8803e137f4f287d4363757f5d44b37d52f9fca22fb97df86",
}

// embeddedRoots returns the pinned code-signing roots shipped with the
// installer (see trust/README.md).
func embeddedRoots() ([]*x509.Certificate, error) {
	entries, err := trustFS.ReadDir("trust")
	if err != nil {
		return nil, err
	}
	var roots []*x509.Certificate
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		data, err := trustFS.ReadFile(path.Join("trust", e.Name()))
		if err != nil {
			return nil, err
		}
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("trust/%s is not a PEM certificate", e.Name())
		}
		sum := sha256.Sum256(block.Bytes)
		if want, ok := pinnedRoots[e.Name()]; !ok || !strings.EqualFold(want, hex.EncodeToString(sum[:])) {
			return nil, fmt.Errorf("trust/%s does not match its pinned fingerprint", e.Name())
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		roots = append(roots, cert)
	}
	return roots, nil
}

// launcherRoots is the system trust store plus the embedded code-signing roots.
func launcherRoots() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	extra, err := embeddedRoots()
	if err != nil {
		return nil, err
	}
	for _, c := range extra {
		pool.AddCert(c)
	}
	return pool, nil
}
