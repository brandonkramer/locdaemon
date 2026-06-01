package runtime_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brandonkramer/ipc"
	"github.com/brandonkramer/poll"
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

func testAssigner() handler.Map {
	return handler.Map{
		"ping": handler.New(func(_ context.Context) (string, error) { return "pong", nil }),
	}
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

func startDaemon(t *testing.T) (home string, rt *runtime.Runtime, stop func()) {
	t.Helper()
	home = testHome(t)
	ready := make(chan *runtime.Runtime, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runtime.Run(context.Background(), runtime.Config{
			Home:       home,
			Layout:     testLayout,
			Foreground: true,
			Assigner:   testAssigner(),
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
			select {
			case rt = <-ready:
			case <-time.After(time.Second):
				t.Fatal("daemon reachable but OnReady not called")
			}
			return home, rt, func() {
				rt.SignalShutdown()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("daemon did not stop")
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon did not become reachable")
	return "", nil, nil
}

func TestRunServesJSONRPC(t *testing.T) {
	home, _, stop := startDaemon(t)
	defer stop()

	ctx := context.Background()
	got, err := client.CallHome[string](ctx, home, testLayout, "ping", nil, time.Second)
	if err != nil || got != "pong" {
		t.Fatalf("CallHome=%q err=%v", got, err)
	}
}

func TestRunErrAlreadyRunning(t *testing.T) {
	home, _, stop := startDaemon(t)
	defer stop()

	err := runtime.Run(context.Background(), runtime.Config{
		Home:     home,
		Layout:   testLayout,
		Assigner: testAssigner(),
		IsLive:   func(string) bool { return true },
		Prepare: func(home string) error {
			return os.MkdirAll(layout.Sessions(home, testLayout), 0o755)
		},
	})
	if !errors.Is(err, runtime.ErrAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}
}

func TestRunGuardStartAndPrepareErrors(t *testing.T) {
	t.Parallel()
	home, err := os.MkdirTemp("/tmp", "agentd-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	want := errors.New("blocked")
	err = runtime.Run(context.Background(), runtime.Config{
		Home:       home,
		Layout:     testLayout,
		GuardStart: func(string) error { return want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("guard err=%v", err)
	}

	err = runtime.Run(context.Background(), runtime.Config{
		Home:    home,
		Layout:  testLayout,
		Prepare: func(string) error { return want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("prepare err=%v", err)
	}
}

func TestServeConn(t *testing.T) {
	home, _, stop := startDaemon(t)
	defer stop()

	if !client.Responds(context.Background(), home, testLayout, "ping", nil, time.Second) {
		t.Fatal("expected daemon to respond")
	}
}

func TestRunOnShutdownHook(t *testing.T) {
	home := testHome(t)
	var called atomic.Bool
	ready := make(chan *runtime.Runtime, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runtime.Run(context.Background(), runtime.Config{
			Home:       home,
			Layout:     testLayout,
			Foreground: true,
			Assigner:   testAssigner(),
			Prepare: func(home string) error {
				return os.MkdirAll(layout.Sessions(home, testLayout), 0o755)
			},
			OnReady: func(rt *runtime.Runtime) error {
				ready <- rt
				return nil
			},
			OnShutdown: func(*runtime.Runtime) { called.Store(true) },
		})
	}()

	select {
	case rt := <-ready:
		rt.SignalShutdown()
	case <-time.After(3 * time.Second):
		t.Fatal("daemon not ready")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("run did not exit")
	}
	if !called.Load() {
		t.Fatal("OnShutdown not called")
	}
}

func TestRunErrorPaths(t *testing.T) {
	home := testHome(t)
	release, err := runtime.AcquireHomeLock(context.Background(), home, testLayout)
	if err != nil {
		t.Fatal(err)
	}
	err = runtime.Run(context.Background(), runtime.Config{Home: home, Layout: testLayout, Assigner: testAssigner()})
	if err == nil {
		t.Fatal("expected lock busy error")
	}
	release()

	err = runtime.Run(context.Background(), runtime.Config{
		Home: home, Layout: testLayout, Assigner: testAssigner(),
		IsLive: func(string) bool { return true },
	})
	if !errors.Is(err, runtime.ErrAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}

	want := errors.New("onlocked")
	err = runtime.Run(context.Background(), runtime.Config{
		Home: testHome(t), Layout: testLayout, Assigner: testAssigner(),
		Prepare:  func(h string) error { return os.MkdirAll(layout.Sessions(h, testLayout), 0o755) },
		OnLocked: func(string) error { return want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("onlocked err=%v", err)
	}

	home = testHome(t)
	err = runtime.Run(context.Background(), runtime.Config{
		Home: home, Layout: testLayout, Foreground: true, Assigner: testAssigner(),
		Prepare: func(h string) error { return os.MkdirAll(layout.Sessions(h, testLayout), 0o755) },
		OnReady: func(*runtime.Runtime) error { return errors.New("ready fail") },
	})
	if err == nil {
		t.Fatal("expected onready error")
	}
}

func TestRunAcceptLoop(t *testing.T) {
	home, rt, stop := startDaemon(t)
	defer stop()
	if !client.Responds(context.Background(), home, testLayout, "ping", nil, time.Second) {
		t.Fatal("expected response")
	}
	rt.SignalShutdown()
	stop()
}

func TestSeqFetchErrors(t *testing.T) {
	t.Parallel()
	want := errors.New("fetch")
	_, err := poll.Seq(context.Background(), 0, 100, func() int { return 1 }, func() (string, error) {
		return "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	_, err = poll.Seq(context.Background(), 1, 500, func() int { return 1 }, func() (string, error) {
		return "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("seq err=%v", err)
	}
	_, err = poll.Until(context.Background(), 100, func() (int, error) { return 0, want }, func(int) bool { return false })
	if !errors.Is(err, want) {
		t.Fatalf("until err=%v", err)
	}
}

func TestServeConnLocal(t *testing.T) {
	home := testHome(t)
	if err := os.MkdirAll(layout.Sessions(home, testLayout), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := ipc.Listen(layout.RPCAddr(home, testLayout))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = runtime.ServeConnLocal(testAssigner(), conn)
	}()

	conn, err := ipc.Dial(context.Background(), layout.RPCAddr(home, testLayout))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	got, err := client.CallConn[string](context.Background(), conn, "ping", nil, time.Second)
	if err != nil || got != "pong" {
		t.Fatalf("CallConn=%q err=%v", got, err)
	}
}

func TestRunContextCancel(t *testing.T) {
	t.Parallel()
	home := testHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runtime.Run(ctx, runtime.Config{
		Home:     home,
		Layout:   testLayout,
		Assigner: testAssigner(),
		Prepare:  func(h string) error { return os.MkdirAll(layout.Sessions(h, testLayout), 0o755) },
	})
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestRunObserveDefaultMux(t *testing.T) {
	t.Parallel()
	home := testHome(t)
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
