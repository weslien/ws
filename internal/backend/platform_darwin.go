//go:build darwin

package backend

func newAutoBackend() Backender {
	if hasContainerCLI() {
		return NewContainerBackend()
	}
	return NewCopyBackend()
}
