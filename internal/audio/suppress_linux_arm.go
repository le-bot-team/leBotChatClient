//go:build linux && arm

package audio

import (
	"log"
	"os"
	"sync"
	"syscall"
)

var suppressMu sync.Mutex

// suppressCOutput suppresses PortAudio/ALSA C library debug output (written
// directly to stderr fd 2) when debug mode is disabled. Go's log output is
// preserved by temporarily redirecting it to the saved original stderr fd.
//
// Returns a restore function that MUST be called after the PortAudio
// operation completes. Thread-safe: concurrent callers serialize on a mutex.
func suppressCOutput(enableDebug bool) func() {
	if enableDebug {
		return func() {}
	}

	suppressMu.Lock()

	// Save the original stderr file descriptor
	savedFd, err := syscall.Dup(syscall.Stderr)
	if err != nil {
		suppressMu.Unlock()
		return func() {}
	}

	// Create a Go file backed by the saved fd, so Go's log package
	// can continue writing to the real terminal
	savedFile := os.NewFile(uintptr(savedFd), "saved-stderr")
	log.SetOutput(savedFile)

	// Redirect fd 2 to /dev/null — C library output goes here
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		log.SetOutput(os.Stderr)
		savedFile.Close()
		suppressMu.Unlock()
		return func() {}
	}
	_ = syscall.Dup2(int(devNull.Fd()), syscall.Stderr)
	devNull.Close()

	return func() {
		// Restore fd 2 to the original stderr
		_ = syscall.Dup2(savedFd, syscall.Stderr)
		// Restore Go's log output back to os.Stderr
		log.SetOutput(os.Stderr)
		savedFile.Close()
		suppressMu.Unlock()
	}
}
