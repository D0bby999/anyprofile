// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/fingerprint"
	"github.com/D0bby999/anyprofile/internal/store"
)

// capture measures the browser once and records the verdict. It never fails
// the launch: any error is logged and recorded as "unchecked".
func (l *Launcher) capture(prof store.Profile, eng engine.Engine, c *cdppipe.Conn, sessionID int64) {
	pl, ok := eng.(engine.PrimaryLanguageProbe)
	primaryOnly := ok && pl.ProbeSeesPrimaryLanguageOnly()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	verdict, diff, baseline, last := "unchecked", []string(nil), "", ""
	if s, err := fingerprint.Capture(ctx, c); err != nil {
		l.log.Warn("fingerprint capture", "profile", prof.ID, "err", err)
	} else {
		b, _ := json.Marshal(s)
		last = string(b)
		if sessionID > 0 {
			l.saveChecks(prof, eng, s, primaryOnly, sessionID)
		}
		prev, _ := l.st.FingerprintBaseline(prof.ID)
		var base fingerprint.Sample
		if prev == "" || json.Unmarshal([]byte(prev), &base) != nil {
			verdict, baseline = "baseline", last
		} else {
			verdict, diff = fingerprint.Compare(base, s, prof.Timezone, prof.Languages, primaryOnly)
		}
	}
	if err := l.st.SaveFingerprint(prof.ID, baseline, last, verdict, diff); err != nil {
		l.log.Warn("save fingerprint", "profile", prof.ID, "err", err)
	}
	l.log.Info("fingerprint", "profile", prof.ID, "verdict", verdict, "diff", strings.Join(diff, ","))
	if l.OnData != nil {
		l.OnData("profiles")
	}
}

// saveChecks records the consistency checks of this launch on its session.
func (l *Launcher) saveChecks(prof store.Profile, eng engine.Engine, s fingerprint.Sample, primaryOnly bool, sessionID int64) {
	e := fingerprint.Expect{Engine: prof.Engine, Timezone: prof.Timezone, Languages: prof.Languages, OS: prof.Platform,
		ChromiumMajor: fingerprint.MajorVersion(eng.Pinned().Chromium), Proxied: prof.ProxyID != "",
		PrimaryLangOnly: primaryOnly}
	if prof.Engine == "apostate" && prof.EngineMeta.HWC > 0 {
		e.HWC = min(prof.EngineMeta.HWC, runtime.NumCPU()) // the engine clamps to the host the same way
	}
	b, _ := json.Marshal(fingerprint.Checks(s, e))
	if err := l.st.SetSessionChecks(sessionID, b); err != nil {
		l.log.Warn("save checks", "profile", prof.ID, "err", err)
	}
}
