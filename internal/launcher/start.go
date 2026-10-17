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
	if l.BeforeStart != nil {
		if err := l.BeforeStart(p.info.ProfileID); err != nil {
			return err
		}
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
	if st := eng.Status(ctx); !st.Installed {
		return apperr.Newf("engine_missing", "engine %s %s is not installed", prof.Engine, st.Version.Version)
	}
	udd := l.dirs.Profile(prof.ID)
	if err := os.MkdirAll(udd, 0o700); err != nil {
		return err
	}
	if pid := singletonPID(udd); pid != 0 && alive(pid) {
		return apperr.Newf("folder_in_use", "another process (pid %d) already uses this profile's folder; close it first", pid)
	}

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
	front := frontPID()
	pid, wait, conn, err := spawn(spec, udd, filepath.Join(l.dirs.Logs(), "browser-"+prof.ID+".log"))
	if err != nil {
		return err
	}
	if p.behind {
		go l.keepBehind(pid, front, 6*time.Second)
	}
	p.cdp = conn
	if err := l.waitReady(udd, pid, wait); err != nil {
		if p.cdp != nil {
			p.cdp.Close()
		}
		killTree(pid)
		return err
	}
	startTime, err := pidStartTime(pid)
	if err != nil {
		killTree(pid)
		return err
	}
	cdp := 0
	if rt.CDP {
		cdp = readDevToolsPort(udd, 5*time.Second)
	}
	if err := l.st.PutLaunch(store.Launch{ProfileID: prof.ID, PID: pid, PIDStartTime: startTime,
		RelayPort: rt.RelayPort, CDPPort: cdp}); err != nil {
		killTree(pid)
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
	p.info.PID, p.info.CDPPort, p.started = pid, cdp, time.Now()
	l.mu.Unlock()
	l.set(p, Running, "")
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
func (l *Launcher) waitReady(udd string, pid int, wait <-chan error) error {
	deadline := time.Now().Add(l.readyTimeout)
	for time.Now().Before(deadline) {
		select {
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

// keepBehind puts the app the user is in back in front if the new browser
// takes the focus while starting up (Chromium activates itself once its
// window shows). Once is enough; after that, a click on the browser is the
// user's own.
func (l *Launcher) keepBehind(pid, front int, d time.Duration) {
	if front == 0 || front == pid {
		return
	}
	if waitFront(pid, true, d) {
		l.log.Info("browser took the focus", "pid", pid, "back", stepBack(pid))
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
			l.log.Info("browser took the focus", "pid", in.PID, "back", stepBack(in.PID))
		}
	}
}

// stepBack moves the focus off the browser pid. Asking the other app to
// activate is refused since macOS 14 unless we are active ourselves; hiding
// the browser makes macOS activate the app that was below it, and showing it
// again right away does not take the focus back.
func stepBack(pid int) bool {
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
