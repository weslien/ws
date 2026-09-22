//go:build !linux

package backend

func newAutoBackend() Backender {
	return NewCopyBackend()
}
