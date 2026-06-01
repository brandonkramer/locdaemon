package locdaemon_test

import (
	"testing"

	"github.com/brandonkramer/locdaemon/layout"
)

func TestModuleSubpackages(t *testing.T) {
	t.Parallel()
	home := "/tmp/locdaemon-doc"
	ly := layout.Layout{
		SessionsDir: "sessions",
		SocketName:  "daemon.sock",
		LockName:    "daemon.lock",
		PipePrefix:  "locdaemon",
	}
	if layout.Socket(home, ly) == "" {
		t.Fatal("expected layout helper from root module consumers")
	}
}
