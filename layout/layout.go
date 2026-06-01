// Package layout resolves daemon home paths and local IPC endpoints from svcroot layouts.
package layout

import (
	"path/filepath"

	"github.com/brandonkramer/ipc"
	"github.com/brandonkramer/svcroot"
)

type (
	// Layout names files under a daemon home directory.
	Layout = svcroot.Layout
	// Channel names a logical local IPC endpoint.
	Channel = svcroot.Channel
)

const (
	// ChannelRPC is the primary JSON-RPC control socket.
	ChannelRPC = svcroot.ChannelRPC
	// ChannelObserve is the read-only observe HTTP socket.
	ChannelObserve = svcroot.ChannelObserve
)

// Sessions returns home/sessions for layout.
func Sessions(home string, layout Layout) string {
	return svcroot.Sessions(home, &layout)
}

// Socket returns the RPC socket path for home.
func Socket(home string, layout Layout) string {
	return svcroot.Socket(home, &layout)
}

// Lock returns the daemon lock path for home.
func Lock(home string, layout Layout) string {
	return svcroot.Lock(home, &layout)
}

// ObserveSocket returns the observe socket path for home.
func ObserveSocket(home string, layout Layout) string {
	return svcroot.ObserveSocket(home, &layout)
}

// ChannelAddr resolves a logical channel name to a cross-platform endpoint.
func ChannelAddr(home string, layout Layout, channel Channel) (ipc.Addr, error) {
	unix, err := svcroot.ChannelUnix(home, &layout, channel)
	if err != nil {
		return ipc.Addr{}, err
	}
	l := layout.WithDefaults()
	return ipc.Addr{
		Unix:       unix,
		PipePrefix: l.PipePrefix,
		PipeKey:    pipeKeyForChannel(home, channel),
	}, nil
}

func pipeKeyForChannel(home string, channel Channel) string {
	if channel == ChannelRPC {
		return home
	}
	return filepath.Join(home, string(channel))
}

// RPCAddr returns the local RPC endpoint for home and layout.
func RPCAddr(home string, layout Layout) ipc.Addr {
	addr, err := ChannelAddr(home, layout, ChannelRPC)
	if err != nil {
		l := layout.WithDefaults()
		return ipc.Addr{
			Unix:       Socket(home, layout),
			PipePrefix: l.PipePrefix,
			PipeKey:    home,
		}
	}
	return addr
}

// ObserveAddr returns the observe HTTP endpoint when configured.
func ObserveAddr(home string, layout Layout) (ipc.Addr, error) {
	return ChannelAddr(home, layout, ChannelObserve)
}
