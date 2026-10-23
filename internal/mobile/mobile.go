// SPDX-License-Identifier: AGPL-3.0-or-later

// Package mobile runs Android devices: a QEMU VM on macOS, an LDPlayer
// instance on Windows. A Driver does the work for one kind; the Manager keeps
// each device's state, records what runs so it can be adopted after a crash,
// and notices a device that stops by itself.
package mobile

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

type State string

const (
	Stopped      State = "stopped"
	Provisioning State = "provisioning" // being created; the first boot can take minutes
	Starting     State = "starting"
	Running      State = "running"
	Stopping     State = "stopping"
	Working      State = "working" // a backup, a restore or a delete on a stopped device
)

// Info is a device's live state, sent to the UI on every change.
type Info struct {
	DeviceID string `json:"deviceId"`
	State    State  `json:"state"`
	Error    string `json:"error,omitempty"` // why the last create, start or stop failed
}

// Device is what a driver needs to know about a device: no secrets, no database.
type Device struct {
	ID    string
	Name  string
	Ref   string // the driver's own handle, from Create
	RAMMB int
	CPUs  int
}

// StartOptions is the network a device gets for one run.
type StartOptions struct {
	// Upstream is the device's proxy, password included; nil means no proxy.
	Upstream *netproxy.Upstream
	Timezone string // IANA, like "Asia/Tokyo"
	Locale   string // BCP 47, like "ja-JP"
}

// Launch is what survives the app: enough to find and adopt the device again.
type Launch struct {
	PID          int
	PIDStartTime string
	ADBPort      int
	RelayPort    int
}

// Run is a started or adopted device. Done closes once it has stopped, on
// its own or through Stop.
type Run struct {
	Launch Launch
	Done   <-chan struct{}
}

// Driver runs one kind of device. Every call may block for as long as the
// device takes; the Manager calls at most one at a time per device.
type Driver interface {
	// Name is the driver column of mobile_devices: "vm" or "ldplayer".
	Name() string
	// Available is nil when devices can be created and opened, or an
	// *apperr.Error saying what is missing (a download, LDPlayer itself).
	Available() error
	// Create makes the device and returns its handle and the Android image it runs.
	Create(ctx context.Context, d Device) (ref, imageVersion string, err error)
	Start(ctx context.Context, d Device, o StartOptions) (Run, error)
	Stop(ctx context.Context, d Device) error
	// Delete removes the device and its data; it is stopped.
	Delete(ctx context.Context, d Device) error
	// Backup writes the stopped device into the empty folder dir; Restore
	// puts it back from there.
	Backup(ctx context.Context, d Device, dir string) error
	Restore(ctx context.Context, d Device, dir string) error
	// Adopt takes back a device that outlived the app. An error means it is gone.
	Adopt(ctx context.Context, d Device, l Launch, up *netproxy.Upstream) (Run, error)
}

// Shower is a Driver whose device shows in a window the user can close
// while the device keeps running; Show opens it again.
type Shower interface {
	Show(ctx context.Context, d Device) error
}

// Launches stores what runs; *store.Store outside tests.
type Launches interface {
	PutMobileLaunch(store.MobileLaunch) error
	DeleteMobileLaunch(deviceID string) error
	ListMobileLaunches() ([]store.MobileLaunch, error)
}

var ErrBusy = apperr.New("mobile_busy", "the device is starting, stopping or being created; wait for it")

type Manager struct {
	drv      Driver
	st       Launches
	log      *slog.Logger
	onChange func(Info)

	mu   sync.Mutex
	devs map[string]*entry
}

type entry struct {
	info Info
	done <-chan struct{}
}

// NewManager makes a Manager for drv. onChange may be nil.
func NewManager(drv Driver, st Launches, log *slog.Logger, onChange func(Info)) *Manager {
	return &Manager{drv: drv, st: st, log: log, onChange: onChange, devs: map[string]*entry{}}
}

func (m *Manager) Driver() Driver { return m.drv }

// Info is the device's state; Stopped when the Manager never saw it run.
func (m *Manager) Info(id string) Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.devs[id]; ok {
		return e.info
	}
	return Info{DeviceID: id, State: Stopped}
}

// List is every device the Manager knows, by id.
func (m *Manager) List() []Info {
	m.mu.Lock()
	out := make([]Info, 0, len(m.devs))
	for _, e := range m.devs {
		out = append(out, e.info)
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

// Running counts the devices that are not stopped.
func (m *Manager) Running() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.devs {
		if e.info.State != Stopped {
			n++
		}
	}
	return n
}

// set changes a device's state and tells onChange.
func (m *Manager) set(id string, st State, errText string, done <-chan struct{}) {
	m.mu.Lock()
	e, ok := m.devs[id]
	if !ok {
		e = &entry{}
		m.devs[id] = e
	}
	e.info = Info{DeviceID: id, State: st, Error: errText}
	e.done = done
	info := e.info
	m.mu.Unlock()
	if m.onChange != nil {
		m.onChange(info)
	}
}

// claim moves a stopped device to st, or reports it busy.
func (m *Manager) claim(id string, st State) error {
	m.mu.Lock()
	if e, ok := m.devs[id]; ok && e.info.State != Stopped {
		m.mu.Unlock()
		return ErrBusy
	}
	m.mu.Unlock()
	m.set(id, st, "", nil)
	return nil
}

// Create makes the device through the driver.
func (m *Manager) Create(ctx context.Context, d Device) (ref, imageVersion string, err error) {
	if err := m.claim(d.ID, Provisioning); err != nil {
		return "", "", err
	}
	ref, imageVersion, err = m.drv.Create(ctx, d)
	m.set(d.ID, Stopped, errText(err), nil)
	return ref, imageVersion, err
}

// Start opens a stopped device. A running one is left as it is.
func (m *Manager) Start(ctx context.Context, d Device, o StartOptions) error {
	if m.Info(d.ID).State == Running {
		return nil
	}
	if err := m.claim(d.ID, Starting); err != nil {
		return err
	}
	run, err := m.drv.Start(ctx, d, o)
	if err != nil {
		m.set(d.ID, Stopped, errText(err), nil)
		return err
	}
	m.running(d.ID, run)
	return nil
}

// running records a started or adopted device and watches for its end.
func (m *Manager) running(id string, run Run) {
	l := run.Launch
	if err := m.st.PutMobileLaunch(store.MobileLaunch{DeviceID: id, PID: l.PID, PIDStartTime: l.PIDStartTime,
		ADBPort: l.ADBPort, RelayPort: l.RelayPort}); err != nil {
		m.log.Warn("record mobile launch", "device", id, "err", err)
	}
	m.set(id, Running, "", run.Done)
	go func() {
		<-run.Done
		m.finish(id, run.Done)
	}()
}

// finish marks the run that done belongs to as stopped, once. A later run
// of the same device is left alone.
// The launch row goes before the lock is released, so it cannot remove the
// row of a run that starts right after.
func (m *Manager) finish(id string, done <-chan struct{}) {
	m.mu.Lock()
	e, ok := m.devs[id]
	if !ok || e.done != done {
		m.mu.Unlock()
		return
	}
	if err := m.st.DeleteMobileLaunch(id); err != nil {
		m.log.Warn("forget mobile launch", "device", id, "err", err)
	}
	e.done = nil
	e.info = Info{DeviceID: id, State: Stopped}
	info := e.info
	m.mu.Unlock()
	if m.onChange != nil {
		m.onChange(info)
	}
}

// Stop closes a running device and waits until it has stopped.
func (m *Manager) Stop(ctx context.Context, d Device) error {
	m.mu.Lock()
	e, ok := m.devs[d.ID]
	if !ok || e.info.State == Stopped {
		m.mu.Unlock()
		return nil
	}
	if e.info.State != Running {
		m.mu.Unlock()
		return ErrBusy
	}
	done := e.done
	m.mu.Unlock()
	m.set(d.ID, Stopping, "", done)
	if err := m.drv.Stop(ctx, d); err != nil {
		m.set(d.ID, Running, errText(err), done)
		return err
	}
	select {
	case <-done:
		// The watcher may still be on its way; Stopped is what the caller sees.
		m.finish(d.ID, done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Busy runs fn on a stopped device while no start or create can begin.
func (m *Manager) Busy(id string, st State, fn func() error) error {
	if err := m.claim(id, st); err != nil {
		return err
	}
	err := fn()
	m.set(id, Stopped, "", nil)
	return err
}

// AdoptDevice is a device that may have outlived the app, with the proxy it needs.
type AdoptDevice struct {
	Device   Device
	Upstream *netproxy.Upstream
}

// Adopt runs at app startup: each recorded launch is taken back when its
// device is known and the driver finds it still running; the rest are forgotten.
func (m *Manager) Adopt(ctx context.Context, devs map[string]AdoptDevice) {
	launches, err := m.st.ListMobileLaunches()
	if err != nil {
		m.log.Error("list mobile launches", "err", err)
		return
	}
	for _, l := range launches {
		d, ok := devs[l.DeviceID]
		if ok {
			run, err := m.drv.Adopt(ctx, d.Device, Launch{PID: l.PID, PIDStartTime: l.PIDStartTime,
				ADBPort: l.ADBPort, RelayPort: l.RelayPort}, d.Upstream)
			if err == nil {
				m.running(l.DeviceID, run)
				continue
			}
			m.log.Info("mobile device not adopted", "device", l.DeviceID, "err", err)
		}
		if err := m.st.DeleteMobileLaunch(l.DeviceID); err != nil {
			m.log.Warn("forget mobile launch", "device", l.DeviceID, "err", err)
		}
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Msg
	}
	return err.Error()
}
