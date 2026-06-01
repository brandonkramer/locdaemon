package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/brandonkramer/ipc"
	"github.com/brandonkramer/message"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"golang.org/x/sync/errgroup"

	"github.com/brandonkramer/locdaemon/layout"
	"github.com/brandonkramer/locdaemon/observe"
)

// ErrAlreadyRunning is returned when a live daemon already owns home.
var ErrAlreadyRunning = errors.New("locdaemon: daemon already running")

// Runtime is passed to daemon hooks for shutdown coordination.
type Runtime struct {
	Home string

	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// Shutdown returns a channel closed when SignalShutdown is called.
func (rt *Runtime) Shutdown() <-chan struct{} {
	return rt.shutdown
}

// SignalShutdown closes the shutdown channel once.
func (rt *Runtime) SignalShutdown() {
	rt.shutdownOnce.Do(func() { close(rt.shutdown) })
}

// ShuttingDown reports whether shutdown was signaled.
func (rt *Runtime) ShuttingDown() bool {
	select {
	case <-rt.shutdown:
		return true
	default:
		return false
	}
}

// Config configures Run.
type Config struct {
	Home       string
	Foreground bool
	Layout     layout.Layout

	// AllowRemotePeer disables local peer credential checks when true.
	AllowRemotePeer bool
	Assigner        jrpc2.Assigner

	// GuardStart runs before taking the lock; return ErrAlreadyRunning or other error to abort.
	GuardStart func(home string) error
	// Prepare runs after GuardStart (mkdir, etc.).
	Prepare func(home string) error
	// IsLive checks whether a daemon is already serving home after lock acquisition.
	IsLive func(home string) bool
	// OnLocked runs after lock is held and home is confirmed not live.
	OnLocked func(home string) error
	// OnReady starts background work after the listener is open.
	OnReady func(rt *Runtime) error
	// OnShutdown runs when shutdown is signaled (before accept loop exits).
	OnShutdown func(rt *Runtime)
	// OnServeError reports JSON-RPC serve failures for accepted connections.
	OnServeError func(err error)
}

type daemonRuntime struct {
	cfg         Config
	releaseLock func()
	ln          net.Listener
	rt          *Runtime
	cleanupDone chan struct{}
}

// Run starts the JSON-RPC daemon and blocks until ctx is canceled or shutdown is signaled.
// Foreground keeps listening after accept errors until shutdown.
func Run(ctx context.Context, cfg Config) error {
	dr := &daemonRuntime{cfg: cfg, rt: &Runtime{Home: cfg.Home, shutdown: make(chan struct{})}}
	return dr.run(ctx)
}

func (dr *daemonRuntime) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if dr.cfg.GuardStart != nil {
		if err := dr.cfg.GuardStart(dr.cfg.Home); err != nil {
			return err
		}
	}
	if dr.cfg.Prepare != nil {
		if err := dr.cfg.Prepare(dr.cfg.Home); err != nil {
			return err
		}
	}
	release, err := AcquireHomeLock(ctx, dr.cfg.Home, dr.cfg.Layout)
	if errors.Is(err, ErrLockBusy) {
		if dr.cfg.IsLive != nil && dr.cfg.IsLive(dr.cfg.Home) {
			return fmt.Errorf("%w for %s", ErrAlreadyRunning, dr.cfg.Home)
		}
		return fmt.Errorf("daemon lock busy for %s", dr.cfg.Home)
	}
	if err != nil {
		return err
	}
	dr.releaseLock = release
	defer dr.releaseLock()

	if dr.cfg.IsLive != nil && dr.cfg.IsLive(dr.cfg.Home) {
		return fmt.Errorf("%w for %s", ErrAlreadyRunning, dr.cfg.Home)
	}
	if dr.cfg.OnLocked != nil {
		if err := dr.cfg.OnLocked(dr.cfg.Home); err != nil {
			return err
		}
	}
	if err := dr.openListener(); err != nil {
		return err
	}
	defer dr.ln.Close()

	if dr.cfg.OnReady != nil {
		if err := dr.cfg.OnReady(dr.rt); err != nil {
			return err
		}
	}
	dr.wireShutdown(ctx)
	return dr.acceptLoop(ctx)
}

// ServeConn serves one JSON-RPC connection until the client closes or the server stops.
func ServeConn(assigner jrpc2.Assigner, conn net.Conn) error {
	defer conn.Close()
	srv := jrpc2.NewServer(assigner, nil)
	s := srv.Start(channel.LSP(conn, conn))
	return s.Wait()
}

func (dr *daemonRuntime) openListener() error {
	ln, err := ipc.Listen(layout.RPCAddr(dr.cfg.Home, dr.cfg.Layout))
	if err != nil {
		return err
	}
	dr.ln = ln
	return nil
}

func (dr *daemonRuntime) wireShutdown(ctx context.Context) {
	dr.cleanupDone = make(chan struct{})
	go func() {
		defer close(dr.cleanupDone)
		<-dr.rt.shutdown
		if dr.ln != nil {
			dr.ln.Close()
		}
		if dr.cfg.OnShutdown != nil {
			dr.cfg.OnShutdown(dr.rt)
		}
	}()

	go func() {
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			sig := make(chan os.Signal, 1)
			notifyShutdown(sig)
			select {
			case <-sig:
				return nil
			case <-gctx.Done():
				return gctx.Err()
			}
		})
		_ = g.Wait()
		dr.rt.SignalShutdown()
	}()
}

func (dr *daemonRuntime) acceptLoop(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			dr.rt.SignalShutdown()
			<-dr.cleanupDone
			return err
		}
		conn, err := dr.ln.Accept()
		if err != nil {
			if dr.rt.ShuttingDown() {
				<-dr.cleanupDone
				return nil
			}
			if dr.cfg.Foreground {
				continue
			}
			return err
		}
		go dr.serveConn(conn)
	}
}

func (dr *daemonRuntime) serveConn(conn net.Conn) {
	var err error
	if dr.cfg.AllowRemotePeer {
		err = ServeConn(dr.cfg.Assigner, conn)
	} else {
		err = ServeConnLocal(dr.cfg.Assigner, conn)
	}
	if err != nil && dr.cfg.OnServeError != nil {
		dr.cfg.OnServeError(err)
	}
}

// ServeConnLocal serves JSON-RPC after verifying the peer is a local caller.
func ServeConnLocal(assigner jrpc2.Assigner, conn net.Conn) error {
	peer, err := ipc.PeerFromConn(conn)
	if err != nil {
		_ = conn.Close()
		return err
	}
	if !peer.CanWrite() {
		_ = conn.Close()
		return ipc.ErrAccessDenied
	}
	return ServeConn(assigner, conn)
}

// ObserveConfig configures the read-only observe HTTP server.
type ObserveConfig struct {
	Home   string
	Layout layout.Layout
	Mux    *http.ServeMux
}

// RunObserve serves read-only GET endpoints over the observe channel until ctx is canceled.
func RunObserve(ctx context.Context, cfg ObserveConfig) error {
	addr, err := layout.ObserveAddr(cfg.Home, cfg.Layout)
	if err != nil {
		return err
	}
	mux := cfg.Mux
	if mux == nil {
		mux = http.NewServeMux()
		mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
			message.HandleGET(w, r, observe.TopicReady, func() (any, error) {
				return map[string]string{"status": "ok"}, nil
			})
		})
	}
	return ipc.RunUnixHTTP(ctx, addr.Unix, true, mux)
}
