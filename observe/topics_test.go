package observe_test

import (
	"testing"

	"github.com/brandonkramer/message"

	"github.com/brandonkramer/locdaemon/observe"
)

func TestTopics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  message.Topic
		want message.Topic
	}{
		{"ready", observe.TopicReady, "ready"},
		{"status", observe.TopicStatus, "status"},
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
