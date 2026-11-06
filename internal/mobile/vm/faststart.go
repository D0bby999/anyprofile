// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/vm/vnet"
)

// Fast start: freeze a running device to disk on Stop and thaw it on Start,
// instead of booting Android again. The stream is QEMU migration state (RAM
// and device state) only — never the disks. An internal qcow2 snapshot would
// revert the disks and lose apps installed after it.
//
// A snapshot is valid only for the exact runtime, image, device shape and disk
// files it was taken from. Anything that touches the disks (restore, reset, a
// crash while running) changes their stamps and the next Start cold-boots.
const (
	ramStateFile = "ram.state"
	ramMetaFile  = "ram.json"
	ramVersion   = 1
)

// errNoRAM means "there is no valid saved state"; Start should just boot.
var errNoRAM = errors.New("no saved RAM state")

type ramMeta struct {
	Version int               `json:"version"`
	QEMU    string            `json:"qemu"`
	Image   string            `json:"image"`
	RAMMB   int               `json:"ramMb"`
	CPUs    int               `json:"cpus"`
	ScreenW int               `json:"screenW"`
	ScreenH int               `json:"screenH"`
	Density int               `json:"density"`
	Disks   map[string]string `json:"disks"`
	SavedAt string            `json:"savedAt"`
}

func fileStamp(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano()), nil
}

// ramMetaFor is the fingerprint a saved state must match to be usable.
func (v *Driver) ramMetaFor(rt Runtime, d mobile.Device) (*ramMeta, error) {
	q, err := fileStamp(rt.QEMU)
	if err != nil {
		return nil, err
	}
	w, h, dens := screenOf(d)
	m := &ramMeta{
		Version: ramVersion,
		QEMU:    q,
		Image:   rt.ImageVersion,
		RAMMB:   d.RAMMB,
		CPUs:    d.CPUs,
		ScreenW: w,
		ScreenH: h,
		Density: dens,
		Disks:   map[string]string{},
		SavedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, f := range []string{"vda.qcow2", "vdb.qcow2", "efi_vars.fd"} {
		s, err := fileStamp(filepath.Join(v.dir(d), f))
		if err != nil {
			return nil, err
		}
		m.Disks[f] = s
	}
	return m, nil
}

// ramValid reports whether a saved state exists and matches this device now.
func (v *Driver) ramValid(rt Runtime, d mobile.Device) bool {
	b, err := os.ReadFile(filepath.Join(v.dir(d), ramMetaFile))
	if err != nil {
		return false
	}
	var saved ramMeta
	if json.Unmarshal(b, &saved) != nil || saved.Version != ramVersion {
		return false
	}
	cur, err := v.ramMetaFor(rt, d)
	if err != nil {
		return false
	}
	if saved.QEMU != cur.QEMU || saved.Image != cur.Image ||
		saved.RAMMB != cur.RAMMB || saved.CPUs != cur.CPUs ||
		saved.ScreenW != cur.ScreenW || saved.ScreenH != cur.ScreenH ||
		saved.Density != cur.Density || len(saved.Disks) != len(cur.Disks) {
		return false
	}
	for name, stamp := range cur.Disks {
		if saved.Disks[name] != stamp {
			return false
		}
	}
	st, err := os.Stat(filepath.Join(v.dir(d), ramStateFile))
	return err == nil && st.Size() > 0
}

// DropRAM removes a device's saved RAM state (~2.5 GB); the next Start boots.
func (v *Driver) DropRAM(d mobile.Device) { v.dropRAM(d) }

// dropRAM removes a saved state; the next Start boots.
func (v *Driver) dropRAM(d mobile.Device) {
	dir := v.dir(d)
	os.Remove(filepath.Join(dir, ramStateFile))
	os.Remove(filepath.Join(dir, ramMetaFile))
}

// restore starts QEMU with -incoming, so it loads the saved state and runs
// instead of booting. A failure deletes the state and returns an error; Start
// then boots normally.
func (v *Driver) restore(ctx context.Context, rt Runtime, d mobile.Device, dial vnet.Dialer) (*proc, error) {
	if !v.ramValid(rt, d) {
		v.dropRAM(d) // stale or half-written: reclaim the space
		return nil, errNoRAM
	}
	state := filepath.Join(v.dir(d), ramStateFile)
	p, err := v.bootWith(ctx, rt, d, dial, 2*time.Minute, true, "-incoming", "file:"+state)
	if err != nil {
		v.dropRAM(d)
		return nil, fmt.Errorf("restore: %w", err)
	}
	return p, nil
}

// saveRAM freezes the guest, writes its RAM state next to the disks, and quits
// QEMU. The disks stay as they are; the next Start resumes this exact state.
func (v *Driver) saveRAM(ctx context.Context, d mobile.Device, p *proc) error {
	rt, err := Locate(v.runtime)
	if err != nil {
		return err
	}
	dir := v.dir(d)
	state := filepath.Join(dir, ramStateFile)
	tmp := state + ".part"
	_ = os.Remove(tmp)

	// mapped-ram writes RAM straight into the file at page offsets, and
	// -incoming file: maps it back. A unix socket or exec:cat costs ~20 s for
	// 2.4 GB because QEMU writes thousands of small chunks.
	c, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := qmpCall(c, p.qmp, "stop", nil); err != nil {
		return err
	}
	resume := func() {
		rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer rcancel()
		_, _ = qmpCall(rctx, p.qmp, "cont", nil)
	}
	// QEMU throttles migration to 128 MB/s by default, which is ~20 s for a
	// 2.4 GB state. A local file can take it much faster.
	if _, err := qmpCall(c, p.qmp, "migrate-set-parameters", map[string]any{
		"max-bandwidth": int64(8) << 30,
	}); err != nil {
		resume()
		return err
	}
	if _, err := qmpCall(c, p.qmp, "migrate", map[string]string{"uri": "file:" + tmp}); err != nil {
		resume()
		os.Remove(tmp)
		return err
	}
	if err := waitMigration(c, p.qmp); err != nil {
		resume()
		os.Remove(tmp)
		return err
	}
	_, _ = qmpCall(c, p.qmp, "quit", nil) // QEMU exits; migrate already finished
	select {
	case <-p.exited:
	case <-time.After(10 * time.Second):
		v.kill(p)
	}
	if err := os.Rename(tmp, state); err != nil {
		return err
	}
	meta, err := v.ramMetaFor(rt, d)
	if err != nil {
		os.Remove(state)
		return err
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	mt := filepath.Join(dir, ramMetaFile)
	if err := os.WriteFile(mt+".part", b, 0o600); err != nil {
		os.Remove(state)
		return err
	}
	if err := os.Rename(mt+".part", mt); err != nil {
		os.Remove(state)
		return err
	}
	return nil
}

// waitIncoming waits until QEMU has consumed the stream, then continues. The
// stream records the guest as paused (it was saved with QMP stop), so without
// the cont the VM would sit paused forever and adb would never answer.
func (v *Driver) waitIncoming(ctx context.Context, p *proc) error {
	for {
		raw, err := qmpCall(ctx, p.qmp, "query-migrate", nil)
		if err == nil {
			var st struct {
				Status    string `json:"status"`
				ErrorDesc string `json:"error-desc"`
			}
			if json.Unmarshal(raw, &st) == nil {
				switch st.Status {
				case "completed":
					_, err := qmpCall(ctx, p.qmp, "cont", nil)
					return err
				case "failed", "cancelled":
					if st.ErrorDesc != "" {
						return errors.New(st.ErrorDesc)
					}
					return fmt.Errorf("incoming %s", st.Status)
				}
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("the saved state did not load in time")
		case <-p.exited:
			return errors.New("qemu exited while loading the saved state")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitMigration(ctx context.Context, qmp string) error {
	for {
		raw, err := qmpCall(ctx, qmp, "query-migrate", nil)
		if err != nil {
			return err
		}
		var st struct {
			Status    string `json:"status"`
			ErrorDesc string `json:"error-desc"`
		}
		if err := json.Unmarshal(raw, &st); err != nil {
			return err
		}
		switch st.Status {
		case "completed":
			return nil
		case "failed", "cancelled":
			if st.ErrorDesc != "" {
				return errors.New(st.ErrorDesc)
			}
			return fmt.Errorf("migration %s", st.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
