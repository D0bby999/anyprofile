// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

// On macOS a browser outlives its last window. Here closing that window
// means the user is done, so the launcher quits the browser the way Stop
// does (cookies saved) and the app shows the profile closed.
//
// The browser must not quit while it is the app in front: macOS would
// activate an app of its own choosing, maybe a full-screen one on another
// desktop, and the screen jumped there. So AnyProfile takes the focus first,
// and the browser quits behind it. Left in front if AnyProfile cannot take
// it, the browser quits once the user clicks anything else.
const (
	windowsEvery = time.Second
	// Pages seen gone this many checks in a row: a script that closes every
	// tab and opens a new one is not taken for a closed window.
	emptyChecks = 2
)

// leaveBrowser gives the focus to AnyProfile when the browser pid is in
// front, and reports whether the browser is out of front; tests replace it.
var leaveBrowser = func(pid int) bool {
	if pid == 0 || frontPID() != pid {
		return true
	}
	activateSelf()
	return waitFront(pid, false, time.Second)
}

func (l *Launcher) quitWithLastWindow(profileID string, c *cdppipe.Conn) {
	seen, empty := false, 0
	for {
		select {
		case <-c.Done():
			return
		case <-time.After(windowsEvery):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ts, err := pageTargets(ctx, c)
		cancel()
		if err != nil {
			continue
		}
		if len(ts) > 0 {
			seen, empty = true, 0
			continue
		}
		if !seen {
			continue // still starting: no window yet
		}
		if empty++; empty < emptyChecks {
			continue
		}
		if in, ok := l.Info(profileID); ok && !leaveBrowser(in.PID) {
			continue // quit once the user has moved to another app
		}
		l.log.Info("last window closed; quitting the browser", "profile", profileID)
		_ = l.Stop(profileID) // fails only when a Stop is already under way
		return
	}
}
