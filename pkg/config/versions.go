package config

// InstallerVersion is stamped at build time with -ldflags "-X
// bellum-installer/pkg/config.InstallerVersion=<version>" (see Makefile).
var InstallerVersion = "dev"

// Versions holds all version strings and paths for the installer
type Versions struct {
	Workdir                   string
	ProtonVer                 string
	ProtonBaseURL             string
	ProtonSHA256              string
	UMUVersion                string
	UMUZipappSHA256           string
	EACRuntimeSHA256Allowlist []string
	LauncherSHA256Allowlist   []string
	LauncherSigner            string
	WinetricksVer             string
	DXVKVer                   string
	VKD3DVer                  string
	DXVKNVAPIVer              string
}

// DefaultVersions contains the version configuration
var DefaultVersions = Versions{
	Workdir:       ".",
	ProtonVer:     "proton-cachyos-11.0-20261005-slr-x86_64",
	ProtonBaseURL: "https://github.com/CachyOS/proton-cachyos/releases/download",
	// SHA-256 from the official CachyOS GitHub release asset metadata.
	ProtonSHA256: "096bfe73b506d6565b04ecc45214197a4091818f16ed91f5324b4d2082d0a263",
	// umu-launcher's self-contained zipapp (needs only python3), so users
	// don't have to find a distro package for it.
	UMUVersion:      "1.4.4",
	UMUZipappSHA256: "eb590691841f7fad3fc3ad8fd5db4ccb87849fe7948e62b28ece7a4ee48cc851",
	// Measured from the entitled Steam client install of app 1826330,
	// depot 1826331, manifest 3310269496439035229 (build 10437216).
	// Digest covers the six paths in packages.eacRuntimeFiles.
	EACRuntimeSHA256Allowlist: []string{"4d18c3a5b896c757be9e25bf1004b81568bc4d4e56ddd8d1a2a634eebf12d1f9"},
	// Official Astarte updater download inspected on 2026-09-26.
	LauncherSHA256Allowlist: []string{"2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a"},
	LauncherSigner:          "ASTARTE INDUSTRIES INC.",
	// winetricks is the copy shipped in the pinned Proton archive's
	// protonfixes directory; run through `umu-run winetricks`.
	WinetricksVer: "bundled with pinned Proton (20260125-next)",
	// The installer uses Proton's integrated components instead of independently
	// pinned DLL overlays. The Proton archive hash is the reproducible content pin.
	DXVKVer:      "integrated with pinned Proton",
	VKD3DVer:     "integrated with pinned Proton",
	DXVKNVAPIVer: "integrated with pinned Proton",
}
