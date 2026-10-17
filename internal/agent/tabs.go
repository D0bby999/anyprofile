// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Tab is one page of the browser as the agent sees it. ID is a short prefix of
// the target id; any unambiguous prefix of 4 or more characters works.
type Tab struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Current bool   `json:"current"`
}

type targetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Title    string `json:"title"`
}

const shortID = 8

func short(targetID string) string {
	if len(targetID) > shortID {
		return targetID[:shortID]
	}
	return targetID
}

// pages lists the page targets, leaving out the fingerprint check's own page
// (a .invalid host).
func (b *Browser) pages(ctx context.Context) ([]targetInfo, error) {
	var r struct {
		TargetInfos []targetInfo `json:"targetInfos"`
	}
	if err := b.call(ctx, "", "Target.getTargets", nil, &r); err != nil {
		return nil, err
	}
	out := r.TargetInfos[:0]
	for _, t := range r.TargetInfos {
		if t.Type != "page" {
			continue
		}
		if u, err := url.Parse(t.URL); err == nil && strings.HasSuffix(u.Hostname(), ".invalid") {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// Tabs lists the tabs, marking the one tools use when no tab is given.
func (b *Browser) Tabs(ctx context.Context) ([]Tab, error) {
	b.touch()
	ps, err := b.pages(ctx)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	cur := b.current
	b.mu.Unlock()
	known := false
	for _, p := range ps {
		known = known || p.TargetID == cur
	}
	if !known && len(ps) > 0 {
		// Nothing used yet: name the tab tools would pick (the one on screen).
		if t, err := b.tab(ctx, ""); err == nil {
			cur = t.targetID
		}
	}
	out := make([]Tab, 0, len(ps))
	for _, p := range ps {
		out = append(out, Tab{ID: short(p.TargetID), URL: p.URL, Title: p.Title, Current: p.TargetID == cur})
	}
	return out, nil
}

// tab finds the tab a tool works on and attaches to it: the one asked for, else
// the one used last, else the one on screen, else the first.
func (b *Browser) tab(ctx context.Context, id string) (*tab, error) {
	b.touch()
	ps, err := b.pages(ctx)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, apperr.New("agent_no_tab", "the browser has no tab open; use open_tab")
	}
	var pick string
	switch {
	case id != "":
		if len(id) < 4 {
			return nil, apperr.New("agent_tab_not_found", "give at least 4 characters of the tab id; see list_tabs")
		}
		for _, p := range ps {
			if strings.HasPrefix(p.TargetID, id) {
				if pick != "" {
					return nil, apperr.Newf("agent_tab_not_found", "tab id %q matches more than one tab; see list_tabs", id)
				}
				pick = p.TargetID
			}
		}
		if pick == "" {
			return nil, apperr.Newf("agent_tab_not_found", "no tab %q; see list_tabs", id)
		}
	default:
		b.mu.Lock()
		cur := b.current
		b.mu.Unlock()
		for _, p := range ps {
			if p.TargetID == cur {
				pick = cur
			}
		}
		if pick == "" {
			pick = ps[0].TargetID
			for _, p := range ps {
				t, err := b.attach(ctx, p.TargetID)
				if err != nil {
					continue
				}
				var vis string
				if b.evalJSON(ctx, t, "document.visibilityState", &vis) == nil && vis == "visible" {
					pick = p.TargetID
					break
				}
			}
		}
	}
	b.forget(ps)
	t, err := b.attach(ctx, pick)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.current = pick
	b.mu.Unlock()
	return t, nil
}

// forget drops tabs that closed without a detach event reaching us.
func (b *Browser) forget(ps []targetInfo) {
	live := map[string]bool{}
	for _, p := range ps {
		live[p.TargetID] = true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for id := range b.tabs {
		if !live[id] {
			delete(b.tabs, id)
		}
	}
}

// attach opens a flat session to the page and starts its network log.
func (b *Browser) attach(ctx context.Context, targetID string) (*tab, error) {
	b.mu.Lock()
	t := b.tabs[targetID]
	b.mu.Unlock()
	if t != nil {
		return t, nil
	}
	var r struct {
		SessionID string `json:"sessionId"`
	}
	if err := b.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": targetID, "flatten": true}, &r); err != nil {
		return nil, err
	}
	t = &tab{targetID: targetID, session: r.SessionID, net: newNetLog(maxRequests)}
	b.mu.Lock()
	b.tabs[targetID] = t
	b.mu.Unlock()
	// Network events start the log; Page and Runtime stay off, which the
	// page could notice.
	if err := b.call(ctx, t.session, "Network.enable", map[string]any{}, nil); err != nil {
		return nil, err
	}
	return t, nil
}

// activate brings the tab to the front, so input and screenshots reach it.
func (b *Browser) activate(ctx context.Context, t *tab) error {
	return b.call(ctx, "", "Target.activateTarget", map[string]any{"targetId": t.targetID}, nil)
}

// OpenTab opens url in a new tab, which becomes the current one.
func (b *Browser) OpenTab(ctx context.Context, rawURL string) (Tab, error) {
	b.touch()
	if err := checkURL(rawURL); err != nil {
		return Tab{}, err
	}
	var r struct {
		TargetID string `json:"targetId"`
	}
	if err := b.call(ctx, "", "Target.createTarget", map[string]any{"url": rawURL}, &r); err != nil {
		return Tab{}, err
	}
	t, err := b.attach(ctx, r.TargetID)
	if err != nil {
		return Tab{}, err
	}
	b.mu.Lock()
	b.current = r.TargetID
	b.mu.Unlock()
	b.waitLoaded(ctx, t, "")
	return b.describe(ctx, t), nil
}

// CloseTab closes a tab; the browser stays open.
func (b *Browser) CloseTab(ctx context.Context, id string) error {
	t, err := b.tab(ctx, id)
	if err != nil {
		return err
	}
	if err := b.call(ctx, "", "Target.closeTarget", map[string]any{"targetId": t.targetID}, nil); err != nil {
		return err
	}
	b.mu.Lock()
	delete(b.tabs, t.targetID)
	if b.current == t.targetID {
		b.current = ""
	}
	b.mu.Unlock()
	return nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return apperr.New("agent_bad_url", "only http and https addresses can be opened")
	}
	return nil
}

// Navigate goes to url, or does action: "back", "forward" or "reload". It
// waits up to 30 s for the page to load.
func (b *Browser) Navigate(ctx context.Context, id, rawURL, action string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	loader := ""
	switch {
	case rawURL != "":
		if err := checkURL(rawURL); err != nil {
			return Tab{}, err
		}
		var r struct {
			LoaderID  string `json:"loaderId"`
			ErrorText string `json:"errorText"`
		}
		if err := b.call(ctx, t.session, "Page.navigate", map[string]any{"url": rawURL}, &r); err != nil {
			return Tab{}, err
		}
		if r.ErrorText != "" {
			return Tab{}, apperr.Newf("agent_navigate", "the page did not load: %s", r.ErrorText)
		}
		loader = r.LoaderID
	case action == "reload":
		if err := b.call(ctx, t.session, "Page.reload", map[string]any{}, nil); err != nil {
			return Tab{}, err
		}
	case action == "back" || action == "forward":
		var h struct {
			CurrentIndex int `json:"currentIndex"`
			Entries      []struct {
				ID int `json:"id"`
			} `json:"entries"`
		}
		if err := b.call(ctx, t.session, "Page.getNavigationHistory", nil, &h); err != nil {
			return Tab{}, err
		}
		i := h.CurrentIndex - 1
		if action == "forward" {
			i = h.CurrentIndex + 1
		}
		if i < 0 || i >= len(h.Entries) {
			return Tab{}, apperr.Newf("agent_navigate", "there is no page to go %s to", action)
		}
		if err := b.call(ctx, t.session, "Page.navigateToHistoryEntry", map[string]any{"entryId": h.Entries[i].ID}, nil); err != nil {
			return Tab{}, err
		}
	default:
		return Tab{}, apperr.New("agent_navigate", "give a url, or action back, forward or reload")
	}
	if loader == "" {
		time.Sleep(300 * time.Millisecond) // let the old document go first
	}
	b.waitLoaded(ctx, t, loader)
	return b.describe(ctx, t), nil
}

type frame struct {
	ID       string `json:"id"`
	LoaderID string `json:"loaderId"`
	URL      string `json:"url"`
}

func (b *Browser) mainFrame(ctx context.Context, t *tab) (frame, error) {
	var r struct {
		FrameTree struct {
			Frame frame `json:"frame"`
		} `json:"frameTree"`
	}
	err := b.call(ctx, t.session, "Page.getFrameTree", nil, &r)
	return r.FrameTree.Frame, err
}

// waitLoaded polls until the document (of loader, when given) is complete, at
// most 30 s; a page that keeps loading is left as it is.
func (b *Browser) waitLoaded(ctx context.Context, t *tab, loader string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		f, err := b.mainFrame(ctx, t)
		if err == nil && (loader == "" || f.LoaderID == loader) {
			var state string
			if b.evalJSON(ctx, t, "document.readyState", &state) == nil && state == "complete" {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (b *Browser) describe(ctx context.Context, t *tab) Tab {
	var v struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	b.evalJSON(ctx, t, "({url: location.href, title: document.title})", &v)
	return Tab{ID: short(t.targetID), URL: v.URL, Title: v.Title, Current: true}
}

// world returns the isolated world of the tab's current document, making one
// when the document changed. Scripts of the page cannot see it.
func (b *Browser) world(ctx context.Context, t *tab) (int, error) {
	f, err := b.mainFrame(ctx, t)
	if err != nil {
		return 0, err
	}
	b.mu.Lock()
	if t.worldLoader == f.LoaderID && t.worldID != 0 {
		id := t.worldID
		b.mu.Unlock()
		return id, nil
	}
	b.mu.Unlock()
	var r struct {
		ExecutionContextID int `json:"executionContextId"`
	}
	if err := b.call(ctx, t.session, "Page.createIsolatedWorld", map[string]any{"frameId": f.ID, "worldName": "anyprofile"}, &r); err != nil {
		return 0, err
	}
	b.mu.Lock()
	t.worldLoader, t.worldID = f.LoaderID, r.ExecutionContextID
	b.mu.Unlock()
	return r.ExecutionContextID, nil
}

type evalReply struct {
	Result struct {
		Value json.RawMessage `json:"value"`
	} `json:"result"`
	ExceptionDetails *struct {
		Text      string `json:"text"`
		Exception struct {
			Description string `json:"description"`
		} `json:"exception"`
	} `json:"exceptionDetails"`
}

// evalJSON runs expr in the isolated world and decodes its value into out.
func (b *Browser) evalJSON(ctx context.Context, t *tab, expr string, out any) error {
	for try := 0; try < 2; try++ {
		id, err := b.world(ctx, t)
		if err != nil {
			return err
		}
		var r evalReply
		err = b.call(ctx, t.session, "Runtime.evaluate", map[string]any{
			"expression": expr, "contextId": id, "returnByValue": true, "awaitPromise": true,
		}, &r)
		if err != nil && strings.Contains(err.Error(), "context") && try == 0 {
			// The world went with a same-loader reload; make a new one.
			b.mu.Lock()
			t.worldID = 0
			b.mu.Unlock()
			continue
		}
		if err != nil {
			return err
		}
		if r.ExceptionDetails != nil {
			return fmt.Errorf("page script: %s %s", r.ExceptionDetails.Text, r.ExceptionDetails.Exception.Description)
		}
		if out == nil || len(r.Result.Value) == 0 {
			return nil
		}
		return json.Unmarshal(r.Result.Value, out)
	}
	return fmt.Errorf("page script: no context")
}

// WaitFor waits until text shows on the page (or, with gone, until it is no
// longer there), or until the address contains urlPart; at most timeout.
func (b *Browser) WaitFor(ctx context.Context, id, text string, gone bool, urlPart string, timeout time.Duration) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if text == "" && urlPart == "" {
		return Tab{}, apperr.New("agent_wait", "give text or url to wait for")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		var v struct {
			Text string `json:"t"`
			URL  string `json:"u"`
		}
		if b.evalJSON(ctx, t, "({t: document.body ? document.body.innerText : '', u: location.href})", &v) == nil {
			ok := true
			if text != "" {
				ok = strings.Contains(v.Text, text) != gone
			}
			if urlPart != "" {
				ok = ok && strings.Contains(v.URL, urlPart)
			}
			if ok {
				return b.describe(ctx, t), nil
			}
		}
		select {
		case <-ctx.Done():
			return Tab{}, apperr.Newf("agent_wait", "still not there after %s", timeout)
		case <-time.After(400 * time.Millisecond):
		}
	}
}
