package layout_test

import (
	"strings"
	"testing"

	"github.com/brandonkramer/locdaemon/layout"
)

var testLayout = layout.Layout{
	SessionsDir:       "sessions",
	SocketName:        "daemon.sock",
	ObserveSocketName: "observe.sock",
	LockName:          "daemon.lock",
	PipePrefix:        "locdaemon",
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"sessions", layout.Sessions("/home", testLayout), "/home/sessions"},
		{"socket", layout.Socket("/home", testLayout), "/home/sessions/daemon.sock"},
		{"lock", layout.Lock("/home", testLayout), "/home/sessions/daemon.lock"},
		{"observe", layout.ObserveSocket("/home", testLayout), "/home/sessions/observe.sock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Fatalf("got=%q want=%q", tc.got, tc.want)
			}
		})
	}
}

func TestChannelAddr(t *testing.T) {
	t.Parallel()
	home := "/tmp/locdaemon-home"
	rpc, err := layout.ChannelAddr(home, testLayout, layout.ChannelRPC)
	if err != nil || rpc.Unix == "" || rpc.PipePrefix != "locdaemon" || rpc.PipeKey != home {
		t.Fatalf("rpc=%+v err=%v", rpc, err)
	}
	observe, err := layout.ChannelAddr(home, testLayout, layout.ChannelObserve)
	if err != nil || !strings.Contains(observe.Unix, "observe.sock") {
		t.Fatalf("observe=%+v err=%v", observe, err)
	}
	if observe.PipeKey == home {
		t.Fatal("expected observe pipe key scoped to channel")
	}
}

func TestRPCAddrAndObserveAddr(t *testing.T) {
	t.Parallel()
	home := "/tmp/locdaemon-home"
	rpc := layout.RPCAddr(home, testLayout)
	if rpc.Unix == "" || rpc.PipeKey != home {
		t.Fatalf("rpc=%+v", rpc)
	}
	observe, err := layout.ObserveAddr(home, testLayout)
	if err != nil || observe.Unix == "" {
		t.Fatalf("observe=%+v err=%v", observe, err)
	}
}
