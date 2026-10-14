// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Adopt runs at app startup. A browser that outlived a crashed app is taken
// back when its pid is alive, started at the recorded time, and still holds
// the profile's SingletonLock; its relay is rebuilt on the same port, because
// the browser's --proxy-server points there. Anything else is forgotten.
func (l *Launcher) Adopt() {
	launches, err := l.st.ListLaunches()
	if err != nil {
		l.log.Error("list launches", "err", err)
		return
	}
	running := map[string]bool{}
	defer func() {
		// Sessions of browsers that did not survive the app end now.
		if err := l.st.CloseStaleSessions(running); err != nil {
			l.log.Warn("close stale sessions", "err", err)
		}
	}()
	for _, ln := range launches {
		udd := l.dirs.Profile(ln.ProfileID)
		if !sameProcess(ln.PID, ln.PIDStartTime) || singletonPID(udd) != ln.PID {
			_ = l.st.DeleteLaunch(ln.ProfileID)
			continue
		}
		p := &proc{info: Info{ProfileID: ln.ProfileID, State: Running, PID: ln.PID, CDPPort: ln.CDPPort},
			done: make(chan struct{})}
		running[ln.ProfileID] = true
		if sess, ok, err := l.st.OpenSession(ln.ProfileID); err == nil && ok {
			p.sessionID = sess.ID
		}
		if ln.RelayPort > 0 {
			if err := l.adoptRelay(p, ln.ProfileID, ln.RelayPort); err != nil {
				// The browser fails closed (its proxy port is dead), it does not leak.
				p.info.Error = "proxy relay could not be restored: " + err.Error()
				l.log.Warn("adopt relay", "profile", ln.ProfileID, "err", err)
			}
		}
		l.mu.Lock()
		l.procs[ln.ProfileID] = p
		l.mu.Unlock()
		l.onChange(p.info)
		go l.watch(p, ln.PID, nil)
	}
}

func (l *Launcher) adoptRelay(p *proc, profileID string, port int) error {
	prof, err := l.st.GetProfile(profileID)
	if err != nil {
		return err
	}
	if prof.ProxyID == "" {
		return nil
	}
	up, err := l.upstream(prof.ProxyID)
	if err != nil {
		return err
	}
	p.relay, err = netproxy.StartRelay(up, port)
	return err
}
