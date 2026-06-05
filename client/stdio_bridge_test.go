package client_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/creachadair/jrpc2/channel"

	"github.com/brandonkramer/locdaemon/client"
)

func TestRelayFramesCopiesOneRecord(t *testing.T) {
	var buf bytes.Buffer
	src := &fakeChannel{recvMsgs: [][]byte{[]byte(`{"jsonrpc":"2.0","id":1}`)}}
	dst := &fakeChannel{w: &buf}

	if err := client.RelayFrames(dst, src); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected forwarded frame")
	}
}

type fakeChannel struct {
	recvMsgs [][]byte
	recvIdx  int
	w        *bytes.Buffer
}

func (f *fakeChannel) Send(msg []byte) error {
	_, err := f.w.Write(msg)
	return err
}

func (f *fakeChannel) Recv() ([]byte, error) {
	if f.recvIdx >= len(f.recvMsgs) {
		return nil, io.EOF
	}
	msg := f.recvMsgs[f.recvIdx]
	f.recvIdx++
	return msg, nil
}

func (f *fakeChannel) Close() error { return nil }

var _ channel.Channel = (*fakeChannel)(nil)
