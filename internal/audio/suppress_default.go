//go:build !(linux && arm)

package audio

// suppressCOutput is a no-op on non-target platforms.
// PortAudio C library debug output is typically not an issue outside
// the embedded Linux/ARM environment.
func suppressCOutput(_ bool) func() {
	return func() {}
}
