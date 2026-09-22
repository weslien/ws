//go:build darwin

package backend

import "os"

func newAutoBackend() Backender {
	// Explicit overrides
	switch os.Getenv("WS_BACKEND") {
	case "copy":
		return NewCopyBackend()
	case "container":
		return NewContainerBackend()
	}
	// Auto-detect: use container backend if the real apple/container CLI
	// is installed (probed via 'container help' containing 'machine').
	// Falls back to CopyBackend (directory copies) otherwise.
	if hasContainerCLI() {
		return NewContainerBackend()
	}
	return NewCopyBackend()
}
