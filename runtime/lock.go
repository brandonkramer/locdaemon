package runtime

import (
	"context"

	"github.com/brandonkramer/filelock"

	"github.com/brandonkramer/locdaemon/layout"
)

// ErrLockBusy is returned when another process holds the daemon lock.
var ErrLockBusy = filelock.ErrBusy

// AcquireHomeLock takes an exclusive non-blocking lock for home until release is called.
func AcquireHomeLock(ctx context.Context, home string, ly layout.Layout) (release func(), err error) {
	return filelock.Acquire(ctx, layout.Lock(home, ly), filelock.WritePID())
}

// WithSidecarLock runs fn while holding an exclusive lock on path+".lock".
func WithSidecarLock(ctx context.Context, path string, fn func() error) error {
	return filelock.WithSidecar(ctx, path, filelock.DefaultSidecar, fn)
}
