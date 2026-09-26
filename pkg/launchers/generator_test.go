package launchers

import (
	"strings"
	"testing"
)

func TestAllVendorsUseUMUAndEAC(t *testing.T) {
	for _, vendor := range []string{"AMD", "NVIDIA", "Intel"} {
		script := generateWrapperContent(LauncherConfig{Wineprefix: "/tmp/Bellum's prefix", Protonpath: "/proton", GPUType: vendor})
		if !strings.Contains(script, "exec umu-run") || !strings.Contains(script, "PROTON_EAC_RUNTIME") {
			t.Fatalf("%s wrapper does not require umu and EAC", vendor)
		}
		if strings.Contains(script, "wineboot") || strings.Contains(script, "\nwine ") || strings.Contains(script, "cd \"$GAME_DIR\"") {
			t.Fatalf("%s wrapper uses legacy Wine/game path", vendor)
		}
		if !strings.Contains(script, `LAUNCH_VARS='/tmp/Bellum'"'"'s prefix/launch_vars.env'`) {
			t.Fatalf("%s wrapper fails to quote paths", vendor)
		}
	}
}
