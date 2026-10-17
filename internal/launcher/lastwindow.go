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
// It waits until the browser is no longer the app in front. A front app
// that quits makes macOS activate the app used before it, and switch to
// that app's desktop (Space) when it is on another one: the screen jumped
// away a second after the user closed the window. Left in front without a
// window, the browser quits as soon as the user clicks anything else, and
// nothing moves.
const (
	windowsEvery = time.Second
	// Pages seen gone this many checks in a row: a script that closes every
	// tab and opens a new one is not taken for a closed window.
	emptyChecks = 2
)

// inFront reports whether pid is the app in front; tests replace it.
var inFront = func(pid int) bool { return pid != 0 && frontPID() == pid }

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
		if in, ok := l.Info(profileID); ok && inFront(in.PID) {
			continue // quit once the user has moved to another app
		}
		l.log.Info("last window closed; quitting the browser", "profile", profileID)
		_ = l.Stop(profileID) // fails only when a Stop is already under way
		return
	}
}
