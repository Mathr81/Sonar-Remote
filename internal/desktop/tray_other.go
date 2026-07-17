//go:build !windows

// Package desktop's tray/flyout experience is Windows-only. On other platforms
// Run simply blocks until shutdown is invoked elsewhere, so the binary still
// builds and behaves as a plain headless server.
package desktop

import (
	"os"
	"os/signal"
	"syscall"
)

// Run blocks until an interrupt/terminate signal, then calls shutdown. The tray
// UI itself is not available off Windows.
func Run(_ string, _ int, shutdown func()) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	if shutdown != nil {
		shutdown()
	}
	return nil
}
