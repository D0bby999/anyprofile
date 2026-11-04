// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// lease is one session's hold on a profile or device. It lives in memory: a
// lease only means something while the app runs.
type lease struct {
	id      string
	key     string
	owner   string
	surface string // profile/device as the caller named it
	record  bool
	steps   []actRequest
	expires time.Time
}

type leaseStore struct {
	mu    sync.Mutex
	byID  map[string]*lease
	byKey map[string]*lease
}

func (l *leaseStore) sweep(now time.Time) {
	for id, x := range l.byID {
		if now.After(x.expires) {
			delete(l.byID, id)
			if l.byKey[x.key] == x {
				delete(l.byKey, x.key)
			}
		}
	}
}

func (l *leaseStore) acquire(key string, ttl time.Duration) (*lease, error) {
	return l.acquireFor(key, "", ttl)
}

// acquireFor lets the same owner (an MCP session) re-open its profile without
// colliding with itself, while a different owner gets busy.
func (l *leaseStore) acquireFor(key, owner string, ttl time.Duration) (*lease, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byID == nil {
		l.byID = map[string]*lease{}
		l.byKey = map[string]*lease{}
	}
	now := time.Now()
	l.sweep(now)
	if x := l.byKey[key]; x != nil {
		if owner != "" && x.owner == owner {
			x.expires = now.Add(ttl)
			return x, nil
		}
		code := "profile_busy"
		if len(key) > 7 && key[:7] == "device:" {
			code = "device_busy"
		}
		return nil, apperr.Newf(code, "held by %s for %s", x.id, x.expires.Sub(now).Round(time.Second))
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	x := &lease{id: "s_" + hex.EncodeToString(b), key: key, owner: owner, expires: now.Add(ttl)}
	l.byID[x.id] = x
	l.byKey[key] = x
	return x, nil
}

func (l *leaseStore) touch(id string) (*lease, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byID[id]
	if x == nil || time.Now().After(x.expires) {
		return nil, apperr.Newf("session_not_found", "no live session %q", id)
	}
	x.expires = time.Now().Add(60 * time.Second)
	return x, nil
}

// releaseFor releases a key only for the owner that holds it.
// ownerOf returns the live owner of a key, or "" when it is free. Expired
// leases are removed on the way.
func (l *leaseStore) ownerOf(key string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byKey[key]
	if x == nil {
		return ""
	}
	if time.Now().After(x.expires) {
		delete(l.byID, x.id)
		delete(l.byKey, key)
		return ""
	}
	return x.owner
}

func (l *leaseStore) releaseFor(key, owner string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byKey[key]
	if x == nil || x.owner != owner {
		return false
	}
	delete(l.byID, x.id)
	delete(l.byKey, key)
	return true
}

func (l *leaseStore) release(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byID[id]
	if x == nil {
		return false
	}
	delete(l.byID, id)
	if l.byKey[x.key] == x {
		delete(l.byKey, x.key)
	}
	return true
}

type sessionIn struct {
	Profile    string `json:"profile,omitempty"`
	Device     string `json:"device,omitempty"`
	TTLSeconds int    `json:"ttlSeconds,omitempty"`
	Record     bool   `json:"record,omitempty"`
}

func (s *Server) sessionKey(in sessionIn) (string, error) {
	if (in.Profile == "") == (in.Device == "") {
		return "", apperr.New("api_surface", "give profile or device, not both")
	}
	if in.Profile != "" {
		p, err := s.profile(in.Profile)
		if err != nil {
			return "", err
		}
		return "profile:" + p.ID, nil
	}
	if s.Mobile == nil {
		return "", apperr.New("mobile_unsupported", "this app has no mobile driver")
	}
	d, err := s.device(in.Device)
	if err != nil {
		return "", err
	}
	return "device:" + d.ID, nil
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var in sessionIn
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, apperr.New("api_body", "the body is not JSON: "+err.Error()))
		return
	}
	ttl := time.Duration(in.TTLSeconds) * time.Second
	if ttl <= 0 || ttl > time.Hour {
		ttl = 60 * time.Second
	}
	key, err := s.sessionKey(in)
	if err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	x, err := (&s.leases).acquire(key, ttl)
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	if in.Record {
		surface := in.Profile
		if surface == "" {
			surface = in.Device
		}
		(&s.leases).setRecord(x.id, surface)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": x.id, "key": key, "expiresAt": x.expires.UTC().Format(time.RFC3339)})
}

func (l *leaseStore) setRecord(id, surface string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if x := l.byID[id]; x != nil {
		x.record = true
		x.surface = surface
	}
}

// recordable actions are intent, not observation: find/read/screenshot are
// skipped, and a step marked secret keeps only a placeholder.
var recordable = map[string]bool{
	"navigate": true, "goto": true, "click": true, "type": true, "press": true, "press_key": true,
	"scroll": true, "select": true, "hover": true, "double_click": true, "right_click": true, "drag": true,
	"tap": true, "tap_text": true, "swipe": true, "long_press": true, "launch": true, "open_url": true, "pull": true,
}

func (l *leaseStore) recordStep(id string, in actRequest) {
	if id == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byID[id]
	if x == nil || !x.record || !recordable[strings.ToLower(in.Action)] {
		return
	}
	in.Session = ""
	if in.Secret {
		in.Text = "{{SECRET}}"
	}
	in.Secret = false
	x.steps = append(x.steps, in)
}

func (l *leaseStore) setRecordKey(key, surface string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byKey[key]
	if x == nil {
		return false
	}
	x.record = true
	x.surface = surface
	return true
}

func (l *leaseStore) appendForOwner(owner string, step actRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, x := range l.byKey {
		if x.owner == owner && x.record {
			step.Session = ""
			x.steps = append(x.steps, step)
			return
		}
	}
}

func (l *leaseStore) flowForOwner(owner string) (map[string]any, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, x := range l.byKey {
		if x.owner != owner || !x.record {
			continue
		}
		flow := map[string]any{"version": 1, "steps": append([]actRequest(nil), x.steps...)}
		if strings.HasPrefix(x.key, "device:") {
			flow["device"] = x.surface
		} else {
			flow["profile"] = x.surface
		}
		return flow, true
	}
	return nil, false
}

func (l *leaseStore) stopRecordOwner(owner string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, x := range l.byKey {
		if x.owner == owner {
			x.record = false
			x.steps = nil
		}
	}
}

func (l *leaseStore) flow(id string) (map[string]any, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.byID[id]
	if x == nil || !x.record {
		return nil, false
	}
	flow := map[string]any{"version": 1, "steps": append([]actRequest(nil), x.steps...)}
	if strings.HasPrefix(x.key, "device:") {
		flow["device"] = x.surface
	} else {
		flow["profile"] = x.surface
	}
	return flow, true
}

func (s *Server) sessionRecord(w http.ResponseWriter, r *http.Request) {
	flow, ok := (&s.leases).flow(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, apperr.New("session_not_found", "no recording session with that id"))
		return
	}
	writeJSON(w, http.StatusOK, flow)
}

func (s *Server) sessionHeartbeat(w http.ResponseWriter, r *http.Request) {
	x, err := (&s.leases).touch(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": x.id, "expiresAt": x.expires.UTC().Format(time.RFC3339)})
}

func (s *Server) releaseSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !(&s.leases).release(id) {
		writeErr(w, http.StatusNotFound, apperr.Newf("session_not_found", "no session %q", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"released": id})
}

// checkSession validates a session id against the action's surface and
// extends its lease. An empty session is allowed for backward compatibility.
func (s *Server) checkSession(id string, in actRequest) error {
	key, err := s.sessionKey(sessionIn{Profile: in.Profile, Device: in.Device})
	if err != nil {
		return err
	}
	if id == "" {
		if o := (&s.leases).ownerOf(key); o != "" {
			return apperr.Newf("profile_busy", "held by another session; pass its session id or close it").Safe()
		}
		return nil
	}
	x, err := (&s.leases).touch(id)
	if err != nil {
		return err
	}
	if x.key != key {
		return apperr.Newf("session_surface", "session %s is for %s, not %s", id, x.key, key)
	}
	return nil
}
