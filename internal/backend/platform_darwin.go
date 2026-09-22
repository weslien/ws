//go:build darwin

package backend

import "os"

func newAutoBackend() Backender {
	// ContainerBackend is opt-in only. The macOS system 'container' binary
	// is a different tool, and the real apple/container CLI is still
	// experimental. Set WS_BACKEND=container to enable it.
	if os.Getenv("WS_BACKEND") == "container" {
		return NewContainerBackend()
	}
	return NewCopyBackend()
}
