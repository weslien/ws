//go:build darwin

package backend

import "os"

func newAutoBackend() Backender {
	if os.Getenv("WS_BACKEND") == "copy" {
		return NewCopyBackend()
	}
	if hasContainerCLI() {
		return NewContainerBackend()
	}
	return NewCopyBackend()
}
