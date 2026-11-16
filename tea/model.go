package tea

import (
	"context"

	"github.com/nsf/immutable"
	"github.com/nsf/logu"
)

type Model struct {
	NumPresses int
	Sessions   immutable.Map[string, Session]
}

type Session struct {
	ID         string
	NumPresses int
}

var StaticSession = Session{}

type sessionStateKey struct{}

func NewSessionContext(ctx context.Context, sess Session) context.Context {
	return context.WithValue(ctx, sessionStateKey{}, sess)
}

func SessionFromContextMust(ctx context.Context) Session {
	val, ok := ctx.Value(sessionStateKey{}).(Session)
	return logu.FatalOK(val, ok)
}
