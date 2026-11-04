// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

func (l *Launcher) start(ctx context.Context, p *proc) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l.BeforeStart != nil {
		if err := l.BeforeStart(p.info.ProfileID); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	prof, err := l.st.GetProfile(p.info.ProfileID)
	if err != nil {
		return err
	}
	if prof.DeletedAt != "" {
		return errors.New("profile is in the trash")
	}
	eng, ok := l.engines[prof.Engine]
	if !ok {
		return fmt.Errorf("engine %q is not available", prof.Engine)
	}
	if err := l.pickVersion(ctx, eng, &prof); err != nil {
		return err
	}
	udd := l.dirs.Profile(prof.ID)
	if err := os.MkdirAll(udd, 0o700); err != nil {
		return err
	}
	if pid := singletonPID(udd); pid != 0 && alive(pid) {
		return apperr.Newf("folder_in_use", "another process (pid %d) already uses this profile's folder; close it first", pid)
	}
	// The browser reads the cookie jar as soon as it starts, so the last copy
	// goes back before that (see cookieguard.go).
	l.cookieGuard(prof, udd)

	var up netproxy.Upstream // kept in memory only, to measure the exit after launch
	if prof.ProxyID != "" {
		if up, err = l.upstream(prof.ProxyID); err != nil {
			return err
		}
		if p.relay, err = netproxy.StartRelay(up, 0); err != nil {
			return fmt.Errorf("start proxy relay: %w", err)
		}
	}

	var key []byte
	if eng.UsesCookieKey() {
		if key, err = cookieKey(l.sec, prof.ID, udd); err != nil {
			return err
		}
		defer secrets.Wipe(key)
	}
	rt := engine.Runtime{UserDataDir: udd, CDP: prof.CDPNextLaunch, CDPPipe: l.Capture, CookieKey: key}
	if p.relay != nil {
		rt.RelayPort = p.relay.Port()
	}
	// By default the start pages open every time. With "restore" Chromium
	// brings back the last session, and the start pages open only when there
	// is none yet.
	onOpen, _ := l.st.GetSetting(store.SettingOnOpen)
	rt.FreshStart = onOpen != "restore"
	hadSession := hasSession(udd)
	for _, id := range prof.Extensions {
		if dir := l.dirs.Extension(id); isDir(dir) {
			rt.Extensions = append(rt.Extensions, dir)
		}
	}
	if urls := prof.StartURLs(); len(urls) > 0 && (rt.FreshStart || !hadSession) {
		rt.StartURLs = urls
	}
	if err := eng.PrepareProfile(prof, rt); err != nil {
		l.log.Warn("prepare profile", "profile", prof.ID, "err", err)
	}
	spec, err := eng.BuildLaunch(ctx, prof, rt)
	if err != nil {
		return err
	}
	if spec.Warning != "" && l.log != nil {
		l.log.Warn("profile identity clamped for this machine", "profile", prof.ID, "detail", spec.Warning)
	}
	spec.Args = append(spec.Args, l.TestArgs...)
	// The first window a profile ever shows is the one nobody sized yet.
	// Chromium restores the bounds of every later one, so the user's own size
	// and position are never overridden.
	if prof.LastOpenAt == "" && l.Screen != nil {
		if area, ok := l.Screen(); ok {
			r := firstWindow(area, l.opening())
			spec.Args = append(spec.Args,
				fmt.Sprintf("--window-size=%d,%d", r.W, r.H),
				fmt.Sprintf("--window-position=%d,%d", r.X, r.Y))
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	front := frontPID()
	pid, wait, conn, err := spawn(spec, udd, filepath.Join(l.dirs.Logs(), "browser-"+prof.ID+".log"))
	if err != nil {
		return err
	}
	started := false
	defer func() {
		if started {
			return
		}
		if conn != nil {
			conn.Close()
		}
		killTree(pid)
		_ = l.st.DeleteLaunch(prof.ID)
		if p.sessionID > 0 {
			_ = l.st.EndSession(p.sessionID)
		}
	}()
	if p.behind {
		epoch := l.focusEpoch.Load()
		go l.keepBehind(pid, front, 6*time.Second, epoch)
	}
	p.cdp = conn
	if err := l.waitReady(ctx, udd, pid, wait); err != nil {
		return err
	}
	startTime, err := pidStartTime(pid)
	if err != nil {
		return err
	}
	cdp := 0
	if rt.CDP {
		cdp = readDevToolsPort(udd, 5*time.Second)
	}
	if err := l.st.PutLaunch(store.Launch{ProfileID: prof.ID, PID: pid, PIDStartTime: startTime,
		RelayPort: rt.RelayPort, CDPPort: cdp}); err != nil {
		return err
	}
	_ = l.st.MarkOpened(prof.ID)
	sess := store.Session{ProfileID: prof.ID, Engine: prof.Engine, EngineVersion: prof.EngineVersion, ProxyID: prof.ProxyID}
	if err := l.st.StartSession(&sess); err != nil {
		l.log.Warn("start session", "profile", prof.ID, "err", err)
	}
	p.sessionID = sess.ID
	if prof.ProxyID != "" && l.CheckExit != nil && sess.ID > 0 {
		go l.measureExit(prof.ID, sess.ID, up)
	}
	if prof.CDPNextLaunch { // diagnostics are one launch only
		prof.CDPNextLaunch = false
		_ = l.st.UpdateProfile(prof)
	}
	l.mu.Lock()
	if err := ctx.Err(); err != nil {
		l.mu.Unlock()
		return err
	}
	p.info.PID, p.info.CDPPort, p.started = pid, cdp, time.Now()
	p.info.State = Running
	info := p.info
	started = true
	l.mu.Unlock()
	l.onChange(info)
	if !p.behind {
		// A window the user asked for comes to the front. It is raised only
		// once it exists: asking while Chromium is still starting makes the
		// app active without a window, and the window that appears afterwards
		// stays behind this one — it opens, and nothing on it answers until
		// the user raises it by hand.
		go l.bringFront(pid)
	}
	if p.cdp != nil {
		go l.capture(prof, eng, p.cdp, p.sessionID)
		go l.quitWithLastWindow(prof.ID, p.cdp)
		if !rt.FreshStart {
			go l.keepTabs(prof.ID, udd, prof.StartURLs(), hadSession, p.cdp)
		}
	}
	go l.watch(p, pid, wait)
	return nil
}

func (l *Launcher) upstream(proxyID string) (netproxy.Upstream, error) {
	px, err := l.st.GetProxy(proxyID)
	if err != nil {
		return netproxy.Upstream{}, fmt.Errorf("proxy: %w", err)
	}
	up := netproxy.Upstream{Scheme: px.Scheme, Host: px.Host, Port: px.Port, Username: px.Username}
	pw, err := l.sec.Get(secrets.ServiceProxy, proxyID)
	switch {
	case err == nil:
		up.Password = string(pw)
	case !errors.Is(err, secrets.ErrNotFound):
		return up, fmt.Errorf("read proxy password: %w", err)
	}
	return up, nil
}

// waitReady returns once the browser holds its SingletonLock, or fails if it exits first.
func (l *Launcher) waitReady(ctx context.Context, udd string, pid int, wait <-chan error) error {
	deadline := time.Now().Add(l.readyTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-wait:
			return fmt.Errorf("browser exited during startup (%v); see logs/browser-*.log", err)
		case <-time.After(100 * time.Millisecond):
		}
		if singletonPID(udd) == pid {
			return nil
		}
	}
	return errors.New("browser did not become ready in time")
}

// measureExit records the exit the proxy gives right after a launch. It never
// fails the launch; an error is stored on the session for the user to see.
func (l *Launcher) measureExit(profileID string, sessionID int64, up netproxy.Upstream) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	e, err := l.CheckExit(ctx, up)
	msg := ""
	if err != nil {
		msg = err.Error()
		l.log.Warn("exit check", "profile", profileID, "err", msg)
	}
	if err := l.st.SetSessionExit(sessionID, store.Session{ExitIP: e.IP, ExitCountry: e.Country, ExitTZ: e.Timezone,
		ExitError: msg, ExitIP6: e.IPv6, ExitTZ6: e.IPv6Timezone}); err != nil {
		l.log.Warn("save exit", "profile", profileID, "err", err)
	}
	if l.OnData != nil {
		l.OnData("profiles")
	}
}

// bringWait is how long a browser is given to show its window before giving up
// on raising it; a cold start on a busy computer is the slow case.
const bringWait = 6 * time.Second

// bringFront raises the window the user asked for, once Chromium has created
// it, and checks that it is really the app in front. Asking before the window
// exists makes the app active with nothing to show, and the window that
// appears afterwards stays behind this one: it looks open, and nothing on it
// answers until the user raises it by hand. The first request is always made —
// the user asked for this window — and it is only repeated while this app is
// still the one in front.
func (l *Launcher) bringFront(pid int) {
	if frontPID() == 0 { // the platform does not report the front app
		activate(pid)
		return
	}
	epoch := l.focusEpoch.Load()
	// Chromium creates the window a moment after it is ready to launch.
	deadline := time.Now().Add(bringWait)
	for !onCurrentDesktop(pid) { // not on this desktop yet
		if time.Now().After(deadline) || l.focusEpoch.Load() != epoch {
			// No window here: another desktop, or none at all. One request is
			// still the right answer — it is the window the user asked for.
			l.log.Warn("browser window is not on this desktop", "pid", pid)
			activate(pid)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for attempt := 0; attempt < 5; attempt++ {
		if l.focusEpoch.Load() != epoch {
			return
		}
		// The first request is always worth making: the user asked for this
		// window. Later ones are only repeated while this app is still the
		// one in front — once the user is in another app, it is theirs.
		activate(pid)
		for settled := time.Now().Add(300 * time.Millisecond); time.Now().Before(settled); time.Sleep(20 * time.Millisecond) {
			switch f := frontPID(); {
			case f == pid:
				l.log.Info("browser window raised", "pid", pid, "attempt", attempt+1)
				return // the window is in front: done
			case attempt > 0 && f != 0 && f != os.Getpid():
				return // the user is in another app: leave it alone
			}
		}
	}
	l.log.Warn("browser window stayed behind", "pid", pid)
}

// keepBehind puts the app the user is in back in front whenever the new
// browser takes the focus while starting up, for d. Chromium activates
// itself more than once (at launch, then as its first window shows); after
// d, a click on the browser is the user's own. epoch is the focus the user
// asked for when this browser started: once a later action asks for another
// window (tiling what was just opened, say), fighting for the old one would
// undo it, so the loop stops.
func (l *Launcher) keepBehind(pid, front int, d time.Duration, epoch int64) {
	if front == 0 || front == pid {
		return
	}
	end := time.Now().Add(d)
	for range 5 { // a browser that keeps the focus is not fought forever
		if l.focusEpoch.Load() != epoch {
			return // the user has asked for another window since
		}
		if !waitFront(pid, true, time.Until(end)) {
			return
		}
		l.log.Info("browser took the focus", "pid", pid, "back", stepBack(pid, front))
	}
}

// Behind is agent.Hub.Behind: when the browser comes in front of the app the
// user is in during the call it wraps, that app is put back in front.
func (l *Launcher) Behind(profileID string) func() {
	in, ok := l.Info(profileID)
	front := frontPID()
	if !ok || in.PID == 0 || front == 0 || front == in.PID {
		return func() {} // the user is looking at this browser, or we cannot tell
	}
	return func() {
		if waitFront(in.PID, true, 400*time.Millisecond) {
			l.log.Info("browser took the focus", "pid", in.PID, "back", stepBack(in.PID, front))
		}
	}
}

// stepBack gives the focus back from the browser pid to front, the app the
// user was in. When that is AnyProfile (Open clicked in the app), it simply
// activates itself. Another app cannot be asked to activate since macOS 14
// unless we are active ourselves; hiding the browser makes macOS activate
// some other app, and showing it again right away does not take the focus
// back. That app is not always front: it can be a full-screen app on
// another desktop, which is why AnyProfile never hides for its own Open.
func stepBack(pid, front int) bool {
	if front == os.Getpid() {
		activateSelf()
		return waitFront(pid, false, 500*time.Millisecond)
	}
	setHidden(pid, true)
	ok := waitFront(pid, false, 500*time.Millisecond)
	setHidden(pid, false)
	return ok && waitFront(pid, false, 150*time.Millisecond)
}

// waitFront waits until pid is (or is no longer) the front app.
func waitFront(pid int, is bool, d time.Duration) bool {
	for end := time.Now().Add(d); ; time.Sleep(15 * time.Millisecond) {
		if (frontPID() == pid) == is {
			return true
		}
		if time.Now().After(end) {
			return false
		}
	}
}
