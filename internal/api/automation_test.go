// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func postJSON(t *testing.T, srv interface{ URL() string }, path string, body any, out any) int {
	return 0
}

func TestHTTPAutomationFindAndRun(t *testing.T) {
	_, s, srv := mcpSetup(t)
	s.Traces = t.TempDir()
	if err := s.backend.Start(context.Background(), "p1", false); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: rewrite{token: token}}
	do := func(path string, body any, out any) int {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
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
		OK     bool `json:"ok"`
		Result []struct {
			Ref string `json:"ref"`
		} `json:"result"`
	}
	if code := do("/v1/act", map[string]any{"profile": "Kênh", "action": "find", "target": map[string]any{"text": "Go"}}, &one); code != 200 || !one.OK || len(one.Result) != 1 || one.Result[0].Ref != "e5" {
		t.Fatalf("act = %d %+v", code, one)
	}

	var run struct {
		Steps []struct {
			OK bool `json:"ok"`
		} `json:"steps"`
	}
	steps := []map[string]any{
		{"action": "find", "target": map[string]any{"text": "Go"}},
		{"action": "find", "target": map[string]any{"text": "Go"}},
	}
	if code := do("/v1/run", map[string]any{"profile": "Kênh", "steps": steps}, &run); code != 200 || len(run.Steps) != 2 || !run.Steps[0].OK || !run.Steps[1].OK {
		t.Fatalf("run = %d %+v", code, run)
	}

	code := do("/v1/act", map[string]any{"profile": "Kênh", "action": "bogus"}, nil)
	if code != 400 {
		t.Fatalf("bad action status = %d, want 400", code)
	}
}
