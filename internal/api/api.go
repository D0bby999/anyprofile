// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api is AnyProfile's local REST API. It listens on 127.0.0.1 only,
// answers only requests that carry the token, and refuses anything a web
// page could send (an Origin header, or a Host that is not this loopback
// port), so a site open in some browser cannot drive it.
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Profile is what the API tells a script about one profile.
type Profile struct {
	ID     string   `json:"id"`
	Serial int      `json:"serial"`
	Name   string   `json:"name"`
	Sites  []string `json:"sites"`
	Tags   []string `json:"tags"`
	State  string   `json:"state"`
	// DevToolsPort is the loopback DevTools port, 0 unless this launch has one.
	DevToolsPort int `json:"devtoolsPort"`
	// WebSocketDebuggerURL is what Puppeteer's connect and Playwright's
	// connectOverCDP take; empty without a DevTools port.
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl,omitempty"`
}

// Backend is the part of the app the API drives.
type Backend interface {
	Profiles() ([]Profile, error)
	Start(ctx context.Context, id string, devtools bool) error
	Stop(id string) error
	// Pipe is the DevTools pipe of a running profile; a browser adopted after
	// a crash has none.
	Pipe(id string) (agent.Conn, bool)
}

// Server serves the API on one loopback port with one token.
type Server struct {
	backend Backend
	log     *slog.Logger
	port    int
	token   [32]byte // SHA-256 of the token, so the comparison is constant-time
	srv     *http.Server
	// devtoolsURL finds a browser's WebSocket URL; tests replace it.
	devtoolsURL func(port int) string
	// Agent keeps the MCP endpoint's view of each browser; /mcp is served
	// when it is set. It outlives the server, which restarts with Settings.
	Agent *agent.Hub
	// Uploads is the only folder upload_file takes files from; Recipes holds
	// the recipes /mcp offers. Both are read again on every request.
	Uploads, Recipes string
	prompts          promptSet
}

func New(b Backend, log *slog.Logger, port int, token string) *Server {
	return &Server{backend: b, log: log, port: port, token: sha256.Sum256([]byte(token)), devtoolsURL: browserWSURL}
}

// Start listens on 127.0.0.1:port; it fails at once when the port is taken.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(s.port))
	if err != nil {
		return apperr.Newf("api_port_busy", "port %d is in use", s.port)
	}
	s.srv = &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go s.srv.Serve(ln)
	s.log.Info("api listening", "addr", ln.Addr().String())
	return nil
}

// Close stops listening; requests in flight get a few seconds to finish.
func (s *Server) Close() {
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
	s.log.Info("api stopped")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/profiles", s.list)
	mux.HandleFunc("GET /v1/profiles/{id}", s.get)
	mux.HandleFunc("POST /v1/profiles/{id}/start", s.start)
	mux.HandleFunc("POST /v1/profiles/{id}/stop", s.stop)
	if s.Agent != nil {
		mux.Handle("/mcp", s.mcpHandler())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		if code, msg := s.refuse(r); code != 0 {
			writeErr(rec, code, apperr.New(msg, "request refused"))
		} else {
			mux.ServeHTTP(rec, r)
		}
		// The token never reaches the log: only method, path and outcome.
		s.log.Info("api", "method", r.Method, "path", r.URL.Path, "status", rec.code)
	})
}

// refuse returns a status and code for a request that must not run.
func (s *Server) refuse(r *http.Request) (int, string) {
	if r.Header.Get("Origin") != "" {
		return http.StatusForbidden, "api_origin"
	}
	port := ":" + strconv.Itoa(s.port)
	if r.Host != "127.0.0.1"+port && r.Host != "localhost"+port {
		return http.StatusForbidden, "api_host"
	}
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	sum := sha256.Sum256([]byte(tok))
	if !ok || subtle.ConstantTimeCompare(sum[:], s.token[:]) != 1 {
		return http.StatusUnauthorized, "api_token"
	}
	return 0, ""
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	ps, err := s.backend.Profiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": ps})
}

func (s *Server) find(id string) (Profile, error) {
	ps, err := s.backend.Profiles()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range ps {
		if p.ID == id {
			if p.DevToolsPort > 0 {
				p.WebSocketDebuggerURL = s.devtoolsURL(p.DevToolsPort)
			}
			return p, nil
		}
	}
	return Profile{}, store.ErrNotFound
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	p, err := s.find(r.PathValue("id"))
	if err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DevTools bool `json:"devtools"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err == nil && len(strings.TrimSpace(string(body))) > 0 {
		err = json.Unmarshal(body, &in)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, apperr.New("api_bad_body", "the body must be JSON like {\"devtools\": true}"))
		return
	}
	id := r.PathValue("id")
	if _, err := s.find(id); err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	if err := s.backend.Start(r.Context(), id, in.DevTools); err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	s.get(w, r)
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.find(id); err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	if err := s.backend.Stop(id); err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	s.get(w, r)
}

// browserWSURL asks the browser for its WebSocket endpoint.
func browserWSURL(port int) string {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var v struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	json.NewDecoder(resp.Body).Decode(&v)
	return v.URL
}

func statusOf(err error) int {
	var ae *apperr.Error
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &ae):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func writeErr(w http.ResponseWriter, status int, err error) {
	code, msg := "internal", err.Error()
	var ae *apperr.Error
	if errors.As(err, &ae) {
		code, msg = ae.Code, ae.Msg
	} else if errors.Is(err, store.ErrNotFound) {
		code, msg = "not_found", "no such profile"
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
