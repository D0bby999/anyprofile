// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
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
	if p.sessionID > 0 {
		_ = l.st.EndSession(p.sessionID)
	}
	l.mu.Lock()
	delete(l.procs, p.info.ProfileID)
	l.mu.Unlock()
	l.set(p, Stopped, "")
	close(p.done)
	if l.AfterExit != nil {
		var ranFor time.Duration
		if !p.started.IsZero() {
			ranFor = time.Since(p.started)
		}
		l.AfterExit(p.info.ProfileID, ranFor)
	}
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
	pid, cdp := p.info.PID, p.cdp
	l.mu.Unlock()
	l.set(p, Stopping, "")
	askQuit(pid, cdp)
	select {
	case <-p.done:
		return nil
	case <-time.After(l.termGrace):
	}
	l.log.Warn("browser did not quit when asked; killing, unsaved data is lost", "profile", profileID, "pid", pid)
	killTree(pid)
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("browser did not exit")
	}
}

// closeOverPipe sends Browser.close: the browser saves its session and
// cookie jar and quits. It reports whether the browser took the request.
func closeOverPipe(cdp *cdppipe.Conn) bool {
	if cdp == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := cdp.Call(ctx, "", "Browser.close", nil)
	return err == nil
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
