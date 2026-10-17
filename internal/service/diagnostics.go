// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/store"
	"github.com/D0bby999/anyprofile/internal/update"
)

// diagLogLines is how much of the app log a diagnostics report carries.
const diagLogLines = 200

var (
	reEmail    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	reProxyUsr = regexp.MustCompile(`([a-z0-9]+://)?[^\s@/:"]+@([A-Za-z0-9.-]+:\d+)`)
	reIPv4     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reIPv6     = regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){3,7}[0-9a-fA-F]{1,4}\b`)
	reSecret   = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization|cookie)(["=: ]+)\S+`)
)

// Redact removes what a log line could hold about a person: e-mail
// addresses, proxy user names, IP addresses (the proxy's exit, but also the
// home one in a network error), anything named like a secret, and the home
// folder's name. Loopback addresses stay: they say which local port failed.
func Redact(s string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	s = reProxyUsr.ReplaceAllString(s, "${1}<user>@${2}") // before e-mail: user@host:port looks like one
	s = reEmail.ReplaceAllString(s, "<email>")
	s = reSecret.ReplaceAllString(s, "${1}${2}<hidden>")
	s = reIPv6.ReplaceAllString(s, "<ipv6>")
	return reIPv4.ReplaceAllStringFunc(s, func(ip string) string {
		if strings.HasPrefix(ip, "127.") || ip == "0.0.0.0" {
			return ip
		}
		return "<ip>"
	})
}

// Diagnostics is a plain-text report a user can paste into a bug report:
// versions, engines, counts, a few settings and the end of the app log, all
// passed through Redact. It names no profile, proxy or account.
func (s *Service) Diagnostics(ctx context.Context) string {
	var b strings.Builder
	fmt.Fprintf(&b, "AnyProfile %s\n", update.Current)
	fmt.Fprintf(&b, "OS: %s %s (%s), %d cores\n", runtime.GOOS, osVersion(), runtime.GOARCH, runtime.NumCPU())
	fmt.Fprintf(&b, "Time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "Disk encryption on: %v\n", FileVaultOn())
	ids := make([]string, 0, len(s.Engines))
	for id := range s.Engines {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, r := range s.Engines[id].Releases() {
			st := s.Engines[id].Status(ctx, r.Version)
			fmt.Fprintf(&b, "Engine %s %s (Chromium %s): installed=%v %s\n", id, r.Version, r.Chromium, st.Installed, st.Problem)
		}
	}
	fmt.Fprintf(&b, "Default engine: %s\n", s.DefaultEngine())
	if ps, err := s.Store.ListProfiles(store.ProfileFilter{}); err == nil {
		fmt.Fprintf(&b, "Profiles: %d, open: %d\n", len(ps), len(s.Launcher.List()))
	}
	if px, err := s.Store.ListProxies(); err == nil {
		fmt.Fprintf(&b, "Proxies: %d\n", len(px))
	}
	for _, k := range []string{"language", "on_open", "update_auto", "update_checked_at"} {
		v, _ := s.Store.GetSetting(k)
		fmt.Fprintf(&b, "Setting %s: %q\n", k, v)
	}
	fmt.Fprintf(&b, "\n--- last %d lines of anyprofile.log ---\n", diagLogLines)
	for _, l := range tail(filepath.Join(s.Dirs.Logs(), "anyprofile.log"), diagLogLines) {
		b.WriteString(Redact(l))
		b.WriteByte('\n')
	}
	return Redact(b.String())
}

// tail returns up to n last lines of path.
func tail(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{"(no log: " + err.Error() + ")"}
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}
