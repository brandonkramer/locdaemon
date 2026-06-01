package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandonkramer/locdaemon/runtime"
)

func TestAcquireHomeLockExclusive(t *testing.T) {
	home := t.TempDir()
	release, err := runtime.AcquireHomeLock(context.Background(), home, testLayout)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = runtime.AcquireHomeLock(context.Background(), home, testLayout)
	if err != runtime.ErrLockBusy {
		t.Fatalf("err=%v want %v", err, runtime.ErrLockBusy)
	}
}

func TestWithSidecarLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	called := false
	if err := runtime.WithSidecarLock(context.Background(), path, func() error {
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

func TestAcquireHomeLockMkdirError(t *testing.T) {
	home := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(home, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.AcquireHomeLock(context.Background(), home, testLayout); err == nil {
		t.Fatal("expected acquire error")
	}
}

func TestAcquireHomeLockAndSidecarErrors(t *testing.T) {
	home := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(home, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.AcquireHomeLock(context.Background(), home, testLayout); err == nil {
		t.Fatal("expected mkdir error")
	}
	if err := runtime.WithSidecarLock(context.Background(), "/dev/null/impossible/state", func() error { return nil }); err == nil {
		t.Fatal("expected sidecar lock error")
	}
}

func TestAcquireHomeLockWritePID(t *testing.T) {
	home := t.TempDir()
	release, err := runtime.AcquireHomeLock(context.Background(), home, testLayout)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
