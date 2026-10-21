package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"github.com/a-h/templ"
	"github.com/alexflint/go-arg"
	"github.com/nsf/go-pwa-template/internal/idlib"
	"github.com/nsf/go-pwa-template/tpl"
	"github.com/nsf/rndgo/logu"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vearutop/statigz"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type session struct {
	id       string
	callback func()
}

func renderTemplate[T any](f func(arg T) templ.Component, arg T) []byte {
	var buf bytes.Buffer
	ctx := context.Background()
	logu.Err(f(arg).Render(ctx, &buf))
	return buf.Bytes()
}

type appServer struct {
	est      EventStreamTerminator
	lock     sync.Mutex
	views    int
	sessions map[string]*session
}

func (s *appServer) startSession(callback func()) *session {
	s.lock.Lock()
	defer s.lock.Unlock()
	sess := &session{callback: callback, id: idlib.Generate()}
	s.sessions[sess.id] = sess
	return sess
}

func (s *appServer) endSession(sess *session) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.sessions, sess.id)
}

func (s *appServer) getPageStringUnlocked() string {
	return fmt.Sprintf("Page has been viewed %d time(s)", s.views)
}

func (s *appServer) serveIndexTemplate(w http.ResponseWriter, r *http.Request) {
	s.lock.Lock()
	data := renderTemplate(tpl.Index, s.getPageStringUnlocked())
	s.lock.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	logu.Err2(w.Write(data))
}

func (s *appServer) updates(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	sess := s.startSession(func() {
		data := renderTemplate(tpl.IndexContent, s.getPageStringUnlocked())
		logu.Err(sse.PatchElements(string(data)))
	})
	defer s.endSession(sess)

	s.lock.Lock()
	s.views++
	for _, sess := range s.sessions {
		sess.callback()
	}
	s.lock.Unlock()

	select {
	case <-sse.Context().Done():
	case <-s.est:
	}
}

//go:embed static
var embeddedFiles embed.FS

type Command struct {
	SlogLevel slog.Level `arg:"--slog-level" default:"INFO" help:"log level for slog" placeholder:"LEVEL"`
	Addr      string     `default:"127.0.0.1:8000" help:"address to listen on for http connections"`
}

type EventStreamTerminator chan struct{}

func NewEventStreamTerminator() EventStreamTerminator {
	return make(EventStreamTerminator)
}

func (v EventStreamTerminator) Terminate() {
	close(v)
}

func main() {
	ctx := context.Background()

	var cmd Command
	arg.MustParse(&cmd)

	slog.SetLogLoggerLevel(cmd.SlogLevel)
	slog.Info("slog level", "level", cmd.SlogLevel)

	est := NewEventStreamTerminator()
	server := &appServer{
		est:      est,
		sessions: map[string]*session{},
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", statigz.FileServer(embeddedFiles, statigz.EncodeOnInit))
	mux.HandleFunc("GET /", server.serveIndexTemplate)
	mux.HandleFunc("GET /updates", server.updates)

	var wg sync.WaitGroup
	wg.Add(1)

	httpServer := http.Server{Addr: cmd.Addr, Handler: mux}
	httpServer.RegisterOnShutdown(func() {
		est.Terminate()
	})

	go func() {
		slog.Info("listen and serve", "addr", cmd.Addr)
		if err := httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				slog.Error("unexpected http server error", "error", err)
				os.Exit(1)
			}
		}
		slog.Info("http server stopped")
		wg.Done()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	slog.Info("SIGTERM received")
	slog.Info("graceful shutdown initiated")

	slog.Info("stopping http server")
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("unexpected error shutting down server", "error", err)
	}

	wg.Wait()
}
