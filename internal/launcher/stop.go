// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// watch waits for exit (wait is nil for an adopted process) and cleans up.
func (l *Launcher) watch(p *proc, pid int, wait <-chan error) {
	if wait != nil {
		<-wait
	} else {
		for alive(pid) {
			time.Sleep(time.Second)
		}
	}
	if p.relay != nil {
		p.relay.Close()
	}
	if p.cdp != nil {
		p.cdp.Close()
	}
	_ = l.st.DeleteLaunch(p.info.ProfileID)
	l.mu.Lock()
	delete(l.procs, p.info.ProfileID)
	l.mu.Unlock()
	l.set(p, Stopped, "")
	close(p.done)
}

// Stop asks the browser to quit and save its session, then kills its process
// group if it is still there after the grace period.
func (l *Launcher) Stop(profileID string) error {
	l.mu.Lock()
	p, ok := l.procs[profileID]
	if !ok || p.info.State != Running {
		l.mu.Unlock()
		if !ok {
			return nil
		}
		return fmt.Errorf("profile is %s", p.info.State)
	}
	pid := p.info.PID
	l.mu.Unlock()
	l.set(p, Stopping, "")
	// SIGHUP, not SIGTERM: from Chromium 155 a SIGTERM on macOS means "the
	// login session is ending" and exits without writing the cookie jar
	// (measured by the Fury authors and again here). SIGHUP takes the same
	// path as a menu Quit. Main process only; it shuts its helpers down.
	unix.Kill(pid, unix.SIGHUP)
	select {
	case <-p.done:
		return nil
	case <-time.After(l.termGrace):
	}
	l.log.Warn("browser did not quit when asked; killing, unsaved data is lost", "profile", profileID, "pid", pid)
	signalGroup(pid, unix.SIGKILL)
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("browser did not exit")
	}
}

// Focus brings a running profile's windows to the front.
func (l *Launcher) Focus(profileID string) bool {
	l.mu.Lock()
	p, ok := l.procs[profileID]
	l.mu.Unlock()
	return ok && p.info.State == Running && activate(p.info.PID)
}

// StopAll stops every running browser in parallel.
func (l *Launcher) StopAll() {
	var wg sync.WaitGroup
	for _, in := range l.List() {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _ = l.Stop(id) }(in.ProfileID)
	}
	wg.Wait()
}
