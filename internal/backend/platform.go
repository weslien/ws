package backend

// NewAuto picks the best backend for the current platform.
// On Linux this returns an overlayfs backend; otherwise copy backend.
func NewAuto() Backender {
	return newAutoBackend()
}
