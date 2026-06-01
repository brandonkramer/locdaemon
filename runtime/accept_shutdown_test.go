package runtime_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/brandonkramer/locdaemon/client"
	"github.com/brandonkramer/locdaemon/layout"
)

func TestRunShutdownDuringAccept(t *testing.T) {
	home, rt, stop := startDaemon(t)
	if !client.Reachable(context.Background(), home, testLayout, time.Second) {
		t.Fatal("not reachable")
	}
	rt.SignalShutdown()
	stop()
}

func TestRunAcceptErrorContinuesInForeground(t *testing.T) {
	home, rt, stop := startDaemon(t)
	defer stop()
	if err := os.Remove(layout.Socket(home, testLayout)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	rt.SignalShutdown()
	stop()
}
