package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/brandonkramer/ipc"
	"github.com/brandonkramer/poll"

	loclayout "github.com/brandonkramer/locdaemon/layout"
)

const (
	DefaultStartTimeout = 5 * time.Second
	DefaultPollInterval = 100 * time.Millisecond
)

// ErrStartTimeout is returned when the daemon does not become ready before StartTimeout elapses.
var ErrStartTimeout = errors.New("locdaemon: daemon start timeout")

// SpawnOptions configures EnsureRunning.
type SpawnOptions struct {
	Home         string
	Layout       loclayout.Layout
	Binary       string
	Args         []string
	Env          []string
	Ready        func(home string) bool
	StartTimeout time.Duration
	PollInterval time.Duration
	Prepare      func(home string) error
	OnReady      func(home string) error
}

// EnsureRunning starts binary when Ready is false, then waits until Ready or timeout.
func EnsureRunning(ctx context.Context, opt SpawnOptions) error {
	if opt.Ready != nil && opt.Ready(opt.Home) {
		if opt.OnReady != nil {
			return opt.OnReady(opt.Home)
		}
		return nil
	}
	if opt.Prepare != nil {
		if err := opt.Prepare(opt.Home); err != nil {
			return err
		}
	}
	binary := opt.Binary
	if binary == "" {
		binary = os.Args[0]
	}
	startTimeout := opt.StartTimeout
	if startTimeout <= 0 {
		startTimeout = DefaultStartTimeout
	}
	pollInterval := opt.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	cmd := exec.CommandContext(ctx, binary, opt.Args...) //nolint:gosec // binary and args are caller-controlled daemon startup
	ipc.SetDetach(cmd)
	if len(opt.Env) > 0 {
		cmd.Env = append(os.Environ(), opt.Env...)
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	err := poll.Wait(ctx, func(context.Context) (bool, error) {
		if opt.Ready != nil && opt.Ready(opt.Home) {
			if opt.OnReady != nil {
				return true, opt.OnReady(opt.Home)
			}
			return true, nil
		}
		return false, nil
	}, poll.WithTimeout(startTimeout), poll.WithInterval(pollInterval))
	if err == nil {
		return nil
	}
	if errors.Is(err, poll.ErrTimeout) {
		return fmt.Errorf("%w after %s", ErrStartTimeout, startTimeout)
	}
	return err
}
