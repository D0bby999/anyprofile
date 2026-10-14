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
	"golang.org/x/sys/unix"
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
	// "start" opens the start pages every time; otherwise Chromium restores the
	// last session, and the start pages open only when there is none yet.
	onOpen, _ := l.st.GetSetting(store.SettingOnOpen)
	rt.FreshStart = onOpen == "start"
	hadSession := hasSession(udd)
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
	pid, wait, conn, err := spawn(spec, udd, filepath.Join(l.dirs.Logs(), "browser-"+prof.ID+".log"))
	if err != nil {
		return err
	}
	p.cdp = conn
	if err := l.waitReady(udd, pid, wait); err != nil {
		if p.cdp != nil {
			p.cdp.Close()
		}
		signalGroup(pid, unix.SIGKILL)
		return err
	}
	startTime, err := pidStartTime(pid)
	if err != nil {
		signalGroup(pid, unix.SIGKILL)
		return err
	}
	cdp := 0
	if rt.CDP {
		cdp = readDevToolsPort(udd, 5*time.Second)
	}
	if err := l.st.PutLaunch(store.Launch{ProfileID: prof.ID, PID: pid, PIDStartTime: startTime,
		RelayPort: rt.RelayPort, CDPPort: cdp}); err != nil {
		signalGroup(pid, unix.SIGKILL)
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
	if err := l.st.SetSessionExit(sessionID, e.IP, e.Country, e.Timezone, msg); err != nil {
		l.log.Warn("save exit", "profile", profileID, "err", err)
	}
	if l.OnData != nil {
		l.OnData("profiles")
	}
}
