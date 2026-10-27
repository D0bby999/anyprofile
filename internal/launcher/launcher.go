// SPDX-License-Identifier: AGPL-3.0-or-later

// Package launcher starts, stops and adopts browser processes. No DevTools
// port is opened unless the profile asked for diagnostics; processes are
// managed through their pid, Chromium's SingletonLock and signals.
package launcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

type State string

const (
	Starting State = "starting"
	Running  State = "running"
	Stopping State = "stopping"
	Stopped  State = "stopped"
)

// Info is a profile's process state, as sent to the UI.
type Info struct {
	ProfileID string `json:"profileId"`
	State     State  `json:"state"`
	PID       int    `json:"pid"`
	CDPPort   int    `json:"cdpPort"` // 0 unless diagnostics were enabled
	Error     string `json:"error"`
}

type proc struct {
	info  Info
	relay *netproxy.Relay
	cdp   *cdppipe.Conn
	done  chan struct{} // closed when the browser has exited and been cleaned up
	// sessionID is the sessions row of this launch; 0 when it could not be recorded.
	sessionID int64
	// started is when the browser came up (for an adopted one: its session start).
	started time.Time
	// behind: started for a script or an agent, so it must not take the focus.
	behind bool
	// gated: this launch took a launchGate slot, released exactly once
	// when the proc leaves the launcher's map. Adopted browsers never
	// took a slot, so they never release one.
	gated bool
}

type Launcher struct {
	st       *store.Store
	sec      secrets.Store
	dirs     paths.Dirs
	engines  map[string]engine.Engine
	onChange func(Info)
	log      *slog.Logger

	mu    sync.Mutex
	procs map[string]*proc
	gate  *launchGate

	readyTimeout, termGrace time.Duration
	// TestArgs are added to every launch. Only integration tests set them,
	// to run the browsers headless: no window shows on the screen of
	// someone at work while they run.
	TestArgs []string

	// Capture measures the fingerprint at each launch; the app turns it on.
	Capture bool
	// OnData is told "profiles" after a measurement is saved; may be nil.
	OnData func(kind string)
	// CheckExit measures a proxy's exit right after a launch (netproxy.Check
	// in the app). Nil: sessions are recorded without an exit.
	CheckExit func(ctx context.Context, up netproxy.Upstream) (netproxy.Exit, error)
	// BeforeStart runs first in every launch and may refuse it (a snapshot
	// of the profile is being written or restored). Nil: nothing to wait for.
	BeforeStart func(profileID string) error
	// AfterExit runs once the browser has exited and the profile is Stopped.
	// ranFor is zero when the start time is unknown.
	AfterExit func(profileID string, ranFor time.Duration)
}

func New(st *store.Store, sec secrets.Store, dirs paths.Dirs, engines []engine.Engine, onChange func(Info), log *slog.Logger) *Launcher {
	m := map[string]engine.Engine{}
	for _, e := range engines {
		m[e.ID()] = e
	}
	if onChange == nil {
		onChange = func(Info) {}
	}
	return &Launcher{st: st, sec: sec, dirs: dirs, engines: m, onChange: onChange, log: log,
		procs: map[string]*proc{}, gate: newLaunchGate(), readyTimeout: 15 * time.Second, termGrace: 8 * time.Second}
}

// SetMaxRunning sets how many browsers may be starting or running at
// once; 0 means unlimited (the default). Starts beyond the ceiling wait
// inside open() until a browser exits. The app sets this from its
// settings; nothing changes until it does.
func (l *Launcher) SetMaxRunning(n int) { l.gate.SetMax(n) }

// MaxRunning reports the current launch ceiling, 0 for unlimited.
func (l *Launcher) MaxRunning() int { return l.gate.Max() }

// WaitingLaunches reports how many starts are queued at the ceiling.
func (l *Launcher) WaitingLaunches() int { return l.gate.Waiting() }

// List returns every profile that is not stopped.
func (l *Launcher) List() []Info {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Info, 0, len(l.procs))
	for _, p := range l.procs {
		out = append(out, p.info)
	}
	return out
}

// Info reports one profile's launch state; ok is false when it is not running.
func (l *Launcher) Info(profileID string) (Info, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.procs[profileID]
	if !ok {
		return Info{}, false
	}
	return p.info, true
}

func (l *Launcher) set(p *proc, s State, errMsg string) {
	l.mu.Lock()
	p.info.State, p.info.Error = s, errMsg
	info := p.info
	l.mu.Unlock()
	l.onChange(info)
}

// Start opens the profile's browser. A profile that is already open is only
// brought to the front: there is never a second browser on one profile.
func (l *Launcher) Start(ctx context.Context, profileID string) error {
	return l.open(ctx, profileID, false)
}

// StartBehind opens the profile's browser for a script or an agent: the app
// the user is in stays in front. An open profile is left where it is.
func (l *Launcher) StartBehind(ctx context.Context, profileID string) error {
	return l.open(ctx, profileID, true)
}

// existing reports whether the profile is already in the map and, if so,
// what a new open should do about it: focus a running browser, or refuse
// because it is between states.
func (l *Launcher) existing(profileID string, behind bool) (bool, error) {
	l.mu.Lock()
	p, ok := l.procs[profileID]
	if !ok {
		l.mu.Unlock()
		return false, nil
	}
	pid, state := p.info.PID, p.info.State
	l.mu.Unlock()
	if state == Running {
		if !behind {
			activate(pid)
		}
		return true, nil
	}
	return true, fmt.Errorf("profile is %s", state)
}

func (l *Launcher) open(ctx context.Context, profileID string, behind bool) error {
	if handled, err := l.existing(profileID, behind); handled {
		return err
	}
	// The launch ceiling queues here, before the profile shows as
	// starting: a queued start has no proc yet, and a cancelled context
	// leaves nothing behind. With no ceiling set this never blocks.
	if err := l.gate.Acquire(ctx); err != nil {
		return err
	}
	l.mu.Lock()
	if _, ok := l.procs[profileID]; ok {
		// Another open admitted the same profile while this one queued.
		l.mu.Unlock()
		l.gate.Release()
		_, err := l.existing(profileID, behind)
		return err
	}
	p := &proc{info: Info{ProfileID: profileID, State: Starting}, done: make(chan struct{}), behind: behind, gated: true}
	l.procs[profileID] = p
	l.mu.Unlock()
	l.onChange(p.info)

	if err := l.start(ctx, p); err != nil {
		if p.relay != nil {
			p.relay.Close()
		}
		l.mu.Lock()
		delete(l.procs, profileID)
		l.mu.Unlock()
		l.gate.Release()
		l.onChange(Info{ProfileID: profileID, State: Stopped, Error: err.Error()})
		return err
	}
	return nil
}

// Conn is the DevTools pipe of a running profile. An adopted browser (one that
// outlived the app) has none.
func (l *Launcher) Conn(profileID string) (*cdppipe.Conn, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.procs[profileID]
	if !ok || p.info.State != Running || p.cdp == nil {
		return nil, false
	}
	return p.cdp, true
}

// WaitConn waits until the profile runs with a pipe, or ctx ends.
func (l *Launcher) WaitConn(ctx context.Context, profileID string) (*cdppipe.Conn, error) {
	for {
		if c, ok := l.Conn(profileID); ok {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
