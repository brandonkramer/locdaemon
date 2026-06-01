package client_test

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/creachadair/jrpc2/handler"

	"github.com/brandonkramer/locdaemon/client"
	"github.com/brandonkramer/locdaemon/layout"
	"github.com/brandonkramer/locdaemon/observe"
	"github.com/brandonkramer/locdaemon/runtime"
)

var testLayout = layout.Layout{
	SessionsDir:       "sessions",
	SocketName:        "daemon.sock",
	ObserveSocketName: "observe.sock",
	LockName:          "daemon.lock",
	PipePrefix:        "locdaemon",
}

func testHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "agentd-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	return home
}

func startDaemon(t *testing.T) (home string, stop func()) {
	t.Helper()
	home = testHome(t)
	ready := make(chan *runtime.Runtime, 1)
	done := make(chan struct{})
	assigner := handler.Map{
		"add": handler.New(func(_ context.Context, args struct{ A, B int }) (int, error) {
			return args.A + args.B, nil
		}),
		"status": handler.New(func(_ context.Context) (string, error) { return "ok", nil }),
	}
	go func() {
		defer close(done)
		_ = runtime.Run(context.Background(), runtime.Config{
			Home:       home,
			Layout:     testLayout,
			Foreground: true,
			Assigner:   assigner,
			Prepare: func(home string) error {
				return os.MkdirAll(layout.Sessions(home, testLayout), 0o755)
			},
			OnReady: func(rt *runtime.Runtime) error {
				ready <- rt
				return nil
			},
		})
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if client.Reachable(context.Background(), home, testLayout, 50*time.Millisecond) {
			rt := <-ready
			return home, func() {
				rt.SignalShutdown()
				<-done
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon not ready")
	return "", nil
}

func TestReachableAndCallHome(t *testing.T) {
	t.Parallel()
	home, stop := startDaemon(t)
	defer stop()
	ctx := context.Background()
	if !client.Reachable(ctx, home, testLayout, time.Second) {
		t.Fatal("not reachable")
	}
	got, err := client.CallHome[int](ctx, home, testLayout, "add", map[string]int{"A": 2, "B": 3}, time.Second)
	if err != nil || got != 5 {
		t.Fatalf("got=%d err=%v", got, err)
	}
}

func TestCallConnViaPipe(t *testing.T) {
	t.Parallel()
	srv, cli := net.Pipe()
	defer cli.Close()
	assigner := handler.Map{
		"echo": handler.New(func(_ context.Context) (string, error) { return "hi", nil }),
	}
	go func() { _ = runtime.ServeConn(assigner, srv) }()

	got, err := client.CallConn[string](context.Background(), cli, "echo", nil, time.Second)
	if err != nil || got != "hi" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestResponds(t *testing.T) {
	t.Parallel()
	home, stop := startDaemon(t)
	defer stop()
	if !client.Responds(context.Background(), home, testLayout, "status", nil, time.Second) {
		t.Fatal("expected response")
	}
}

func TestCallAuto(t *testing.T) {
	t.Parallel()
	home, stop := startDaemon(t)
	defer stop()
	got, err := client.CallAuto[string](context.Background(), client.AutoOptions{
		ResolveHome: func() (string, error) { return home, nil },
		Layout:      testLayout,
		Method:      "status",
		Timeout:     time.Second,
	})
	if err != nil || got != "ok" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestEnsureAlreadyHealthy(t *testing.T) {
	t.Parallel()
	ready := false
	err := client.Ensure(context.Background(), client.EnsureOptions{
		Home:   "/tmp/example",
		Health: func(string) bool { return true },
		OnReady: func(string) error {
			ready = true
			return nil
		},
	})
	if err != nil || !ready {
		t.Fatalf("err=%v ready=%v", err, ready)
	}
}

func TestCallConnTimeout(t *testing.T) {
	t.Parallel()
	srv, cli := net.Pipe()
	defer cli.Close()
	go func() { _ = runtime.ServeConn(handler.Map{}, srv) }()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	_, err := client.CallConn[string](ctx, cli, "nope", nil, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCallHomeUnreachable(t *testing.T) {
	t.Parallel()
	_, err := client.CallHome[string](context.Background(), "/tmp/agentd-no-such-home-xyz", testLayout, "x", nil, time.Millisecond)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRespondsFailures(t *testing.T) {
	t.Parallel()
	if client.Responds(context.Background(), "/tmp/agentd-no-such-home-xyz", testLayout, "x", nil, time.Millisecond) {
		t.Fatal("expected false")
	}
	home, stop := startDaemon(t)
	defer stop()
	if client.Responds(context.Background(), home, testLayout, "missing", nil, time.Second) {
		t.Fatal("expected method failure")
	}
}

func TestCallAutoPaths(t *testing.T) {
	t.Parallel()
	_, err := client.CallAuto[string](context.Background(), client.AutoOptions{
		ResolveHome: func() (string, error) { return "", errors.New("resolve") },
	})
	if err == nil {
		t.Fatal("expected resolve error")
	}
	home, stop := startDaemon(t)
	defer stop()
	_, err = client.CallAuto[string](context.Background(), client.AutoOptions{
		ResolveHome: func() (string, error) { return home, nil },
		Ensure:      func(string) error { return errors.New("ensure") },
	})
	if err == nil {
		t.Fatal("expected ensure error")
	}
	got, err := client.CallAuto[string](context.Background(), client.AutoOptions{
		ResolveHome: func() (string, error) { return home, nil },
		Layout:      testLayout,
		Method:      "status",
	})
	if err != nil || got != "ok" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestEnsureRunningBranches(t *testing.T) {
	t.Parallel()
	readyCalled := false
	home := testHome(t)
	if err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home: home, Ready: func(string) bool { return true },
		OnReady: func(string) error { readyCalled = true; return nil },
	}); err != nil || !readyCalled {
		t.Fatalf("err=%v ready=%v", err, readyCalled)
	}
	if err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home: home, Prepare: func(string) error { return errors.New("prep") },
		Ready: func(string) bool { return false }, Binary: os.Args[0],
	}); err == nil {
		t.Fatal("expected prepare error")
	}
	if err := client.EnsureRunning(context.Background(), client.SpawnOptions{
		Home: home, Ready: func(string) bool { return false }, Binary: "/no/such/binary",
		StartTimeout: 20 * time.Millisecond, PollInterval: 5 * time.Millisecond,
	}); err == nil {
		t.Fatal("expected start error")
	}
}

func TestGetObserve(t *testing.T) {
	t.Parallel()
	home, err := os.MkdirTemp("/tmp", "agentd-observe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	if err := os.MkdirAll(layout.Sessions(home, testLayout), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = runtime.RunObserve(ctx, runtime.ObserveConfig{Home: home, Layout: testLayout})
	}()

	deadline := time.Now().Add(2 * time.Second)
	var payload map[string]string
	for time.Now().Before(deadline) {
		err := client.GetObserve(context.Background(), home, testLayout, "/ready", observe.TopicReady, &payload, time.Second)
		if err == nil && payload["status"] == "ok" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("observe /ready not reachable")
}
