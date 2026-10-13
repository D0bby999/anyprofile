// SPDX-License-Identifier: AGPL-3.0-or-later

// Package fingerprint measures a browser once in a hidden tab and compares the
// sample with a saved baseline.
package fingerprint

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Sample is what the probe returns. The eight fields up to Screen are compared
// against the baseline; TZ and Langs are compared against the profile instead.
type Sample struct {
	Canvas        string  `json:"canvas"`
	WebGLVendor   string  `json:"webglVendor"`
	WebGLRenderer string  `json:"webglRenderer"`
	UA            string  `json:"ua"`
	Platform      string  `json:"platform"`
	HWC           int     `json:"hwc"`
	Mem           float64 `json:"mem"`
	Screen        string  `json:"screen"`
	TZ            string  `json:"tz"`
	Langs         string  `json:"langs"`
	Secure        bool    `json:"secure"`
}

const origin = "https://fp.anyprofile.invalid/" // .invalid never resolves; the page is served locally

//go:embed probe.js
var probeJS string

var pageHTML = base64.StdEncoding.EncodeToString([]byte("<!doctype html><title>probe</title>"))

type evalReply struct {
	Result           struct{ Value json.RawMessage } `json:"result"`
	ExceptionDetails json.RawMessage                 `json:"exceptionDetails"`
}

// Capture measures the browser once in a hidden tab and closes the tab.
func Capture(ctx context.Context, c *cdppipe.Conn) (Sample, error) {
	// A cheap handshake first: it confirms the DevTools pipe is up before the
	// hidden target is created, which Chromium otherwise refuses with
	// "Hidden target can be created only when remote debugging is enabled".
	if _, err := c.Call(ctx, "", "Browser.getVersion", nil); err != nil {
		return Sample{}, err
	}

	var r json.RawMessage
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		r, err = c.Call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank", "background": true, "hidden": true})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return Sample{}, err
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		return Sample{}, err
	}
	var tgt struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(r, &tgt); err != nil {
		return Sample{}, err
	}
	defer c.Call(context.Background(), "", "Target.closeTarget", map[string]any{"targetId": tgt.TargetID})

	r, err = c.Call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": tgt.TargetID, "flatten": true})
	if err != nil {
		return Sample{}, err
	}
	var att struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(r, &att); err != nil {
		return Sample{}, err
	}
	sid := att.SessionID

	if _, err := c.Call(ctx, sid, "Fetch.enable", map[string]any{"patterns": []map[string]string{{"urlPattern": origin + "*"}}}); err != nil {
		return Sample{}, err
	}
	// Fulfil every paused request with a minimal local page; no network.
	go func() {
		for ev := range c.Events() {
			if ev.Method != "Fetch.requestPaused" {
				continue
			}
			var p struct {
				RequestID string `json:"requestId"`
			}
			json.Unmarshal(ev.Params, &p)
			c.Call(context.Background(), ev.SessionID, "Fetch.fulfillRequest", map[string]any{
				"requestId": p.RequestID, "responseCode": 200,
				"responseHeaders": []map[string]string{{"name": "Content-Type", "value": "text/html"}},
				"body":            pageHTML,
			})
		}
	}()

	if _, err := c.Call(ctx, sid, "Page.navigate", map[string]any{"url": origin}); err != nil {
		return Sample{}, err
	}
	// Wait up to 5 s for the page to complete.
	navCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		r, err = c.Call(navCtx, sid, "Runtime.evaluate", map[string]any{"expression": "location.href + '|' + document.readyState", "returnByValue": true})
		if err == nil && strings.Contains(string(r), origin+"|complete") {
			break
		}
		select {
		case <-navCtx.Done():
			return Sample{}, fmt.Errorf("probe page did not load")
		case <-time.After(100 * time.Millisecond):
		}
	}

	r, err = c.Call(ctx, sid, "Runtime.evaluate", map[string]any{"expression": probeJS, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		return Sample{}, err
	}
	var out evalReply
	if err := json.Unmarshal(r, &out); err != nil {
		return Sample{}, err
	}
	if len(out.ExceptionDetails) > 0 {
		return Sample{}, fmt.Errorf("probe threw: %s", out.ExceptionDetails)
	}
	var s Sample
	if err := json.Unmarshal(out.Result.Value, &s); err != nil {
		return Sample{}, fmt.Errorf("probe result: %w", err)
	}
	if !s.Secure {
		return Sample{}, fmt.Errorf("probe page is not a secure context")
	}
	return s, nil
}

// Compare returns "match" or "drift" and the names of differing fields. The
// eight baseline fields are compared with the previous sample; tz and langs
// are compared with the profile's current expected values instead (a proxy
// change legitimately changes them).
func Compare(baseline, got Sample, wantTZ, wantLangs string) (string, []string) {
	var diff []string
	if baseline.Canvas != got.Canvas {
		diff = append(diff, "canvas")
	}
	if baseline.WebGLVendor != got.WebGLVendor {
		diff = append(diff, "webglVendor")
	}
	if baseline.WebGLRenderer != got.WebGLRenderer {
		diff = append(diff, "webglRenderer")
	}
	if baseline.UA != got.UA {
		diff = append(diff, "ua")
	}
	if baseline.Platform != got.Platform {
		diff = append(diff, "platform")
	}
	if baseline.HWC != got.HWC {
		diff = append(diff, "hwc")
	}
	if baseline.Mem != got.Mem {
		diff = append(diff, "mem")
	}
	if baseline.Screen != got.Screen {
		diff = append(diff, "screen")
	}
	if netproxy.CanonicalZone(got.TZ) != netproxy.CanonicalZone(wantTZ) {
		diff = append(diff, "tz")
	}
	if got.Langs != wantLangs {
		diff = append(diff, "langs")
	}
	if len(diff) == 0 {
		return "match", nil
	}
	return "drift", diff
}
