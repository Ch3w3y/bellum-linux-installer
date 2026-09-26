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
	Workdir        string
	ProtonVer      string
	ProtonBaseURL  string
	ProtonSHA256   string
	LauncherSHA256 string
	LauncherSigner string
	WineVer        string
	WinetricksVer  string
	DXVKVer        string
	VKD3DVer       string
	Binaries       Binaries
}

// DefaultVersions contains the version configuration
var DefaultVersions = Versions{
	Workdir:       ".",
	ProtonVer:     "proton-cachyos-10.0-20260424-slr-x86_64",
	ProtonBaseURL: "https://github.com/CachyOS/proton-cachyos/releases/download",
	// These release-specific pins must be populated from the approved release manifest.
	// Empty pins deliberately block downloads and launcher execution.
	ProtonSHA256:   "",
	LauncherSHA256: "",
	LauncherSigner: "",
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
