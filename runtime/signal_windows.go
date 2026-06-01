//go:build windows

package runtime

import (
	"os"
	"os/signal"
)

func notifyShutdown(sig chan os.Signal) {
	signal.Notify(sig, os.Interrupt)
}
