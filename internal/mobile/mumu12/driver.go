// SPDX-License-Identifier: AGPL-3.0-or-later

package mumu12

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Driver manages MuMuPlayer 12 instances on Windows via MuMuManager.exe.
type Driver struct {
	log *slog.Logger

	mu      sync.Mutex
	running map[string]*runState
}

type runState struct {
	idx  int
	adb  adb.Client
	done chan struct{}
}

func New(log *slog.Logger) *Driver {
	return &Driver{log: log, running: map[string]*runState{}}
}

func (d *Driver) Name() string { return "mumu12" }

func (d *Driver) Available() error {
	_, err := findInstall()
	return err
}

func (d *Driver) mgr() (manager, error) {
	dir, err := findInstall()
	if err != nil {
		return manager{}, err
	}
	return manager{dir: dir}, nil
}

func (d *Driver) Create(ctx context.Context, dev mobile.Device) (string, string, error) {
	m, err := d.mgr()
	if err != nil {
		return "", "", err
	}
	out, err := m.run(ctx, "create", "-n", "1")
	if err != nil {
		return "", "", err
	}
	infos, err := parseInfo(out)
	if err != nil || len(infos) == 0 {
		// Fall back to listing all and picking the highest index.
		allOut, lerr := m.run(ctx, "info", "-v", "all")
		if lerr != nil {
			return "", "", fmt.Errorf("mumu12 create parse: %v", err)
		}
		infos, err = parseInfo(allOut)
		if err != nil || len(infos) == 0 {
			return "", "", errors.New("mumu12 did not return a new instance index")
		}
	}
	idx := infos[len(infos)-1].Index
	_, _ = m.run(ctx, "rename", "-v", strconv.Itoa(idx), "-n", "ap-"+dev.ID)
	_, _ = m.run(ctx, settingArgs(idx, dev.CPUs, dev.RAMMB, dev.ScreenW, dev.ScreenH, dev.Density)...)
	return strconv.Itoa(idx), "mumu12-android12", nil
}

func (d *Driver) Start(ctx context.Context, dev mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	m, err := d.mgr()
	if err != nil {
		return mobile.Run{}, err
	}
	idx, err := validateIndex(dev.Ref)
	if err != nil {
		return mobile.Run{}, err
	}
	_, _ = m.run(ctx, settingArgs(idx, dev.CPUs, dev.RAMMB, dev.ScreenW, dev.ScreenH, dev.Density)...)
	if _, err := m.run(ctx, "control", "-v", strconv.Itoa(idx), "launch"); err != nil {
		return mobile.Run{}, err
	}
	// Poll info until ADB port is assigned and Android is up.
	deadline := time.Now().Add(2 * time.Minute)
	var info Info
	for time.Now().Before(deadline) {
		out, err := m.run(ctx, "info", "-v", strconv.Itoa(idx))
		if err == nil {
			if infos, perr := parseInfo(out); perr == nil && len(infos) > 0 && infos[0].ADBPort > 0 && infos[0].IsAndroidStarted {
				info = infos[0]
				break
			}
		}
		select {
		case <-ctx.Done():
			return mobile.Run{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if info.ADBPort == 0 {
		return mobile.Run{}, errors.New("MuMuPlayer 12 did not start in time")
	}
	a := adb.Client{Bin: m.adb(), Port: info.ADBPort}
	if err := a.WaitBoot(ctx, nil); err != nil {
		return mobile.Run{}, err
	}
	// shortcut: Windows MuMu12 proxy tunneling reuses AnyProfile's relay + adb reverse; full VPN app auto-config follows ldplayer/tunnel.go on Windows hardware QA.
	rs := &runState{idx: idx, adb: a, done: make(chan struct{})}
	d.mu.Lock()
	d.running[dev.ID] = rs
	d.mu.Unlock()
	go d.watch(dev.ID, m, rs)
	return mobile.Run{Launch: mobile.Launch{PID: info.PID, ADBPort: info.ADBPort}, Done: rs.done}, nil
}

func (d *Driver) watch(id string, m manager, rs *runState) {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for range tick.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := m.run(ctx, "info", "-v", strconv.Itoa(rs.idx))
		cancel()
		if err != nil {
			continue
		}
		infos, err := parseInfo(out)
		if err != nil || len(infos) == 0 || !infos[0].IsProcessStarted {
			d.mu.Lock()
			if d.running[id] == rs {
				delete(d.running, id)
			}
			d.mu.Unlock()
			close(rs.done)
			return
		}
	}
}

func (d *Driver) Stop(ctx context.Context, dev mobile.Device) error {
	m, err := d.mgr()
	if err != nil {
		return err
	}
	idx, err := validateIndex(dev.Ref)
	if err != nil {
		return err
	}
	_, err = m.run(ctx, "control", "-v", strconv.Itoa(idx), "shutdown")
	return err
}

func (d *Driver) Delete(ctx context.Context, dev mobile.Device) error {
	m, err := d.mgr()
	if err != nil {
		return err
	}
	idx, err := validateIndex(dev.Ref)
	if err != nil {
		return err
	}
	_, err = m.run(ctx, "delete", "-v", strconv.Itoa(idx))
	return err
}

func (d *Driver) Backup(ctx context.Context, dev mobile.Device, dir string) error {
	return errors.New("MuMuPlayer 12 backup is not implemented yet")
}

func (d *Driver) Restore(ctx context.Context, dev mobile.Device, dir string) error {
	return errors.New("MuMuPlayer 12 restore is not implemented yet")
}

func (d *Driver) Adopt(ctx context.Context, dev mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	return mobile.Run{}, errors.New("not running")
}

func (d *Driver) ADB(dev mobile.Device) (adb.Client, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rs := d.running[dev.ID]
	if rs == nil {
		return adb.Client{}, false
	}
	return rs.adb, true
}
