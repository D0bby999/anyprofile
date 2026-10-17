// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package launcher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/store"
)

// A browser that signs in to itself logs the profile out of Google on the
// next launch: its token request fails (no real OAuth client), and the
// account reconcilor then clears the web sign-in to match. So the browser
// must never take part in Google sign-in; the web sign-in is all there is.
func TestApostateStaysOutOfChromeSignIn(t *testing.T) {
	r := newApostateRig(t)
	const id = "si1"
	if err := r.st.CreateProfile(&store.Profile{ID: id, Name: "Signin", Engine: "apostate",
		EngineVersion: apostate.Version, PersonaID: apostate.HostPersona, Platform: apostate.HostPlatform,
		SeedHex: "00000000005191e1", Timezone: "UTC", Languages: "en-US,en", UILocale: "en-US"}); err != nil {
		t.Fatal(err)
	}
	checkOutOfChromeSignIn(t, r, id)
}

func TestFuryStaysOutOfChromeSignIn(t *testing.T) {
	r := newRig(t)
	checkOutOfChromeSignIn(t, r, r.profile(t, 1))
}

func checkOutOfChromeSignIn(t *testing.T, r *rig, id string) {
	t.Helper()
	l := r.launcher(t)
	l.Capture = true // the browser pipe
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := l.StartBehind(ctx, id); err != nil {
		t.Fatal(err)
	}
	defer l.Stop(id)
	c, err := l.WaitConn(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": "chrome://signin-internals", "background": true})
	if err != nil {
		t.Fatal(err)
	}
	var tgt struct {
		TargetID string `json:"targetId"`
	}
	json.Unmarshal(raw, &tgt)
	if raw, err = c.Call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": tgt.TargetID, "flatten": true}); err != nil {
		t.Fatal(err)
	}
	var att struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(raw, &att)
	var text string
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(300 * time.Millisecond) {
		raw, err := c.Call(ctx, att.SessionID, "Runtime.evaluate", map[string]any{"expression": "document.body.innerText", "returnByValue": true})
		if err != nil {
			continue
		}
		var v struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		json.Unmarshal(raw, &v)
		if text = v.Result.Value; strings.Contains(text, "Account Consistency") {
			break
		}
	}
	t.Logf("signin-internals:\n%s", firstLines(text, 40))
	if !strings.Contains(text, "Account Consistency") {
		t.Fatal("chrome://signin-internals did not show its summary")
	}
	if line := lineWith(text, "Account Consistency"); !strings.Contains(line, "None") && !strings.Contains(line, "Disabled") {
		t.Fatalf("the browser takes part in Google sign-in: %q", line)
	}
}

func lineWith(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			if i+1 < len(lines) && !strings.Contains(l, "\t") {
				return l + " " + lines[i+1]
			}
			return l
		}
	}
	return ""
}

func firstLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	return strings.Join(lines[:min(n, len(lines))], "\n")
}
