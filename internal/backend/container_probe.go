package backend

import (
	"os/exec"
	"runtime"
	"strings"
)

// hasContainerCLI returns true if the 'container' binary from
// https://github.com/apple/container is available in PATH.
// On macOS /usr/bin/container exists but is NOT the right tool,
// so we verify by checking the help output for the "machine" command.
func hasContainerCLI() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	path, err := exec.LookPath("container")
	if err != nil {
		return false
	}
	out, err := exec.Command(path, "help").CombinedOutput()
	return err == nil && strings.Contains(string(out), "machine")
}
