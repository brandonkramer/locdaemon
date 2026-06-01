package client_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brandonkramer/locdaemon/client"
)

func TestEnsureRunningWaitsForReady(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(home, "ready")
	err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home:         home,
		Layout:       testLayout,
		Binary:       "/bin/sh",
		Args:         []string{"-c", "sleep 0.05; touch " + marker},
		StartTimeout: 3 * time.Second,
		PollInterval: 10 * time.Millisecond,
		Ready:        func(string) bool { _, err := os.Stat(marker); return err == nil },
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRunningContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := client.EnsureRunning(ctx, client.SpawnOptions{
		Home:   t.TempDir(),
		Ready:  func(string) bool { return false },
		Binary: "/bin/sleep",
		Args:   []string{"10"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureRunningDefaultsAndTimeout(t *testing.T) {
	home := testHome(t)
	marker := filepath.Join(home, "ready")
	err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home:         home,
		Args:         []string{"-c", "touch " + marker},
		Binary:       "/bin/sh",
		Ready:        func(string) bool { _, err := os.Stat(marker); return err == nil },
		StartTimeout: 3 * time.Second,
		PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home: home, Binary: "/bin/true", Ready: func(string) bool { return false },
		StartTimeout: 30 * time.Millisecond, PollInterval: 5 * time.Millisecond,
	})
	if err == nil || err.Error() == "" {
		t.Fatalf("err=%v", err)
	}
}

func TestEnsureRunningOnReadyAfterSpawn(t *testing.T) {
	home := testHome(t)
	marker := filepath.Join(home, "ready2")
	called := false
	err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home: home, Binary: "/bin/sh", Args: []string{"-c", "touch " + marker},
		Ready:        func(string) bool { _, err := os.Stat(marker); return err == nil },
		OnReady:      func(string) error { called = true; return nil },
		StartTimeout: 3 * time.Second, PollInterval: 5 * time.Millisecond,
	})
	if err != nil || !called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
