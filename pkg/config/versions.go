package config

// Binaries holds the paths to required binaries
type Binaries struct {
	Wine       string
	Wineboot   string
	Msidb      string
	Winecfg    string
	Wineserver string
}

// Versions holds all version strings and paths for the installer
type Versions struct {
	Workdir          string
	ProtonVer        string
	ProtonBaseURL    string
	ProtonSHA256     string
	EACRuntimeSHA256 string
	LauncherSHA256   string
	LauncherSigner   string
	WineVer          string
	WinetricksVer    string
	DXVKVer          string
	VKD3DVer         string
	Binaries         Binaries
}

// DefaultVersions contains the version configuration
var DefaultVersions = Versions{
	Workdir:       ".",
	ProtonVer:     "proton-cachyos-11.0-20260703-slr-x86_64",
	ProtonBaseURL: "https://github.com/CachyOS/proton-cachyos/releases/download",
	// SHA-256 from the official CachyOS GitHub release asset metadata.
	ProtonSHA256: "62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b",
	// Steam app 1826330, depot 1826331, public manifest 3310269496439035229
	// (build 10437216). Anonymous SteamCMD cannot access this depot; the
	// runtime hash must be measured from an authenticated Valve installation.
	EACRuntimeSHA256: "",
	// Official Astarte updater download inspected on 2026-09-26.
	LauncherSHA256: "2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a",
	LauncherSigner: "ASTARTE INDUSTRIES INC.",
	WineVer:        "wine-11.8",
	WinetricksVer:  "20250102-modified",
	DXVKVer:        "2.7.1-3-521-low-latency",
	VKD3DVer:       "2.14",
	Binaries: Binaries{
		Wine:       "/usr/bin/wine",
		Wineboot:   "/usr/bin/wineboot",
		Msidb:      "/usr/bin/msidb",
		Winecfg:    "/usr/bin/winecfg",
		Wineserver: "/usr/bin/wineserver",
	},
}
