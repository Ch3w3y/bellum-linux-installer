package core

import (
	"os/exec"
	"testing"
)

// Without a log path, output must go to a writable sink. umu-run (Python)
// exits with status 120 when it can't flush stdout, which broke the DLL
// override step on real hardware.
func TestRunCommandWithoutLogPathAcceptsOutput(t *testing.T) {
	for _, mode := range []RunMode{RunModeSilent, RunModeLog} {
		if err := RunCommand(mode, []string{"sh", "-c", "echo out && echo err >&2"}, nil, ""); err != nil {
			t.Fatalf("mode %v: writing output failed: %v", mode, err)
		}
	}
	if _, err := exec.LookPath("python3"); err == nil {
		if err := RunCommand(RunModeSilent, []string{"python3", "-c", "print('x' * 100000)"}, nil, ""); err != nil {
			t.Fatalf("python3 printing without a log path failed: %v", err)
		}
	}
}
