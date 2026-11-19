package tea

import (
	"github.com/nsf/broadcast"
)

type Cmd interface{ isCmd() }

type cmdImpl struct{}

func (cmdImpl) isCmd() {}

//------------------------------------------

type ResponseHTTPStatus struct {
	cmdImpl
	Status int
}

type Broadcast struct {
	cmdImpl
	Topic broadcast.Topic
	Opts  []broadcast.BroadcastOpt
}
