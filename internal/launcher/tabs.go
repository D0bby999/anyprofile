// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

// Chromium's own restore loses every tab when the window is closed before the
// browser quits: on macOS the browser outlives its last window and records
// that close. So the launcher keeps its own list of the open tabs, beside the
// profile, and reopens it when Chromium restored nothing.
const (
	tabsFile    = "AnyProfile Tabs.json"
	maxTabs     = 20
	tabsEvery   = 3 * time.Second
	restoreWait = 5 * time.Second
)

type targetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

func pageTargets(ctx context.Context, c *cdppipe.Conn) ([]targetInfo, error) {
	raw, err := c.Call(ctx, "", "Target.getTargets", nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		TargetInfos []targetInfo `json:"targetInfos"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(r.TargetInfos, func(t targetInfo) bool { return t.Type != "page" }), nil
}

// webTabs keeps the http(s) pages worth reopening; the fingerprint check's
// page (a .invalid host) and browser pages are left out.
func webTabs(ts []targetInfo) []string {
	var out []string
	for _, t := range ts {
		u, err := url.Parse(t.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || strings.HasSuffix(u.Hostname(), ".invalid") {
			continue
		}
		if out = append(out, t.URL); len(out) == maxTabs {
			break
		}
	}
	return out
}

func isNewTab(u string) bool {
	return strings.HasPrefix(u, "chrome://newtab") || strings.HasPrefix(u, "chrome://new-tab-page")
}

// readTabs returns the remembered tabs, filtered again: the file sits in a
// folder a restored snapshot or another tool may have written.
func readTabs(udd string) []string {
	var urls []string
	b, err := os.ReadFile(filepath.Join(udd, tabsFile))
	if err != nil || json.Unmarshal(b, &urls) != nil {
		return nil
	}
	ts := make([]targetInfo, len(urls))
	for i, u := range urls {
		ts[i] = targetInfo{Type: "page", URL: u}
	}
	return webTabs(ts)
}

func writeTabs(udd string, tabs []string) error {
	b, _ := json.Marshal(tabs)
	tmp := filepath.Join(udd, tabsFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(udd, tabsFile))
}

// keepTabs runs for the life of a launch. When Chromium had a session but
// brought back no web page, it opens the remembered tabs (or the start pages)
// in place of the empty new tab. Then it records the open tabs every few
// seconds; an empty list (every window closed) never replaces a full one.
func (l *Launcher) keepTabs(profileID, udd string, startURLs []string, hadSession bool, c *cdppipe.Conn) {
	if hadSession {
		l.reopenTabs(profileID, udd, startURLs, c)
	}
	saved := readTabs(udd)
	for {
		select {
		case <-c.Done():
			return
		case <-time.After(tabsEvery):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ts, err := pageTargets(ctx, c)
		cancel()
		if err != nil {
			continue
		}
		if tabs := webTabs(ts); len(tabs) > 0 && !slices.Equal(tabs, saved) {
			if err := writeTabs(udd, tabs); err != nil {
				l.log.Warn("save open tabs", "profile", profileID, "err", err)
				continue
			}
			saved = tabs
		}
	}
}

func (l *Launcher) reopenTabs(profileID, udd string, startURLs []string, c *cdppipe.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), restoreWait+5*time.Second)
	defer cancel()
	// The restore is synchronous at startup, but the first page can take a
	// moment to show up as a target.
	var ts []targetInfo
	for deadline := time.Now().Add(restoreWait); ; {
		var err error
		if ts, err = pageTargets(ctx, c); err == nil && len(ts) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(webTabs(ts)) > 0 {
		return // Chromium brought the tabs back itself
	}
	urls := readTabs(udd)
	if len(urls) == 0 {
		urls = startURLs
	}
	opened := 0
	for _, u := range urls {
		if _, err := c.Call(ctx, "", "Target.createTarget", map[string]any{"url": u}); err != nil {
			l.log.Warn("reopen tab", "profile", profileID, "err", err)
			continue
		}
		opened++
	}
	if opened == 0 {
		return
	}
	for _, t := range ts {
		if isNewTab(t.URL) {
			c.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": t.TargetID})
		}
	}
	l.log.Info("reopened tabs", "profile", profileID, "tabs", opened)
}
