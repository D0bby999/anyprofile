// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// leaveAgain is how soon a second try to leave a page that warned about
// unsaved changes counts as meaning it.
const leaveAgain = 2 * time.Minute

// answerDialog answers a browser dialog the way a person working through a
// task does, and notes it for the next tool result. Unanswered, the dialog
// blocks the page and every call after it.
//
//   - alert: OK.
//   - confirm: OK. It follows the agent's own click (Delete, Submit…), so
//     the agent meant it.
//   - prompt: Cancel; the agent types into the page instead.
//   - "Leave site?" (beforeunload): Stay the first time, since unsaved work
//     would be lost; asked to leave again within two minutes, Leave.
func (b *Browser) answerDialog(t *tab, session string, raw json.RawMessage) {
	var d struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	json.Unmarshal(raw, &d)
	accept, note := true, ""
	b.mu.Lock()
	switch d.Type {
	case "alert":
		note = fmt.Sprintf("the page showed an alert %q; closed with OK", d.Message)
	case "confirm":
		note = fmt.Sprintf("the page asked to confirm %q; answered OK", d.Message)
	case "prompt":
		accept = false
		note = fmt.Sprintf("the page asked for text %q; cancelled, type into the page instead", d.Message)
	case "beforeunload":
		if time.Since(t.leaveWarned) < leaveAgain {
			note = "the page warned about unsaved changes; left, as asked again"
			t.leaveWarned = time.Time{}
		} else {
			accept = false
			t.leaveWarned, t.stayed = time.Now(), true
			note = "the page warned that unsaved changes would be lost; stayed. Save first, or ask again within 2 minutes to leave anyway"
		}
	default:
		note = fmt.Sprintf("the page showed a %s dialog %q; answered OK", d.Type, d.Message)
	}
	if t.dialog != "" {
		t.dialog += "; "
	}
	t.dialog += note
	b.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.conn.Call(ctx, session, "Page.handleJavaScriptDialog", map[string]any{"accept": accept})
}
