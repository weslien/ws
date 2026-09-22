package backend

import (
	"os/exec"
	"runtime"
)

// hasContainerCLI returns true if the 'container' binary from
// https://github.com/apple/container is available in PATH.
func hasContainerCLI() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("container")
	return err == nil
}
