package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/brandonkramer/ipc"
	"github.com/creachadair/jrpc2/channel"
	"golang.org/x/sync/errgroup"

	loclayout "github.com/brandonkramer/locdaemon/layout"
)

// StdioBridgeOptions configures ServeStdioBridge.
type StdioBridgeOptions struct {
	ResolveHome func() (string, error)
	Layout      loclayout.Layout
	Stdin       io.Reader
	Stdout      io.Writer
}

// RelayFrames copies framed JSON-RPC records from src to dst until Recv returns EOF or an error.
func RelayFrames(dst, src channel.Channel) error {
	for {
		msg, err := src.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := dst.Send(msg); err != nil {
			return err
		}
	}
}

// NewStdioConn returns a net.Conn over r/w for LSP-framed JSON-RPC (e.g. SSH stdio pipes).
func NewStdioConn(r io.Reader, w io.Writer) net.Conn {
	return &stdioConn{r: r, w: w}
}

// ServeStdioBridge forwards JSON-RPC between stdin/stdout and the local daemon socket.
func ServeStdioBridge(ctx context.Context, opt StdioBridgeOptions) error {
	home, err := opt.ResolveHome()
	if err != nil {
		return err
	}
	conn, err := ipc.DialRetry(ctx, loclayout.RPCAddr(home, opt.Layout))
	if err != nil {
		return fmt.Errorf("daemon not reachable: %w", err)
	}
	defer conn.Close()

	stdin := opt.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opt.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stdio := &stdioConn{r: stdin, w: stdout}
	stdCh := channel.LSP(stdio, nopCloser{Writer: stdio.w})
	sockCh := channel.LSP(conn, conn)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		select {
		case <-gctx.Done():
			return gctx.Err()
		default:
		}
		return RelayFrames(sockCh, stdCh)
	})
	g.Go(func() error {
		select {
		case <-gctx.Done():
			return gctx.Err()
		default:
		}
		return RelayFrames(stdCh, sockCh)
	})
	return g.Wait()
}

type stdioConn struct {
	r io.Reader
	w io.Writer
}

func (c *stdioConn) Read(p []byte) (int, error)         { return c.r.Read(p) }
func (c *stdioConn) Write(p []byte) (int, error)        { return c.w.Write(p) }
func (c *stdioConn) Close() error                       { return nil }
func (c *stdioConn) LocalAddr() net.Addr                { return pipeAddr{"stdio"} }
func (c *stdioConn) RemoteAddr() net.Addr               { return pipeAddr{"stdio"} }
func (c *stdioConn) SetDeadline(t time.Time) error      { return nil }
func (c *stdioConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *stdioConn) SetWriteDeadline(t time.Time) error { return nil }

type pipeAddr struct{ name string }

func (a pipeAddr) Network() string { return a.name }
func (a pipeAddr) String() string  { return a.name }

type nopCloser struct{ io.Writer }

func (n nopCloser) Close() error { return nil }
