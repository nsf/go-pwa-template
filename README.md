# go-pwa-template

Template for **Go + PWA hypermedia** applications: a single Go binary that serves
server-rendered HTML (templ), styles it with Tailwind CSS v4 + daisyUI, and makes the page
reactive/real-time with [Datastar](https://data-star.dev) over SSE — packaged as an
installable Progressive Web App. No React/Vue, no client-side router, no SPA build step
beyond CSS and templ codegen. The page also renders fine with JavaScript disabled; SSE is
purely additive.

Application state is managed in the **TEA style (The Elm Architecture)**: a pure
`Update(model, msg)` reducer plus a set of effectful `Cmd`s that the server layer
dispatches (HTTP responses, SSE broadcasts). The app state is a single `tea.Model` owned
by one goroutine; HTTP handlers only send messages into it and read back the result.

The demo page has two buttons: a **Shared Button** whose counter lives in the global model
and is broadcast to *all* connected clients, and a **Session Button** whose counter is
per-browser-session (each SSE connection gets a session id, patched into the page as a
Datastar signal). Open the page in several windows/tabs to see shared vs. per-session
state.

The code is small — read `tea/` (the app logic), `tpl/index.templ` (the view) and
`main.go` (the wiring).

> Status: a working *skeleton*, not a production-ready PWA. No service worker, no tests,
> no CI, no license. Persistence into the provided `kv.JSONStore` is a TODO
> (`dispatchPersist` in `main.go`).

## Stack

| Layer | Tool | Notes |
|---|---|---|
| Server | Go (`net/http`, go.mod declares `go 1.27.3`) | stdlib mux with method+pattern routes (`GET /`, `GET /updates`, `POST /button`, `POST /session/button`) |
| App skeleton | local `tea/` package | TEA / Elm Architecture: `Model`, `Msg`, `Cmd`, pure `Update`, `Topic`s |
| PWA server harness | [nsf/pwa](https://github.com/nsf/pwa) v0.0.1 | `pwa.Run(Parameters{Addr, LoggerLevel, StaticFiles, Setup})`: owns the HTTP server, serves `//go:embed static` at `/static/` (statigz + brotli/gzip), slog setup, graceful shutdown; `SetupContext` provides `Mux`, `Hub`, `Store`, `HTTPServerShutdownContext`, `GoModelProcessing` |
| Pub/sub | [nsf/broadcast](https://github.com/nsf/broadcast) v0.0.1 | topic `Hub`, per-connection `Subscription`s, `Queue`+`Commit` for batched broadcasts |
| Persistence store | [nsf/kv](https://github.com/nsf/kv) v0.0.2 | `kv.JSONStore` handed to the model loop (wired, but persisting is a TODO) |
| Templating | [a-h/templ](https://templ.guide) v0.3.1118 | `.templ` → generated `_templ.go` |
| Reactivity | [starfederation/datastar-go](https://data-star.dev) v1.2.2 + vendored `datastar.js` v1.0.4 | SSE-driven: `PatchElements` (outerHTML morph of `#index-page`), signal patches (`sessionId`), `data-on:click` POSTs, `data-init="@get('/updates')"`, `ReadSignals` |
| Immutable state | [nsf/immutable](https://github.com/nsf/immutable) v0.0.3 | `immutable.Map[string, Session]` inside the model |
| CSS | Tailwind CSS v4 CLI + vendored daisyUI bundles (`static-src/*.mjs`) | `--minify`; scanned sources = `tpl/*.templ` only; `corporate`/`business` themes; custom `standalone`/`fullscreen` variants |
| CLI flags | [alexflint/go-arg](https://github.com/alexflint/go-arg) v1.6.1 | derived from the `Command` struct |
| Logging | stdlib `log/slog` + [nsf/logu](https://github.com/nsf/logu) (`Err`/`Err2`/`FatalOK`) | level via `--slog-level` |
| IDs | [matoous/go-nanoid/v2](https://github.com/matoous/go-nanoid) via `internal/idlib` | 8 chars, `0-9a-z`, used for session ids |
| Manifest templating | [gott](https://github.com/nsf/gott) (external text-template tool; not a Go module dependency) | `static-src/manifest.json.tpl` → `static/manifest.json` |

## Architecture (TEA)

`tea/` is pure app logic — no HTTP, no I/O:

- `model.go` — `Model{NumPresses, Sessions}`, per-session `Session{ID, NumPresses}`; the
  session is injected into the render context (`NewSessionContext` /
  `SessionFromContextMust`) so the view can read it.
- `msg.go` — messages: `Query`, `SessionJoined`, `SessionLeft`, `ButtonPressed{SessionID}`.
- `cmd.go` — effects: `ResponseHTTPStatus` (what to answer the POST with), `Broadcast`
  (what topics to publish to the SSE hub).
- `update.go` — `Update(model, msg) (model, []Cmd)`, a pure reducer.
- `topic.go` — broadcast topics: `index` (all clients) and `session-<id>` (one client).

`main.go` is the wiring ("runtime"):

- One goroutine (`modelProcessingLoop`) owns the model; handlers send `msgEnvelope{msg,
  replyCh}` into a channel and block for the `(model, cmds)` reply — so the reducer stays
  effectively sequential without any locking.
- `Cmd`s are dispatched per request: `ResponseHTTPStatus` → the HTTP response, `Broadcast`
  → `hub.Queue`/`Commit`.
- `GET /` renders `tpl.Index` with a `StaticSession` (plain HTML, no JS needed).
- `GET /updates` (the page's `data-init` SSE) subscribes the client to topic
  `session-<id>`, patches the generated session id in as the Datastar `sessionId` signal,
  sends `SessionJoined`, then re-renders `tpl.IndexContent` and `PatchElements` on every
  broadcast it receives (topic `index` is also subscribed, that's how shared counter
  updates arrive). On disconnect it sends `SessionLeft`.
- `POST /button` → `ButtonPressed{}` (shared); `POST /session/button` → `ReadSignals`
  pulls `sessionId` out of the request and sends `ButtonPressed{SessionID}`.

## Layout

```
go-pwa-template/
├── main.go                      # HTTP/SSE wiring: msg channel, model loop, Cmd dispatch, embed FS
├── go.mod / go.sum              # module github.com/nsf/go-pwa-template
├── build.bash                   # one-shot build: icons → screenshots → manifest → tailwind → templ → go build
├── watch-all.bash               # dev loop in tmux: tailwind --watch | templ generate -watch | air
├── tea/                         # TEA app core (pure, no I/O)
│   ├── model.go                 # Model/Session, session-in-context helpers
│   ├── msg.go                   # Msg types
│   ├── cmd.go                   # Cmd types (ResponseHTTPStatus, Broadcast)
│   ├── update.go                # Update(model, msg) reducer
│   └── topic.go                 # broadcast topics (index / session-<id>)
├── tpl/
│   ├── index.templ              # head / layout / IndexContent / Index  (source of truth)
│   └── index_templ.go           # generated by `templ generate` — git-ignored, DO NOT EDIT
├── internal/idlib/              # Generate() → nanoid(8, "0-9a-z") for session ids
├── static-src/                  # ALL SOURCE INPUTS (not served)
│   ├── style.css                # @import tailwindcss, daisyUI plugins, custom variants
│   ├── daisyui.mjs              # vendored daisyUI bundle (~344 KB, MIT)
│   ├── daisyui-theme.mjs        # vendored daisyUI theme-definition plugin (~48 KB, MIT)
│   ├── datastar.js              # vendored Datastar v1.0.4 ESM bundle (~36 KB)
│   ├── manifest.json.tpl        # PWA manifest template (gott substitutions)
│   ├── icon.svg                 # app icon source
│   └── desktop.svg, mobile.svg  # manifest screenshot sources
└── static/                      # GENERATED + SERVED at /static/ (git-ignored, absent in a clean tree)
    ├── style.css                # tailwind build output (minified)
    ├── datastar.js              # copied from static-src/
    ├── manifest.json            # rendered from manifest.json.tpl
    ├── icons/                   # PNGs 48/72/96/128/144/152/192/256/384/512
    └── screenshots/             # desktop.png (1440x900), mobile.png (390x844)
```

Everything under `static/` is build output and should be git-ignored (`/static`), as should
`tpl/*_templ.go` (`/tpl/*.go`). Nothing in the repo is hand-edited after generation —
change the source in `static-src/` or `tpl/`, then rebuild.

This is the right default for a *template*, but for an actual app you probably want to drop
`/static` and `/tpl/*.go` from `.gitignore` and commit the generated assets: `main.go` does `//go:embed
static`, so without a populated `static/` a clean checkout fails to compile and
`go install github.com/you/app@latest` does not work.

## Prerequisites

Go toolchain plus these external tools (none of them are pinned or vendored):

- `build.bash`: `tailwindcss` (v4 CLI), `templ`, `gott`, `rsvg-convert`, `magick` (ImageMagick)
- `watch-all.bash`: `tmux`, `air`

## Quick start

```sh
./build.bash
./bin/my-application --addr 127.0.0.1:8000 --slog-level DEBUG
# then open http://127.0.0.1:8000/
```

`build.bash` must run first: it populates `static/` (which `//go:embed static` needs to
compile) and writes the binary.

## Build

```sh
./build.bash
```

Idempotent one-shot build: generated assets, templ codegen, then `go build -o
bin/my-application`. Read the script — it's short.

## Dev loop

```sh
./watch-all.bash
```

Starts a tmux session with a CSS watcher, a templ watcher and `air` for Go hot reload.

## Extending

- New UI/logic: add `Msg`/`Cmd` types and cases in `tea/update.go`, render in
  `tpl/index.templ`, dispatch new `Cmd`s in `main.go`'s dispatch functions.
- Broadcast more granularly: give each entity its own `broadcast.Topic` (see
  `tea/topic.go`) and subscribe subscriptions to it in `main.go`.
- Persist: implement `dispatchPersist(cmds, s)` in `modelProcessingLoop` to write the model
  to the `kv.JSONStore` from `pwa.SetupContext`.
