package tea

type Msg interface{ isMsg() }

type msgImpl struct{}

func (msgImpl) isMsg() {}

//------------------------------------------

type Query struct {
	msgImpl
}

type SessionJoined struct {
	msgImpl
	ID string
}

type SessionLeft struct {
	msgImpl
	ID string
}

type ButtonPressed struct {
	msgImpl
	SessionID string
}
