package main

import (
	"bytes"
	"context"
	"embed"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/alexflint/go-arg"
	"github.com/nsf/broadcast"
	"github.com/nsf/go-pwa-template/internal/idlib"
	"github.com/nsf/go-pwa-template/tea"
	"github.com/nsf/go-pwa-template/tpl"
	"github.com/nsf/kv"
	"github.com/nsf/logu"
	"github.com/nsf/pwa"
	"github.com/starfederation/datastar-go/datastar"
)

type Signals struct {
	SessionID string `json:"sessionId"`
}

func dispatchBroadcast(cmds []tea.Cmd, hub *broadcast.Hub) {
	q := broadcast.Queue{}
	for _, cmd := range cmds {
		switch cmd := cmd.(type) {
		case tea.Broadcast:
			slog.Debug("broadcast", "topic", cmd.Topic)
			q.Broadcast(cmd.Topic, cmd.Opts...)
		}
	}
	hub.Commit(&q)
}

func dispatchResponse(cmds []tea.Cmd, w http.ResponseWriter, r *http.Request) {
	// only one response expected, handle the first and return
	for _, cmd := range cmds {
		switch cmd := cmd.(type) {
		case tea.ResponseHTTPStatus:
			slog.Debug("response http status", "status", cmd.Status)
			w.WriteHeader(cmd.Status)
			return
		}
	}
}

type response struct {
	model tea.Model
	cmds  []tea.Cmd
}

type msgEnvelope struct {
	msg tea.Msg
	ch  chan<- response
}

func modelProcessingLoop(ctx context.Context, in <-chan msgEnvelope, model tea.Model, s kv.JSONStore, hub *broadcast.Hub) {
	var cmds []tea.Cmd
	logger := slog.With("component", "model processing")

	for {
		select {
		case <-ctx.Done():
			if model.Sessions.Len() != 0 {
				logger.Warn("not all sessions terminated!")
			}
			return
		case envelope := <-in:
			model, cmds = tea.Update(model, envelope.msg)
			// TODO: dispatchPersist(cmds, s) // persist the model into DB store here
			dispatchBroadcast(cmds, hub)
			envelope.ch <- response{model: model, cmds: cmds}
		}
	}
}

func msgResponse(ch chan<- msgEnvelope, w http.ResponseWriter, r *http.Request, msg tea.Msg) {
	resp := msgRequest(ch, msg)
	dispatchResponse(resp.cmds, w, r)
}

func msgRequest(ch chan<- msgEnvelope, msg tea.Msg) response {
	resp := make(chan response)
	ch <- msgEnvelope{
		msg: msg,
		ch:  resp,
	}
	return <-resp
}

func patchSessionID(sse *datastar.ServerSentEventGenerator, sess *broadcast.Subscription) {
	type Data struct {
		SessionID string `json:"sessionId"`
	}
	logu.Err(sse.MarshalAndPatchSignals(&Data{SessionID: string(sess.ID())}))
}

func renderTemplate[T any](f func(arg T) templ.Component, session tea.Session, arg T) []byte {
	var buf bytes.Buffer
	ctx := tea.NewSessionContext(context.Background(), session)
	logu.Err(f(arg).Render(ctx, &buf))
	return buf.Bytes()
}

type appServer struct {
	httpServerShutdownCtx context.Context
	hub                   *broadcast.Hub
	in                    chan<- msgEnvelope
}

func (s *appServer) startSubscription(subs ...broadcast.Topic) *broadcast.Subscription {
	id := idlib.Generate()
	allSubs := append([]broadcast.Topic{tea.NewTopicSessionID(id)}, subs...)
	sub := s.hub.Subscribe(broadcast.SubscriptionID(id), allSubs...)
	msgRequest(s.in, tea.SessionJoined{ID: id})
	return sub
}

func (s *appServer) endSubscription(sub *broadcast.Subscription) {
	msgRequest(s.in, tea.SessionLeft{ID: string(sub.ID())})
	sub.Unsubscribe()
}

func (s *appServer) serveIndexTemplate(w http.ResponseWriter, r *http.Request) {
	resp := msgRequest(s.in, tea.Query{})
	data := renderTemplate(tpl.Index, tea.StaticSession, resp.model)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	logu.Err2(w.Write(data))
}

func (s *appServer) updates(w http.ResponseWriter, r *http.Request) {
	sub := s.startSubscription(tea.NewTopicIndex())
	defer s.endSubscription(sub)

	sse := datastar.NewSSE(w, r)
	patchSessionID(sse, sub)

	for {
		select {
		case <-sse.Context().Done():
			return
		case <-s.httpServerShutdownCtx.Done():
			return
		case <-sub.C():
			resp := msgRequest(s.in, tea.Query{})
			sess := resp.model.Sessions.GetOr(string(sub.ID()), tea.StaticSession)
			data := renderTemplate(tpl.IndexContent, sess, resp.model)
			logu.Err(sse.PatchElements(string(data)))
		}
	}
}

func (s *appServer) handleButtonPress(w http.ResponseWriter, r *http.Request) {
	msgResponse(s.in, w, r, tea.ButtonPressed{})
}

func (s *appServer) handleSessionButtonPress(w http.ResponseWriter, r *http.Request) {
	var signals Signals
	logu.Err(datastar.ReadSignals(r, &signals))
	msgResponse(s.in, w, r, tea.ButtonPressed{SessionID: signals.SessionID})
}

//go:embed static
var embeddedFiles embed.FS

type Command struct {
	SlogLevel slog.Level `arg:"--slog-level" default:"INFO" help:"log level for slog" placeholder:"LEVEL"`
	Addr      string     `default:"127.0.0.1:8000" help:"address to listen on for http connections"`
}

func main() {
	var cmd Command
	arg.MustParse(&cmd)

	in := make(chan msgEnvelope)
	model := tea.Model{}

	pwa.Run(pwa.Parameters{
		Addr:        cmd.Addr,
		LoggerLevel: cmd.SlogLevel,
		StaticFiles: embeddedFiles,
		Setup: func(sc pwa.SetupContext) {
			sc.GoModelProcessing(func(ctx context.Context) {
				modelProcessingLoop(ctx, in, model, sc.Store, sc.Hub)
			})
			server := &appServer{
				httpServerShutdownCtx: sc.HTTPServerShutdownContext,
				hub:                   sc.Hub,
				in:                    in,
			}
			sc.Mux.HandleFunc("GET /", server.serveIndexTemplate)
			sc.Mux.HandleFunc("GET /updates", server.updates)
			sc.Mux.HandleFunc("POST /button", server.handleButtonPress)
			sc.Mux.HandleFunc("POST /session/button", server.handleSessionButtonPress)
		},
	})
}
