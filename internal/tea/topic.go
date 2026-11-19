package tea

import (
	"github.com/nsf/broadcast"
)

func NewTopicIndex() broadcast.Topic { return "index" }
func NewTopicSessionID(sessionID string) broadcast.Topic {
	return "session-" + broadcast.Topic(sessionID)
}
