// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

// Package vm is the macOS mobile driver: each device is a LineageOS ARM64
// virtual machine on QEMU with Hypervisor.framework, its disks qcow2
// overlays of one shared image, its network a vnet that dials through the
// device's proxy, its screen a scrcpy window.
package vm

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm/vnet"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

const (
	firstBootTimeout = 6 * time.Minute
	bootTimeout      = 4 * time.Minute
	powerdownTimeout = 30 * time.Second
	guestADBPort     = 5555
	// The screen of a mid-range phone. Android draws it on the CPU (no GPU in
	// the VM) and scrcpy encodes it on the CPU too: at 1080x2400, 42% of frames
	// were janky while scrolling, at 720x1600 16%, with half the CPU. The
	// density keeps the layout of a 1080x2400 phone at 420.
	screenW, screenH, density = 720, 1600, 280
)

// screenOf returns a device's Android display override; zero fields mean the
// image's default.
func screenOf(d mobile.Device) (w, h, dens int) {
	w, h, dens = d.ScreenW, d.ScreenH, d.Density
	if w <= 0 {
		w = screenW
	}
	if h <= 0 {
		h = screenH
	}
	if dens <= 0 {
		dens = density
	}
	return w, h, dens
}

// Driver runs the VMs of one computer.
type Driver struct {
	runtime string // the mobile runtime folder (see Runtime)
	devices string // one folder per device
	log     *slog.Logger
	cat     *engine.Catalog // the image's releases
	offered *engine.Catalog // the runtime's, as Pro offers them (OfferRuntime)

	mu        sync.Mutex
	procs     map[string]*proc
	metricsMu sync.Mutex
	metrics   map[string]*mobile.Metrics
	fastStart bool // save RAM on Stop and thaw on Start
}

// proc is a running VM.
type proc struct {
	pid    int
	qmp    string // QEMU's control socket
	net    *vnet.Network
	adb    adb.Client
	name   string
	rt     Runtime
	scrcpy *exec.Cmd
	paused bool          // QMP stopped the vCPUs
	exited chan struct{} // QEMU is gone
}

func New(runtimeDir, devicesDir string, log *slog.Logger) *Driver {
	return &Driver{runtime: runtimeDir, devices: devicesDir, log: log, offered: engine.NewCatalog(""),
		procs: map[string]*proc{}, metrics: map[string]*mobile.Metrics{}, fastStart: true}
}

// SetFastStart turns the saved-RAM state on or off. Turning it off deletes
// every saved state: they are large, and the user asked for the disk back.
func (v *Driver) SetFastStart(on bool) {
	v.mu.Lock()
	v.fastStart = on
	v.mu.Unlock()
	if !on {
		v.clearRAM()
	}
}

// FastStart reports whether Stop saves the RAM state.
func (v *Driver) FastStart() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fastStart
}

func (v *Driver) clearRAM() {
	entries, err := os.ReadDir(v.devices)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(v.devices, e.Name())
		os.Remove(filepath.Join(dir, ramStateFile))
		os.Remove(filepath.Join(dir, ramMetaFile))
	}
}

func (v *Driver) Name() string { return "vm" }

func (v *Driver) record(id string, fn func(*mobile.Metrics)) {
	v.metricsMu.Lock()
	defer v.metricsMu.Unlock()
	m := v.metrics[id]
	if m == nil {
		m = &mobile.Metrics{}
		v.metrics[id] = m
	}
	fn(m)
}

// Metrics returns what this app run measured for one device.
func (v *Driver) Metrics(id string) mobile.Metrics {
	v.metricsMu.Lock()
	defer v.metricsMu.Unlock()
	if m := v.metrics[id]; m != nil {
		return *m
	}
	return mobile.Metrics{}
}

func (v *Driver) Available() error {
	_, err := Locate(v.runtime)
	return err
}

func (v *Driver) dir(d mobile.Device) string {
	ref := d.Ref
	if ref == "" {
		ref = d.ID
	}
	return filepath.Join(v.devices, filepath.Base(ref))
}

// sockets are kept apart from the device folder: a unix socket path is at
// most 104 bytes on macOS, which "~/Library/Application Support/…" can pass.
func sockets(id string) (netSock, qmpSock string, err error) {
	dir := filepath.Join(os.TempDir(), "anyprofile-mobile")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", "", err
	}
	short := id
	if len(short) > 16 {
		short = short[:16]
	}
	return filepath.Join(dir, short+".net"), filepath.Join(dir, short+".qmp"), nil
}

// macFor gives each device its own stable network card address, in the
// locally administered range QEMU uses (52:54:00).
func macFor(id string) string {
	h := sha256.Sum256([]byte("anyprofile-mobile-mac:" + id))
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", h[0], h[1], h[2])
}

// qemuScreenOf keeps QEMU at the image's known-good 720x1600. Android 16 does
// not always finish booting when the virtual GPU starts at a smaller mode;
// Start applies the preset's smaller display with wm size after boot instead.
func qemuScreenOf(d mobile.Device) (w, h int) {
	w, h = screenW, screenH
	if d.ScreenW > w {
		w = d.ScreenW
	}
	if d.ScreenH > h {
		h = d.ScreenH
	}
	return w, h
}

func qemuArgs(rt Runtime, d mobile.Device, mac, netSock, qmpSock string) []string {
	w, h := qemuScreenOf(d)
	return []string{
		"-name", "anyprofile-" + d.ID,
		"-machine", "virt", "-accel", "hvf", "-cpu", "host",
		"-smp", strconv.Itoa(d.CPUs), "-m", strconv.Itoa(d.RAMMB),
		"-nodefaults", "-display", "none",
		"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" + optEscape(rt.Firmware),
		"-drive", "if=pflash,format=raw,unit=1,file=efi_vars.fd",
		"-drive", "file=vda.qcow2,if=none,id=vd0,format=qcow2,discard=unmap,detect-zeroes=unmap",
		"-device", "virtio-blk-pci,drive=vd0,bootindex=0",
		"-drive", "file=vdb.qcow2,if=none,id=vd1,format=qcow2,discard=unmap,detect-zeroes=unmap",
		"-device", "virtio-blk-pci,drive=vd1,bootindex=1",
		"-netdev", "stream,id=net0,server=off,addr.type=unix,addr.path=" + optEscape(netSock) + ",reconnect-ms=1000",
		"-device", "virtio-net-pci,netdev=net0,mac=" + mac + ",romfile=",
		"-device", fmt.Sprintf("virtio-gpu-pci,xres=%d,yres=%d", w, h),
		"-device", "virtio-rng-pci",
		"-device", "qemu-xhci", "-device", "usb-tablet", "-device", "usb-kbd",
		"-qmp", "unix:" + optEscape(qmpSock) + ",server=on,wait=off",
		"-serial", "file:serial.log",
	}
}

// directDial is the network of a device the user gave no proxy.
func directDial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
}

// offline is the network while a device is first set up: nothing goes out.
func offline(context.Context, string) (net.Conn, error) {
	return nil, errors.New("no network while the device is set up")
}

func dialerFor(up *netproxy.Upstream) vnet.Dialer {
	if up == nil {
		return directDial
	}
	u := *up
	return func(ctx context.Context, addr string) (net.Conn, error) { return u.DialTunnel(ctx, addr) }
}

// boot starts QEMU on its network and waits for Android.
func (v *Driver) boot(ctx context.Context, rt Runtime, d mobile.Device, dial vnet.Dialer, timeout time.Duration, extra ...string) (*proc, error) {
	return v.bootWith(ctx, rt, d, dial, timeout, false, extra...)
}

// bootWith is boot with an optional thaw: a QEMU started with -incoming loads
// the migration stream paused (the state was saved from a paused guest), so it
// needs one QMP cont before adb can answer.
func (v *Driver) bootWith(ctx context.Context, rt Runtime, d mobile.Device, dial vnet.Dialer, timeout time.Duration, thaw bool, extra ...string) (*proc, error) {
	netSock, qmpSock, err := sockets(d.ID)
	if err != nil {
		return nil, err
	}
	dir := v.dir(d)
	n, err := vnet.Start(vnet.Config{Dial: dial, MAC: macFor(d.ID), Socket: netSock})
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	port, err := n.ForwardTCP(guestADBPort)
	if err != nil {
		n.Close()
		return nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dir, "qemu.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		n.Close()
		return nil, err
	}
	os.Remove(qmpSock)
	cmd := exec.Command(rt.QEMU, append(qemuArgs(rt, d, macFor(d.ID), netSock, qmpSock), extra...)...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, logf, logf
	cmd.SysProcAttr = launcher.DetachedAttr()
	if err := cmd.Start(); err != nil {
		logf.Close()
		n.Close()
		return nil, fmt.Errorf("start qemu: %w", err)
	}
	p := &proc{pid: cmd.Process.Pid, qmp: qmpSock, net: n, adb: adb.Client{Bin: rt.ADB, Port: port}, name: d.Name, rt: rt, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		logf.Close()
		close(p.exited)
	}()
	bootStarted := time.Now()
	bctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if thaw {
		if err := v.waitIncoming(bctx, p); err != nil {
			v.kill(p)
			return nil, err
		}
	}
	if err := p.adb.WaitBoot(bctx, p.exited); err != nil {
		v.kill(p)
		if tail := logTail(filepath.Join(dir, "qemu.log")); tail != "" {
			return nil, fmt.Errorf("%w: %s", err, tail)
		}
		return nil, err
	}
	v.record(d.ID, func(m *mobile.Metrics) {
		m.BootMS = time.Since(bootStarted).Milliseconds()
		m.ADBPort = port
	})
	return p, nil
}

// kill ends QEMU and its network, waiting for QEMU to go.
func (v *Driver) kill(p *proc) {
	_ = syscall.Kill(p.pid, syscall.SIGTERM)
	select {
	case <-p.exited:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(p.pid, syscall.SIGKILL)
		select {
		case <-p.exited:
		case <-time.After(5 * time.Second):
		}
	}
	p.net.Close()
	p.adb.Disconnect()
}

// powerdown asks Android to shut down and waits; QEMU is killed after the
// timeout. Android ignores QEMU's power button (system_powerdown), so the
// request goes through adb, with QMP as the fallback.
func (v *Driver) powerdown(ctx context.Context, p *proc) {
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, err := p.adb.Shell(c, "svc power shutdown")
	cancel()
	if err != nil {
		c, cancel = context.WithTimeout(ctx, 5*time.Second)
		_, err = qmpCall(c, p.qmp, "system_powerdown", nil)
		cancel()
	}
	if err == nil {
		select {
		case <-p.exited:
		case <-time.After(powerdownTimeout):
		case <-ctx.Done():
		}
	}
	v.kill(p)
}

// Create makes the device's disks and sets Android up once, offline: no
// setup wizard, adb on.
func (v *Driver) Create(ctx context.Context, d mobile.Device) (string, string, error) {
	rt, err := Locate(v.runtime)
	if err != nil {
		return "", "", err
	}
	d.Ref = d.ID
	dir := v.dir(d)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	if out, err := exec.CommandContext(ctx, rt.QEMUImg, "create", "-q", "-f", "qcow2", "-F", "qcow2",
		"-b", filepath.Join(rt.ImageDir, "vda.qcow2"), filepath.Join(dir, "vda.qcow2")).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("qemu-img create: %v: %s", err, out)
	}
	// The data disk is small when empty; a full copy keeps the user's data
	// independent of the image it started from.
	if err := copyFile(filepath.Join(rt.ImageDir, "vdb.qcow2"), filepath.Join(dir, "vdb.qcow2")); err != nil {
		return "", "", err
	}
	if err := cloneOrCopy(rt.FirmwareVars, filepath.Join(dir, "efi_vars.fd")); err != nil {
		return "", "", err
	}
	if err := setGrubenv(ctx, disk{rt.QEMUImg, filepath.Join(dir, "vda.qcow2")}, grubDefaults); err != nil {
		return "", "", fmt.Errorf("grubenv: %w", err)
	}
	p, err := v.boot(ctx, rt, d, offline, firstBootTimeout)
	if err != nil {
		return "", "", err
	}
	err = provision(ctx, p.adb)
	v.powerdown(ctx, p)
	if err != nil {
		return "", "", err
	}
	if err := v.saveClean(d); err != nil {
		return "", "", fmt.Errorf("save clean state: %w", err)
	}
	ok = true
	return d.ID, rt.ImageVersion, nil
}

// provision runs once, on the first boot.
func provision(ctx context.Context, a adb.Client) error {
	for _, line := range []string{
		"settings put global device_provisioned 1",
		"settings put secure user_setup_complete 1",
		"pm disable-user --user 0 org.lineageos.setupwizard",
		// No PIN, so no lock screen: it would cover every app after boot.
		"locksettings set-disabled true",
	} {
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := a.Shell(c, line)
		cancel()
		if err != nil {
			return fmt.Errorf("set up Android: %w", err)
		}
	}
	return nil
}

// Start boots the device on its proxy, sets its timezone and language, and
// opens its window unless the caller asked for a headless start.
func (v *Driver) Start(ctx context.Context, d mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	started := time.Now()
	rt, err := Locate(v.runtime)
	if err != nil {
		return mobile.Run{}, err
	}
	p, err := v.restore(ctx, rt, d, dialerFor(o.Upstream))
	if err != nil {
		if !errors.Is(err, errNoRAM) {
			v.log.Warn("fast start", "device", d.ID, "err", err)
		}
		p, err = v.boot(ctx, rt, d, dialerFor(o.Upstream), bootTimeout)
		if err != nil {
			return mobile.Run{}, err
		}
	}
	if err := setClock(ctx, p.adb, time.Now()); err != nil {
		v.log.Warn("mobile clock", "device", d.ID, "err", err)
	}
	if err := applyDisplay(ctx, p.adb, d); err != nil {
		v.log.Warn("mobile display", "device", d.ID, "err", err)
	}
	if err := applyAutomationSettings(ctx, p.adb, o.Headless); err != nil {
		v.log.Warn("mobile automation settings", "device", d.ID, "err", err)
	}
	// Devices made before the lock screen was turned off at setup still show it.
	if _, err := p.adb.Shell(ctx, "wm dismiss-keyguard"); err != nil {
		v.log.Warn("mobile keyguard", "device", d.ID, "err", err)
	}
	if err := applyLocale(ctx, p.adb, o.Timezone, o.Locale); err != nil {
		v.log.Warn("mobile locale", "device", d.ID, "err", err)
	}
	st, _ := launcher.ProcessStartTime(p.pid)
	if !o.Headless {
		v.showOn(ctx, d, p)
	}
	run := v.track(d.ID, p, st)
	v.record(d.ID, func(m *mobile.Metrics) {
		m.StartMS = time.Since(started).Milliseconds()
		m.Headless = o.Headless
		m.ADBPort = p.adb.Port
	})
	return run, nil
}

// applyDisplay applies the device's screen preset. The image is 720x1600;
// wm overrides let an automation device run a smaller, cheaper display.
func applyDisplay(ctx context.Context, a adb.Client, d mobile.Device) error {
	w, h, dens := screenOf(d)
	if _, err := a.Shell(ctx, fmt.Sprintf("wm size %dx%d", w, h)); err != nil {
		return err
	}
	if _, err := a.Shell(ctx, "wm density "+strconv.Itoa(dens)); err != nil {
		return err
	}
	return nil
}

// applyAutomationSettings trims animations. A headless device's screen turns
// off after 15 s, because a lit screen composites frames for nobody; a device
// with a window stays awake. Control.wake turns the screen back on before a
// screenshot or a tap.
func applyAutomationSettings(ctx context.Context, a adb.Client, headless bool) error {
	timeout, stayon := "2147483647", "true"
	if headless {
		timeout, stayon = "15000", "false"
	}
	for _, line := range []string{
		"settings put global window_animation_scale 0",
		"settings put global transition_animation_scale 0",
		"settings put global animator_duration_scale 0",
		"settings put system screen_off_timeout " + timeout,
		"svc power stayon " + stayon,
	} {
		if _, err := a.Shell(ctx, line); err != nil {
			return err
		}
	}
	return nil
}

// track registers a running VM and returns its Run; Done closes once QEMU
// has exited and everything around it is cleaned up.
func (v *Driver) track(id string, p *proc, startTime string) mobile.Run {
	v.mu.Lock()
	v.procs[id] = p
	v.mu.Unlock()
	done := make(chan struct{})
	go func() {
		<-p.exited
		v.mu.Lock()
		if v.procs[id] == p {
			delete(v.procs, id)
		}
		v.mu.Unlock()
		p.net.Close()
		v.closeWindow(p)
		p.adb.Disconnect()
		close(done)
	}()
	return mobile.Run{Launch: mobile.Launch{PID: p.pid, PIDStartTime: startTime, ADBPort: p.adb.Port}, Done: done}
}

func (v *Driver) proc(id string) *proc {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.procs[id]
}

func (v *Driver) Stop(ctx context.Context, d mobile.Device) error {
	p := v.proc(d.ID)
	if p == nil {
		return nil
	}
	if p.paused {
		_ = v.Resume(d.ID)
	}
	v.closeWindow(p)
	if v.FastStart() {
		if err := v.saveRAM(ctx, d, p); err == nil {
			return nil
		} else {
			v.log.Warn("save RAM state", "device", d.ID, "err", err)
		}
	}
	v.powerdown(ctx, p)
	return nil
}

// ADB is the adb of a running device.
func (v *Driver) ADB(d mobile.Device) (adb.Client, bool) {
	p := v.proc(d.ID)
	if p == nil {
		return adb.Client{}, false
	}
	return p.adb, true
}

// Show opens the device's window again after the user closed it.
func (v *Driver) Show(ctx context.Context, d mobile.Device) error {
	p := v.proc(d.ID)
	if p == nil {
		return errors.New("the device is not running")
	}
	if p.paused {
		if err := v.Resume(d.ID); err != nil {
			return err
		}
	}
	v.showOn(ctx, d, p)
	return nil
}

// Pause freezes the device's vCPUs with QMP stop. The guest keeps its RAM and
// its apps; the host stops spending CPU on an idle Android. Resume thaws it.
func (v *Driver) Pause(id string) error {
	v.mu.Lock()
	p := v.procs[id]
	v.mu.Unlock()
	if p == nil {
		return errors.New("the device is not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := qmpCall(ctx, p.qmp, "stop", nil); err != nil {
		return err
	}
	v.mu.Lock()
	p.paused = true
	v.mu.Unlock()
	return nil
}

// Resume thaws a paused device. adb may have dropped the connection while the
// guest was frozen, so it reconnects best-effort.
func (v *Driver) Resume(id string) error {
	v.mu.Lock()
	p := v.procs[id]
	v.mu.Unlock()
	if p == nil {
		return errors.New("the device is not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := qmpCall(ctx, p.qmp, "cont", nil); err != nil {
		return err
	}
	v.mu.Lock()
	p.paused = false
	v.mu.Unlock()
	p.adb.Connect(ctx)
	return nil
}

// Paused reports whether the device's vCPUs are frozen.
func (v *Driver) Paused(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	p := v.procs[id]
	return p != nil && p.paused
}

// Windowed reports whether the device has a scrcpy window open.
func (v *Driver) Windowed(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	p := v.procs[id]
	return p != nil && p.scrcpy != nil
}

// openWindows counts the device windows on screen: what a new one steps away
// from. openWindowsLocked is the same for a caller that already holds the lock.
func (v *Driver) openWindows() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.openWindowsLocked()
}

func (v *Driver) openWindowsLocked() int {
	n := 0
	for _, p := range v.procs {
		if p.scrcpy != nil {
			n++
		}
	}
	return n
}

func (v *Driver) showOn(ctx context.Context, d mobile.Device, p *proc) {
	setAwake(ctx, p.adb, true)
	v.mu.Lock()
	defer v.mu.Unlock()
	if p.scrcpy != nil && p.scrcpy.ProcessState == nil {
		if p.scrcpy.Process != nil && p.scrcpy.Process.Signal(syscall.Signal(0)) == nil {
			return // already open
		}
	}
	// 30 fps and 4 Mbps keep the host cool: the guest has no GPU, so every
	// frame is encoded and decoded on the CPU. A phone at 720x1600 needs no
	// more to look right. The geometry is the last one seen for this device;
	// a device that never showed a window steps away from the ones already
	// open, so two of them do not sit on top of each other.
	ws, ok := v.loadWindow(d)
	if !ok {
		ws = defaultWindow(v.openWindowsLocked())
	}
	// Only the position is passed back: CGWindow bounds include the title bar,
	// so feeding the saved height as scrcpy's content height would grow the
	// window on every open. scrcpy sizes it from the 720x1600 video.
	args := []string{"-s", p.adb.Serial(), "--window-title=" + p.name,
		"--no-audio", "--max-fps=30", "--video-bit-rate=4M",
		fmt.Sprintf("--window-x=%d", ws.X), fmt.Sprintf("--window-y=%d", ws.Y)}
	if ws.OnTop {
		args = append(args, "--always-on-top")
	}
	cmd := exec.Command(p.rt.Scrcpy, args...)
	cmd.Env = append(p.adb.Env(), "ADB="+p.rt.ADB, "SCRCPY_SERVER_PATH="+p.rt.ScrcpyServer)
	if err := cmd.Start(); err != nil {
		setAwake(ctx, p.adb, false) // no window: do not leave the screen on
		v.log.Warn("scrcpy", "err", err)
		return
	}
	p.scrcpy = cmd
	go func() {
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				v.mu.Lock()
				closed := p.scrcpy == cmd
				if closed {
					p.scrcpy = nil
				}
				v.mu.Unlock()
				if !closed {
					return // Stop closed it on purpose; the device is going away.
				}
				// The user closed the window: the device keeps running for
				// agents, so put it back into the cheap headless power state.
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				setAwake(c, p.adb, false)
				cancel()
				return
			case <-tick.C:
				if r, ok := scrcpyWindow(cmd.Process.Pid); ok {
					ws.X, ws.Y, ws.W, ws.H = r.X, r.Y, r.W, r.H
					_ = v.saveWindow(d, ws)
				}
			}
		}
	}()
}

// setAwake switches between the windowed power state (screen on forever) and
// the headless one (screen off after 15 s). It is best-effort: a device that
// is stopping may not answer.
func setAwake(ctx context.Context, a adb.Client, on bool) {
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if on {
		_, _ = a.Shell(c, "svc power stayon true")
		_, _ = a.Shell(c, "settings put system screen_off_timeout 2147483647")
		_, _ = a.Shell(c, "input keyevent KEYCODE_WAKEUP")
		return
	}
	_, _ = a.Shell(c, "svc power stayon false")
	_, _ = a.Shell(c, "settings put system screen_off_timeout 15000")
}

func (v *Driver) closeWindow(p *proc) {
	v.mu.Lock()
	c := p.scrcpy
	p.scrcpy = nil
	v.mu.Unlock()
	if c != nil && c.Process != nil {
		_ = c.Process.Signal(syscall.SIGTERM)
	}
}

// cleanDir holds the disks a hard reset restores. They are APFS clones, so
// keeping them costs almost no space until the device writes.
func (v *Driver) cleanDir(d mobile.Device) string { return filepath.Join(v.dir(d), "clean") }

func (v *Driver) saveClean(d mobile.Device) error {
	clean := v.cleanDir(d)
	if err := os.RemoveAll(clean); err != nil {
		return err
	}
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return err
	}
	for _, f := range imageFiles {
		if err := cloneOrCopy(filepath.Join(v.dir(d), f), filepath.Join(clean, f)); err != nil {
			os.RemoveAll(clean)
			return err
		}
	}
	return nil
}

// Reset returns a device to a clean state. soft clears third-party app data
// on a running device; hard stops it and restores the disks saved after
// provisioning, leaving it stopped for the caller to open.
func (v *Driver) Reset(ctx context.Context, d mobile.Device, mode string) error {
	started := time.Now()
	switch mode {
	case "soft":
		p := v.proc(d.ID)
		if p == nil {
			return mobile.ErrNotRunning
		}
		out, err := p.adb.Shell(ctx, "pm list packages -3")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(out, "\n") {
			pkg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "package:"))
			if !validPackage(pkg) {
				continue
			}
			if _, err := p.adb.Shell(ctx, "pm clear "+pkg); err != nil {
				v.log.Warn("mobile soft reset", "device", d.ID, "package", pkg, "err", err)
			}
		}
		if _, err := p.adb.Shell(ctx, "input keyevent HOME"); err != nil {
			return err
		}
	case "hard":
		if v.proc(d.ID) != nil {
			if err := v.Stop(ctx, d); err != nil {
				return err
			}
		}
		clean := v.cleanDir(d)
		for _, f := range imageFiles {
			if !exists(filepath.Join(clean, f)) {
				return mobile.ErrNoClean
			}
		}
		for _, f := range imageFiles {
			dst := filepath.Join(v.dir(d), f)
			tmp := dst + ".reset"
			os.Remove(tmp)
			if err := cloneOrCopy(filepath.Join(clean, f), tmp); err != nil {
				os.Remove(tmp)
				return err
			}
			if err := os.Rename(tmp, dst); err != nil {
				return err
			}
		}
	default:
		return mobile.ErrBadReset
	}
	v.dropRAM(d) // the disks changed; a saved RAM state no longer matches them
	v.record(d.ID, func(m *mobile.Metrics) { m.ResetMS = time.Since(started).Milliseconds() })
	return nil
}

func validPackage(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func (v *Driver) Delete(ctx context.Context, d mobile.Device) error {
	if p := v.proc(d.ID); p != nil {
		v.kill(p)
	}
	netSock, qmpSock, _ := sockets(d.ID)
	os.Remove(netSock)
	os.Remove(qmpSock)
	return os.RemoveAll(v.dir(d))
}

// Backup copies the stopped device's disks into dir; on APFS a clone takes no
// time and no space until the disks change.
func (v *Driver) Backup(ctx context.Context, d mobile.Device, dir string) error {
	if v.proc(d.ID) != nil {
		return errors.New("the device is running")
	}
	for _, f := range imageFiles {
		if err := cloneOrCopy(filepath.Join(v.dir(d), f), filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	return nil
}

func (v *Driver) Restore(ctx context.Context, d mobile.Device, dir string) error {
	if v.proc(d.ID) != nil {
		return errors.New("the device is running")
	}
	for _, f := range imageFiles {
		src := filepath.Join(dir, f)
		if !exists(src) {
			return fmt.Errorf("the backup is missing %s", f)
		}
	}
	for _, f := range imageFiles {
		dst := filepath.Join(v.dir(d), f)
		tmp := dst + ".restore"
		os.Remove(tmp)
		if err := cloneOrCopy(filepath.Join(dir, f), tmp); err != nil {
			os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
	}
	v.dropRAM(d) // restored disks do not match a saved RAM state
	return nil
}

// Adopt takes back a VM that outlived the app: its network is rebuilt on the
// same socket, which QEMU reconnects to by itself.
func (v *Driver) Adopt(ctx context.Context, d mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	if !launcher.SameProcess(l.PID, l.PIDStartTime) {
		return mobile.Run{}, errors.New("the virtual machine is gone")
	}
	rt, err := Locate(v.runtime)
	if err != nil {
		return mobile.Run{}, err
	}
	netSock, qmpSock, err := sockets(d.ID)
	if err != nil {
		return mobile.Run{}, err
	}
	n, err := vnet.Start(vnet.Config{Dial: dialerFor(up), MAC: macFor(d.ID), Socket: netSock})
	if err != nil {
		return mobile.Run{}, err
	}
	port, err := n.ForwardTCP(guestADBPort)
	if err != nil {
		n.Close()
		return mobile.Run{}, err
	}
	p := &proc{pid: l.PID, qmp: qmpSock, net: n, adb: adb.Client{Bin: rt.ADB, Port: port}, name: d.Name, rt: rt, exited: make(chan struct{})}
	// The app may have frozen the device just before it crashed; remember
	// that, or the first adb call would hang until a timeout.
	if st, err := qmpStatus(ctx, qmpSock); err == nil && st == "paused" {
		p.paused = true
	}
	go func() { // not our child: watch the pid
		for launcher.SameProcess(l.PID, l.PIDStartTime) {
			time.Sleep(2 * time.Second)
		}
		close(p.exited)
	}()
	return v.track(d.ID, p, l.PIDStartTime), nil
}

// logTail is the last line QEMU wrote, which says why it stopped.
func logTail(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// cloneOrCopy clones src to dst (APFS), or copies it on other volumes.
func cloneOrCopy(src, dst string) error {
	if err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW); err == nil {
		return nil
	}
	return copyFile(src, dst)
}
