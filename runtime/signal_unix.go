//go:build unix

package runtime

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyShutdown(sig chan os.Signal) {
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
}
