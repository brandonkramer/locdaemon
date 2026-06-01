package runtime

import (
	"net"
	"testing"
	"time"
)

func TestRuntimeShutdownHooks(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Home: "x", shutdown: make(chan struct{})}
	if rt.ShuttingDown() {
		t.Fatal("not shutting down yet")
	}
	rt.SignalShutdown()
	select {
	case <-rt.Shutdown():
	default:
		t.Fatal("expected shutdown channel closed")
	}
	if !rt.ShuttingDown() {
		t.Fatal("expected shutting down")
	}
	rt.SignalShutdown()
}

func TestServeConnOnServeError(t *testing.T) {
	t.Parallel()
	errCh := make(chan error, 1)
	dr := &daemonRuntime{
		cfg: Config{
			OnServeError: func(err error) { errCh <- err },
		},
	}
	srv, cli := net.Pipe()
	go dr.serveConn(srv)
	_ = cli.Close()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected serve error")
		}
	case <-time.After(time.Second):
		t.Fatal("OnServeError not called")
	}
}
