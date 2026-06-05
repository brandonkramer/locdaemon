// Package client dials the local daemon, invokes JSON-RPC methods, relays
// stdin/stdout to the daemon socket (ServeStdioBridge), and reads versioned
// observe-channel envelopes.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/brandonkramer/ipc"
	"github.com/brandonkramer/message"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"

	loclayout "github.com/brandonkramer/locdaemon/layout"
)

const (
	DefaultCallTimeout  = 30 * time.Second
	defaultReachTimeout = 200 * time.Millisecond
)

// AutoOptions configures CallAuto.
type AutoOptions struct {
	ResolveHome func() (string, error)
	Ensure      func(home string) error
	Layout      loclayout.Layout
	Method      string
	Params      any
	Timeout     time.Duration
}

// EnsureOptions configures Ensure.
type EnsureOptions struct {
	Home         string
	Layout       loclayout.Layout
	Binary       string
	Args         []string
	Env          []string
	Health       func(home string) bool
	StartTimeout time.Duration
	PollInterval time.Duration
	Prepare      func(home string) error
	OnReady      func(home string) error
}

// Reachable reports whether a connection to home can be opened.
func Reachable(ctx context.Context, home string, ly loclayout.Layout, timeout time.Duration) bool {
	conn, err := ipc.DialTimeout(ctx, loclayout.RPCAddr(home, ly), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Responds reports whether home is reachable and method succeeds.
func Responds(ctx context.Context, home string, ly loclayout.Layout, method string, params any, timeout time.Duration) bool {
	reach := timeout / 4
	if reach <= 0 || reach > defaultReachTimeout {
		reach = defaultReachTimeout
	}
	if !Reachable(ctx, home, ly, reach) {
		return false
	}
	_, err := CallHome[json.RawMessage](ctx, home, ly, method, params, timeout)
	return err == nil
}

// CallConn invokes method on an open connection.
func CallConn[T any](ctx context.Context, conn net.Conn, method string, params any, timeout time.Duration) (T, error) {
	var zero T
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	c := jrpc2.NewClient(channel.LSP(conn, conn), nil)
	defer c.Close()

	var result T
	if err := c.CallResult(ctx, method, params, &result); err != nil {
		return zero, err
	}
	return result, nil
}

// CallHome dials home and invokes method with timeout.
func CallHome[T any](ctx context.Context, home string, ly loclayout.Layout, method string, params any, timeout time.Duration) (T, error) {
	var zero T
	dctx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	conn, err := ipc.DialRetry(dctx, loclayout.RPCAddr(home, ly))
	if err != nil {
		return zero, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer conn.Close()
	return CallConn[T](ctx, conn, method, params, timeout)
}

// CallAuto resolves home, ensures the daemon, dials, and invokes Method.
func CallAuto[T any](ctx context.Context, opt AutoOptions) (T, error) {
	var zero T
	home, err := opt.ResolveHome()
	if err != nil {
		return zero, err
	}
	if opt.Ensure != nil {
		if err := opt.Ensure(home); err != nil {
			return zero, err
		}
	}
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = DefaultCallTimeout
	}
	dctx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	conn, err := ipc.DialRetry(dctx, loclayout.RPCAddr(home, opt.Layout))
	if err != nil {
		return zero, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer conn.Close()
	return CallConn[T](ctx, conn, opt.Method, opt.Params, timeout)
}

// Ensure starts the daemon binary when Health is false, then waits until ready.
func Ensure(ctx context.Context, opt EnsureOptions) error {
	return EnsureRunning(ctx, SpawnOptions{
		Home:         opt.Home,
		Layout:       opt.Layout,
		Binary:       opt.Binary,
		Args:         opt.Args,
		Env:          opt.Env,
		Ready:        opt.Health,
		StartTimeout: opt.StartTimeout,
		PollInterval: opt.PollInterval,
		Prepare:      opt.Prepare,
		OnReady:      opt.OnReady,
	})
}

// GetObserve decodes a versioned envelope from the observe HTTP channel.
func GetObserve(ctx context.Context, home string, ly loclayout.Layout, path string, topic message.Topic, dest any, timeout time.Duration) error {
	addr, err := loclayout.ObserveAddr(home, ly)
	if err != nil {
		return err
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	c := ipc.NewUnixHTTPClient(addr.Unix)
	if timeout > 0 {
		c.Timeout = timeout
	}
	var env message.Envelope
	if err := c.Get(ctx, path, &env); err != nil {
		return err
	}
	return env.DecodeTopic(topic, dest)
}
