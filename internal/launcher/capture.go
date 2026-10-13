// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"github.com/D0bby999/anyprofile/internal/fingerprint"
	"github.com/D0bby999/anyprofile/internal/store"
)

// capture measures the browser once and records the verdict. It never fails
// the launch: any error is logged and recorded as "unchecked".
func (l *Launcher) capture(prof store.Profile, c *cdppipe.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	verdict, diff, baseline, last := "unchecked", []string(nil), "", ""
	if s, err := fingerprint.Capture(ctx, c); err != nil {
		l.log.Warn("fingerprint capture", "profile", prof.ID, "err", err)
	} else {
		b, _ := json.Marshal(s)
		last = string(b)
		prev, _ := l.st.FingerprintBaseline(prof.ID)
		var base fingerprint.Sample
		if prev == "" || json.Unmarshal([]byte(prev), &base) != nil {
			verdict, baseline = "baseline", last
		} else {
			verdict, diff = fingerprint.Compare(base, s, prof.Timezone, prof.Languages)
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
