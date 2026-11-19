package tea

import (
	"fmt"
	"log/slog"
	"net/http"
)

func cmd(cmds ...Cmd) []Cmd {
	return cmds
}

func broadcastIndex() Cmd {
	return Broadcast{Topic: NewTopicIndex()}
}

func broadcastSession(id string) Cmd {
	return Broadcast{Topic: NewTopicSessionID(id)}
}

func Update(model Model, msg Msg) (Model, []Cmd) {
	switch msg := msg.(type) {
	case Query:
		return model, nil

	case SessionJoined:
		model.Sessions = model.Sessions.Set(msg.ID, Session{ID: msg.ID})
		return model, nil

	case SessionLeft:
		model.Sessions = model.Sessions.Delete(msg.ID)
		return model, nil

	case ButtonPressed:
		if msg.SessionID != "" {
			if s, ok := model.Sessions.Get(msg.SessionID); ok {
				s.NumPresses++
				model.Sessions = model.Sessions.Set(msg.SessionID, s)
			} else {
				slog.Warn("session not found", "id", msg.SessionID)
			}
			return model, cmd(
				broadcastSession(msg.SessionID),
				ResponseHTTPStatus{Status: http.StatusNoContent},
			)
		} else {
			model.NumPresses++
			return model, cmd(
				broadcastIndex(),
				ResponseHTTPStatus{Status: http.StatusNoContent},
			)
		}

	default:
		panic(fmt.Sprintf("unexpected Msg: %#v", msg))
	}
}
