// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

const (
	maxRequests = 500     // kept per tab, newest
	MaxBody     = 200_000 // bytes of one response body returned
)

// Request is one network request of a tab.
type Request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	URL    string `json:"url"`
	Type   string `json:"type"`
	Status int    `json:"status"`
	MIME   string `json:"mime"`
	Size   int64  `json:"size"`
	Failed string `json:"failed,omitempty"`
	Done   bool   `json:"done"`
}

// netLog keeps the newest requests of a tab in memory only. The browser's
// lock guards it.
type netLog struct {
	max   int
	order []string
	byID  map[string]*Request
}

func newNetLog(max int) *netLog { return &netLog{max: max, byID: map[string]*Request{}} }

func (l *netLog) add(method string, params json.RawMessage) {
	var p struct {
		RequestID string `json:"requestId"`
		Type      string `json:"type"`
		Request   struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		} `json:"request"`
		Response struct {
			Status   int    `json:"status"`
			MimeType string `json:"mimeType"`
		} `json:"response"`
		EncodedDataLength float64 `json:"encodedDataLength"`
		ErrorText         string  `json:"errorText"`
	}
	if json.Unmarshal(params, &p) != nil || p.RequestID == "" {
		return
	}
	r := l.byID[p.RequestID]
	switch method {
	case "Network.requestWillBeSent":
		if strings.HasPrefix(p.Request.URL, "data:") {
			return
		}
		if r == nil {
			r = &Request{ID: p.RequestID}
			l.byID[p.RequestID] = r
			l.order = append(l.order, p.RequestID)
			if len(l.order) > l.max {
				delete(l.byID, l.order[0])
				l.order = l.order[1:]
			}
		}
		// A redirect sends this again under the same id, with the new address.
		r.URL, r.Method, r.Type = p.Request.URL, p.Request.Method, p.Type
	case "Network.responseReceived":
		if r != nil {
			r.Status, r.MIME = p.Response.Status, p.Response.MimeType
			if p.Type != "" {
				r.Type = p.Type
			}
		}
	case "Network.loadingFinished":
		if r != nil {
			r.Done, r.Size = true, int64(p.EncodedDataLength)
		}
	case "Network.loadingFailed":
		if r != nil {
			r.Done, r.Failed = true, p.ErrorText
		}
	}
}

// list returns copies of the requests whose address contains urlPart and whose
// type is one of types (any when empty), oldest first, the newest limit.
func (l *netLog) list(urlPart string, types []string, limit int) []Request {
	var out []Request
	for _, id := range l.order {
		r := l.byID[id]
		if urlPart != "" && !strings.Contains(r.URL, urlPart) {
			continue
		}
		if len(types) > 0 && !containsFold(types, r.Type) {
			continue
		}
		out = append(out, *r)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// Requests lists the tab's requests since the agent first used the tab.
func (b *Browser) Requests(ctx context.Context, id, urlPart string, types []string, limit int) (string, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return "", err
	}
	if limit <= 0 || limit > maxRequests {
		limit = 50
	}
	b.mu.Lock()
	rs := t.net.list(urlPart, types, limit)
	total := len(t.net.order)
	b.mu.Unlock()
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tab %s: %d of %d requests kept (oldest first). Requests made before the agent first used this tab are not here; reload to see them.\n", short(t.targetID), len(rs), total)
	for _, r := range rs {
		state := fmt.Sprint(r.Status)
		switch {
		case r.Failed != "":
			state = "failed " + r.Failed
		case !r.Done && r.Status == 0:
			state = "pending"
		}
		u := r.URL
		if len(u) > 300 {
			u = u[:300] + "…"
		}
		fmt.Fprintf(&sb, "%s %s %s %s %s %dB %s\n", r.ID, r.Method, state, r.Type, r.MIME, r.Size, u)
	}
	return sb.String(), nil
}

func textual(mime string) bool {
	return strings.HasPrefix(mime, "text/") || strings.Contains(mime, "json") || strings.Contains(mime, "xml") ||
		strings.Contains(mime, "javascript") || strings.Contains(mime, "x-www-form-urlencoded")
}

// ResponseBody returns the body of one request of the tab, when it is text,
// from offset on and at most MaxBody bytes.
func (b *Browser) ResponseBody(ctx context.Context, id, requestID string, offset int) (string, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	r, ok := t.net.byID[requestID]
	var req Request
	if ok {
		req = *r
	}
	b.mu.Unlock()
	if !ok {
		return "", apperr.Newf("agent_request_not_found", "no request %q in this tab; see network_requests", requestID)
	}
	var body struct {
		Body          string `json:"body"`
		Base64Encoded bool   `json:"base64Encoded"`
	}
	if err := b.call(ctx, t.session, "Network.getResponseBody", map[string]any{"requestId": requestID}, &body); err != nil {
		return "", apperr.New("agent_no_body", "the browser no longer has this body (or it had none); reload and try again")
	}
	data := []byte(body.Body)
	if body.Base64Encoded {
		if !textual(req.MIME) {
			return fmt.Sprintf("%s %s: binary %s, %d bytes; not shown.", req.Method, req.URL, req.MIME, len(body.Body)*3/4), nil
		}
		if data, err = base64.StdEncoding.DecodeString(body.Body); err != nil {
			return "", err
		}
	}
	if offset < 0 || offset > len(data) {
		offset = len(data)
	}
	end := min(len(data), offset+MaxBody)
	// Cut on character boundaries, so a page of text never splits a letter.
	for offset < len(data) && !utf8.RuneStart(data[offset]) {
		offset++
	}
	for end < len(data) && end > offset && !utf8.RuneStart(data[end]) {
		end--
	}
	out := fmt.Sprintf("%s %s → %d %s\n\n%s", req.Method, req.URL, req.Status, req.MIME, data[offset:end])
	if end < len(data) {
		out += fmt.Sprintf("\n… %d more bytes; call response_body with offset=%d.", len(data)-end, end)
	}
	return out, nil
}
