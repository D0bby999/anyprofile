// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/automation"
)

func TestHTTPSessionLease(t *testing.T) {
	_, s, srv := mcpSetup(t)
	client := &http.Client{Transport: rewrite{token: token}}
	do := func(method, path string, body any, out any) int {
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if out != nil {
			_ = json.NewDecoder(res.Body).Decode(out)
		}
		return res.StatusCode
	}

	var one struct {
		ID string `json:"id"`
	}
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "Kênh"}, &one); code != 201 || one.ID == "" {
		t.Fatalf("create = %d %+v", code, one)
	}
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "Kênh"}, nil); code != 409 {
		t.Fatalf("second create = %d, want 409", code)
	}
	if code := do("POST", "/v1/sessions/"+one.ID+"/heartbeat", nil, nil); code != 200 {
		t.Fatalf("heartbeat = %d", code)
	}
	if code := do("POST", "/v1/act", map[string]any{"profile": "p2", "session": one.ID, "action": "find", "target": map[string]any{"text": "Go"}}, nil); code != 400 {
		t.Fatalf("session on another profile = %d, want 400", code)
	}
	if code := do("DELETE", "/v1/sessions/"+one.ID, nil, nil); code != 200 {
		t.Fatalf("release = %d", code)
	}
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "Kênh"}, &one); code != 201 {
		t.Fatalf("create after release = %d", code)
	}

	// A one-second lease expires; the same profile can be leased again.
	var short struct {
		ID string `json:"id"`
	}
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "p2", "ttlSeconds": 1}, &short); code != 201 {
		t.Fatalf("short create = %d", code)
	}
	time.Sleep(1100 * time.Millisecond)
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "p2"}, nil); code != 201 {
		t.Fatalf("expired lease still held: %d", code)
	}

	var recording struct {
		ID string `json:"id"`
	}
	if code := do("POST", "/v1/sessions", map[string]any{"profile": "p3", "record": true}, &recording); code != 201 || recording.ID == "" {
		t.Fatalf("record session = %d %+v", code, recording)
	}
	s.leases.recordStep(recording.ID, actRequest{Action: "click", Target: &automation.Target{Text: "Lưu"}})
	s.leases.recordStep(recording.ID, actRequest{Action: "find", Target: &automation.Target{Text: "Lưu"}})
	s.leases.recordStep(recording.ID, actRequest{Action: "type", Text: "hunter2", Secret: true})
	var flow struct {
		Profile string       `json:"profile"`
		Steps   []actRequest `json:"steps"`
	}
	if code := do("GET", "/v1/sessions/"+recording.ID+"/record", nil, &flow); code != 200 {
		t.Fatalf("record = %d", code)
	}
	if flow.Profile != "p3" || len(flow.Steps) != 2 || flow.Steps[0].Action != "click" || flow.Steps[1].Text != "{{SECRET}}" {
		t.Fatalf("flow = %+v", flow)
	}
}
