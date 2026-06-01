package observe

import "github.com/brandonkramer/message"

const (
	// TopicReady is the observe topic for daemon readiness payloads.
	TopicReady message.Topic = "ready"
	// TopicStatus is the observe topic for daemon status payloads.
	TopicStatus message.Topic = "status"
)
