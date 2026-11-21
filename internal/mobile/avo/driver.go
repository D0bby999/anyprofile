// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/mobile/vm/vnet"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

const bootTimeout = 3 * time.Minute

type proc struct {
	pid      int
	started  string
	adb      adb.Client
	bridge   *adbBridge
	net      *vnet.Network
	rt       Runtime
	rtDir    string
	ctrlSock string
	view     *exec.Cmd
	viewDone chan struct{}
	paused   bool
	exited   chan struct{}
}

// Driver implements mobile.Driver on macOS using avo-vmm + gfxstream (Metal)
// and AOSP Cuttlefish arm64 images.
type Driver struct {
	avoRoot     string
	qemuRuntime string
	devices     string
	log         *slog.Logger

	gmsMu   sync.Mutex
	offered engine.Catalog

	mu      sync.Mutex
	procs   map[string]*proc
	metrics map[string]mobile.Metrics
}

// New creates an Avo Metal mobile driver rooted at avoRoot (or RootFor(qemuRuntime)).
func New(qemuRuntime, devicesDir string, log *slog.Logger) *Driver {
	if log == nil {
		log = slog.Default()
	}
	return &Driver{
		avoRoot:     RootFor(qemuRuntime),
		qemuRuntime: qemuRuntime,
		devices:     devicesDir,
		log:         log,
		procs:       make(map[string]*proc),
		metrics:     make(map[string]mobile.Metrics),
	}
}

func (d *Driver) Name() string { return "avo" }

// SupportsDriver accepts legacy macOS "vm" (QEMU) device rows and upgrades
// their instance disks to Cuttlefish on first open.
func (d *Driver) SupportsDriver(name string) bool {
	return name == "avo" || name == "vm"
}

func (d *Driver) Available() error {
	_, err := Locate(d.avoRoot)
	return err
}

func (d *Driver) SetFastStart(bool) {}
func (d *Driver) DropRAM(mobile.Device) {
}

func (d *Driver) dir(dev mobile.Device) string {
	return filepath.Join(d.devices, dev.ID)
}

func (d *Driver) instDir(dev mobile.Device) string {
	return filepath.Join(d.dir(dev), "inst")
}

func runtimeDirFor(id string) string {
	sum := sha256.Sum256([]byte("anyprofile-avo-rt:" + id))
	return filepath.Join(os.TempDir(), "ap-avo-"+hex.EncodeToString(sum[:6]))
}

func (d *Driver) proc(id string) *proc {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.procs[id]
}

// ADB returns the adb client of a running device.
func (d *Driver) ADB(dev mobile.Device) (adb.Client, bool) {
	p := d.proc(dev.ID)
	if p == nil {
		return adb.Client{}, false
	}
	return p.adb, true
}

// Metrics returns recorded lifecycle timings for a device.
func (d *Driver) Metrics(id string) mobile.Metrics {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.metrics[id]
}

func (d *Driver) ensureDisks(ctx context.Context, rt Runtime, dev mobile.Device, inst string) error {
	if exists(filepath.Join(inst, "super.img")) {
		return nil
	}
	var err error
	if dev.WithGMS {
		if _, err = d.PrepareGMSBase(ctx, rt); err != nil {
			return err
		}
		err = cloneGMSDisks(rt, inst)
	} else {
		if _, err = d.PrepareCleanBase(ctx, rt); err != nil {
			return err
		}
		err = cloneCleanDisks(rt, inst)
	}
	if err != nil {
		return err
	}
	// Clean up legacy QEMU disks if this device was migrated from driver "vm".
	devDir := d.dir(dev)
	for _, old := range []string{"vda.qcow2", "vdb.qcow2", "ram.state", "ram.json", "efi_vars.fd", "qemu.log", "serial.log", "clean"} {
		_ = os.RemoveAll(filepath.Join(devDir, old))
	}
	return nil
}

// Create provisions a new Cuttlefish device directory by cloning the pre-warmed
// GMS or clean Cuttlefish disk set via APFS clonefile (<50ms after one-time setup).
func (d *Driver) Create(ctx context.Context, dev mobile.Device) (string, string, error) {
	rt, err := Locate(d.avoRoot)
	if err != nil {
		return "", "", err
	}
	inst := d.instDir(dev)
	if err := os.MkdirAll(inst, 0o700); err != nil {
		return "", "", err
	}
	if err := d.ensureDisks(ctx, rt, dev, inst); err != nil {
		_ = os.RemoveAll(d.dir(dev))
		return "", "", err
	}
	return dev.ID, "cuttlefish-" + rt.ImageVersion, nil
}

// Start boots the Cuttlefish device on vnet with Metal GPU acceleration.
func (d *Driver) Start(ctx context.Context, dev mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	started := time.Now()
	rt, err := Locate(d.avoRoot)
	if err != nil {
		return mobile.Run{}, err
	}
	inst := d.instDir(dev)
	if err := d.ensureDisks(ctx, rt, dev, inst); err != nil {
		return mobile.Run{}, err
	}
	if err := os.MkdirAll(inst, 0o700); err != nil {
		return mobile.Run{}, err
	}

	phase := func(name string) {
		if o.Progress != nil {
			o.Progress(name)
		}
	}
	phase("boot")
	var onReady func(*proc)
	if !o.Headless {
		// Open the Metal viewport immediately (~1s) so users see Android booting live like MuMu.
		onReady = func(p *proc) { d.showOn(ctx, dev, p) }
	}
	p, err := d.launchVMM(ctx, rt, dev, inst, vm.DialerFor(o.Upstream), bootTimeout, onReady)
	if err != nil {
		return mobile.Run{}, err
	}

	phase("setup")
	d.applyPostBoot(ctx, dev, o, p, inst)

	if !o.Headless && p.view == nil {
		phase("window")
		d.showOn(ctx, dev, p)
	}

	run := mobile.Run{
		Launch: mobile.Launch{
			PID:          p.pid,
			PIDStartTime: p.started,
			ADBPort:      p.adb.Port,
		},
		Done: p.exited,
	}
	d.mu.Lock()
	d.procs[dev.ID] = p
	d.metrics[dev.ID] = mobile.Metrics{
		BootMS:   time.Since(started).Milliseconds(),
		StartMS:  time.Since(started).Milliseconds(),
		ADBPort:  p.adb.Port,
		Headless: o.Headless,
	}
	d.mu.Unlock()
	return run, nil
}

func (d *Driver) applyPostBoot(ctx context.Context, dev mobile.Device, o mobile.StartOptions, p *proc, instDir string) {
	// Only push GPU layers if the host marker is missing; otherwise activate props in the single batch below.
	layersReady := p.rt.LayersDir != "" && instDir != "" && exists(filepath.Join(instDir, ".gpu_layers_ready"))
	if !layersReady {
		ensureGPULayers(ctx, p.rt, p.adb, instDir, d.log)
	}
	if err := vm.ApplyLocale(ctx, p.adb, o.Timezone, o.Locale); err != nil {
		d.log.Warn("avo locale", "device", dev.ID, "err", err)
	}
	// Batch clock, automation settings, GPU layer props, lockscreen, spoofing, mute, and initial Wi-Fi kick into one shell call.
	timeout, stayon, anim := "2147483647", "true", "0.5"
	if o.Headless {
		timeout, stayon, anim = "15000", "false", "0"
	} else if dev.Preset == "automation" {
		anim = "0"
	}
	var batch strings.Builder
	fmt.Fprintf(&batch, "settings put global auto_time 0; cmd alarm set-time %d\n", time.Now().UnixMilli())
	fmt.Fprintf(&batch, "settings put global window_animation_scale %s\nsettings put global transition_animation_scale %s\nsettings put global animator_duration_scale %s\nsettings put system screen_off_timeout %s\nsvc power stayon %s\n", anim, anim, anim, timeout, stayon)
	if layersReady {
		batch.WriteString(gpuLayerPropsScript)
		batch.WriteByte('\n')
	}
	batch.WriteString("locksettings set-disabled true >/dev/null 2>&1 || true\nwm dismiss-keyguard\n")
	if dev.IdentityJSON != "" {
		batch.WriteString(vm.BatteryShellScript(dev.ID))
		batch.WriteByte('\n')
		if carrier := vm.CarrierShellScript(o.Country); carrier != "" {
			batch.WriteString(carrier)
			batch.WriteByte('\n')
		}
	}
	if d.WindowMuted(dev) {
		batch.WriteString("cmd media_session volume --stream 3 --set 0\n")
	}
	batch.WriteString(virtWifiScript)
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	_, _ = p.adb.Shell(c, batch.String())
	cancel()

	if o.Lat != nil && o.Lon != nil && vm.ValidCoords(*o.Lat, *o.Lon) {
		jLat, jLon := vm.JitterCoords(dev.ID, *o.Lat, *o.Lon)
		_, _ = sendControl(ctx, p.ctrlSock, fmt.Sprintf("location %.6f %.6f", jLat, jLon))
		if err := vm.ApplyLocation(ctx, p.adb, jLat, jLon); err != nil {
			d.log.Warn("avo gps", "device", dev.ID, "err", err)
		}
	}
}

func ensureGPULayers(ctx context.Context, rt Runtime, a adb.Client, instDir string, log *slog.Logger) {
	if rt.LayersDir == "" {
		return
	}
	marker := filepath.Join(instDir, ".gpu_layers_ready")
	c, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if _, err := a.Shell(c, "test -f /data/local/debug/vulkan/libVkLayer_avo_gfxstream_sync.so && test -f /data/local/debug/gles/libGLES_avo_profile.so && test -f /data/local/debug/vulkan/libVkLayer_avo_profile.so"); err == nil {
		if instDir != "" {
			_ = os.WriteFile(marker, []byte("1\n"), 0o600)
		}
		activateGPULayerProps(c, a)
		return
	}
	if err := adbRootAndWait(c, a, nil); err != nil {
		log.Warn("avo gpu layers root", "err", err)
		return
	}
	_, _ = a.Shell(c, "mkdir -p /data/local/debug/vulkan /data/local/debug/gles && chmod 755 /data/local/debug /data/local/debug/vulkan /data/local/debug/gles")
	pushes := []struct{ src, dst string }{
		{filepath.Join(rt.LayersDir, "libVkLayer_avo_gfxstream_sync.so"), "/data/local/debug/vulkan/.gfxsync.new"},
		{filepath.Join(rt.LayersDir, "libVkLayer_avo_profile.so"), "/data/local/debug/vulkan/.gpuprofile.new"},
		{filepath.Join(rt.LayersDir, "libGLES_avo_profile.so"), "/data/local/debug/gles/.glesprofile.new"},
	}
	for _, f := range pushes {
		if exists(f.src) {
			_, _ = a.Run(c, "push", f.src, f.dst)
		}
	}
	if _, err := a.Shell(c, `set -e
chmod 644 /data/local/debug/vulkan/.*.new /data/local/debug/gles/.*.new 2>/dev/null || true
chcon u:object_r:apk_data_file:s0 /data/local/debug /data/local/debug/vulkan /data/local/debug/gles /data/local/debug/vulkan/.*.new /data/local/debug/gles/.*.new 2>/dev/null || true
[ -f /data/local/debug/vulkan/.gfxsync.new ] && mv -f /data/local/debug/vulkan/.gfxsync.new /data/local/debug/vulkan/libVkLayer_avo_gfxstream_sync.so
[ -f /data/local/debug/vulkan/.gpuprofile.new ] && mv -f /data/local/debug/vulkan/.gpuprofile.new /data/local/debug/vulkan/libVkLayer_avo_profile.so
[ -f /data/local/debug/gles/.glesprofile.new ] && mv -f /data/local/debug/gles/.glesprofile.new /data/local/debug/gles/libGLES_avo_profile.so`); err == nil && instDir != "" {
		_ = os.WriteFile(marker, []byte("1\n"), 0o600)
		activateGPULayerProps(c, a)
	}
}

const gpuLayerPropsScript = `setprop debug.vulkan.layer.9 VK_LAYER_AVO_gfxstream_sync
[ -n "$(getprop debug.avo.gl.renderer)" ] && setprop debug.gles.layers libGLES_avo_profile.so
if [ -n "$(getprop debug.avo.vk.name)" ] || [ -n "$(getprop debug.avo.gl.renderer)" ]; then setprop debug.vulkan.layers VK_LAYER_AVO_profile; fi`

func activateGPULayerProps(ctx context.Context, a adb.Client) {
	_, _ = a.Shell(ctx, gpuLayerPropsScript)
}

const virtWifiScript = `settings put global captive_portal_mode 0
cmd wifi set-wifi-enabled enabled
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16; do
  cmd wifi status 2>/dev/null | grep -q "Wifi is connected" && break
  [ $((i % 4)) -eq 1 ] && cmd wifi connect-network VirtWifi open >/dev/null 2>&1 || true
  sleep 0.25
done`

func (d *Driver) launchVMM(ctx context.Context, rt Runtime, dev mobile.Device, instDir string, dial vnet.Dialer, timeout time.Duration, onControlReady func(*proc)) (*proc, error) {
	rtDir := runtimeDirFor(dev.ID)
	_ = os.RemoveAll(rtDir)
	if err := os.MkdirAll(rtDir, 0o700); err != nil {
		return nil, err
	}
	netSock := filepath.Join(rtDir, "net.sock")
	vsockSock := filepath.Join(rtDir, "vsock.sock")
	ctrlSock := filepath.Join(rtDir, "control.sock")
	mac := vm.MACFor(dev.ID)

	netw, err := vnet.Start(vnet.Config{Dial: dial, MAC: mac, Socket: netSock})
	if err != nil {
		_ = os.RemoveAll(rtDir)
		return nil, err
	}

	w, h, dpi := screenOf(dev)
	cpus := dev.CPUs
	if cpus <= 0 {
		cpus = 4
	}
	mem := dev.RAMMB
	if mem <= 0 {
		mem = 4096
	}
	args := []string{
		"cuttlefish", rt.ImageDir,
		"--instance", instDir,
		"--runtime", rtDir,
		"--cpus", strconv.Itoa(cpus),
		"--mem", strconv.Itoa(mem),
		"--display", fmt.Sprintf("%dx%d", w, h),
		"--density", strconv.Itoa(dpi),
		"--gpu", "gfxstream",
		"--gfxstream-dir", rt.GfxstreamDir,
		"--net-unix", netSock,
		"--net-mac", mac,
		"--no-microphone",
	}
	if dev.Preset == "interactive" {
		args = append(args, "--refresh-rate", "120")
	}

	spec := BuildLaunchSpec(rt, dev, exists(filepath.Join(instDir, ".gpu_layers_ready")))
	if spec.PropertiesText != "" {
		propFile := filepath.Join(instDir, "identity.prop")
		if err := os.WriteFile(propFile, []byte(spec.PropertiesText), 0o600); err == nil {
			args = append(args, "--properties", propFile)
		}
	}
	if spec.Hardware != "" {
		args = append(args, "--hardware", spec.Hardware)
	}
	if spec.CPUClusters != "" {
		args = append(args, "--cpu-clusters", spec.CPUClusters)
	}

	logPath := filepath.Join(instDir, "vmm.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		netw.Close()
		_ = os.RemoveAll(rtDir)
		return nil, err
	}

	cmd := exec.Command(rt.VMM, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = launcher.DetachedAttr()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		netw.Close()
		_ = os.RemoveAll(rtDir)
		return nil, fmt.Errorf("start anyprofile-vmm: %w", err)
	}
	_ = logFile.Close()

	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()

	bridge, err := startADBBridge(vsockSock)
	if err != nil {
		_ = cmd.Process.Kill()
		netw.Close()
		_ = os.RemoveAll(rtDir)
		return nil, err
	}

	st, _ := launcher.ProcessStartTime(cmd.Process.Pid)
	p := &proc{
		pid:      cmd.Process.Pid,
		started:  st,
		adb:      adb.Client{Bin: rt.ADB, Port: bridge.Port},
		bridge:   bridge,
		net:      netw,
		rt:       rt,
		rtDir:    rtDir,
		ctrlSock: ctrlSock,
		exited:   exited,
	}

	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := waitSocket(wctx, ctrlSock, exited); err != nil {
		_ = d.shutdownProc(context.Background(), p)
		return nil, fmt.Errorf("avo-vmm control socket: %w", err)
	}
	if onControlReady != nil {
		onControlReady(p)
	}
	if err := waitBootClean(wctx, p.adb, exited); err != nil {
		tail := logTail(logPath)
		d.closeWindow(p)
		_ = d.shutdownProc(context.Background(), p)
		if tail != "" {
			return nil, fmt.Errorf("android boot: %w (%s)", err, tail)
		}
		return nil, fmt.Errorf("android boot: %w", err)
	}
	return p, nil
}

func waitBootClean(ctx context.Context, a adb.Client, exited <-chan struct{}) error {
	connected := false
	for {
		select {
		case <-ctx.Done():
			return errors.New("Android did not finish starting in time")
		case <-exited:
			return errors.New("the device stopped while starting")
		default:
		}
		c, cancel := context.WithTimeout(ctx, 4*time.Second)
		if !connected {
			a.Connect(c)
		}
		out, err := a.Shell(c, "getprop sys.boot_completed")
		cancel()
		if err == nil && strings.TrimSpace(out) == "1" {
			return nil
		}
		// A successful shell with an unset property means Android is still booting, not offline.
		connected = err == nil
		if !connected {
			a.Disconnect()
		}
		select {
		case <-ctx.Done():
		case <-exited:
		case <-time.After(1 * time.Second):
		}
	}
}

func waitSocket(ctx context.Context, path string, exited <-chan struct{}) error {
	for {
		if exists(path) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return errors.New("vmm exited before ready")
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func screenOf(dev mobile.Device) (w, h, dpi int) {
	w, h, dpi = dev.ScreenW, dev.ScreenH, dev.Density
	if w <= 0 || h <= 0 {
		w, h = 780, 1688
	}
	if dpi <= 0 {
		dpi = 320
	}
	return w, h, dpi
}

// Stop powers off the Cuttlefish device cleanly and releases host resources.
func (d *Driver) Stop(ctx context.Context, dev mobile.Device) error {
	d.mu.Lock()
	p := d.procs[dev.ID]
	delete(d.procs, dev.ID)
	d.mu.Unlock()
	if p == nil {
		return nil
	}
	d.closeWindow(p)
	if p.paused {
		_, _ = sendControl(ctx, p.ctrlSock, "resume")
	}
	return d.shutdownProc(ctx, p)
}

func (d *Driver) shutdownProc(ctx context.Context, p *proc) error {
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	_, _ = p.adb.Shell(c, "svc power stayon false; reboot -p")
	cancel()

	select {
	case <-p.exited:
	case <-time.After(8 * time.Second):
		if procH, err := os.FindProcess(p.pid); err == nil {
			_ = procH.Signal(syscall.SIGTERM)
		}
		select {
		case <-p.exited:
		case <-time.After(5 * time.Second):
			if procH, err := os.FindProcess(p.pid); err == nil {
				_ = procH.Signal(syscall.SIGKILL)
			}
			<-p.exited
		}
	}
	if p.bridge != nil {
		_ = p.bridge.Close()
	}
	if p.net != nil {
		_ = p.net.Close()
	}
	if p.rtDir != "" {
		_ = os.RemoveAll(p.rtDir)
	}
	return nil
}

// Pause freezes the guest vCPUs via avo-vmm control.sock.
func (d *Driver) Pause(id string) error {
	p := d.proc(id)
	if p == nil {
		return errors.New("the device is not running")
	}
	if _, err := sendControl(context.Background(), p.ctrlSock, "pause"); err != nil {
		return err
	}
	d.mu.Lock()
	p.paused = true
	d.mu.Unlock()
	return nil
}

// Resume thaws paused guest vCPUs via avo-vmm control.sock.
func (d *Driver) Resume(id string) error {
	p := d.proc(id)
	if p == nil {
		return errors.New("the device is not running")
	}
	if _, err := sendControl(context.Background(), p.ctrlSock, "resume"); err != nil {
		return err
	}
	d.mu.Lock()
	p.paused = false
	d.mu.Unlock()
	p.adb.Connect(context.Background())
	return nil
}

// Paused reports whether the device's vCPUs are frozen.
func (d *Driver) Paused(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	p := d.procs[id]
	return p != nil && p.paused
}

// Reset performs soft (clear 3rd-party app data) or hard (recreate disks) reset.
func (d *Driver) Reset(ctx context.Context, dev mobile.Device, mode string) error {
	if mode == "soft" {
		p := d.proc(dev.ID)
		if p == nil {
			return errors.New("the device is not running")
		}
		out, err := p.adb.Shell(ctx, "pm list packages -3")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(out, "\n") {
			pkg := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
			if pkg != "" && !strings.ContainsAny(pkg, ";|&$`") {
				_, _ = p.adb.Shell(ctx, "pm clear "+pkg)
			}
		}
		return nil
	}
	if d.proc(dev.ID) != nil {
		if err := d.Stop(ctx, dev); err != nil {
			return err
		}
	}
	inst := d.instDir(dev)
	_ = os.RemoveAll(inst)
	rt, err := Locate(d.avoRoot)
	if err != nil {
		return err
	}
	return d.ensureDisks(ctx, rt, dev, inst)
}

func (d *Driver) Delete(ctx context.Context, dev mobile.Device) error {
	if d.proc(dev.ID) != nil {
		_ = d.Stop(ctx, dev)
	}
	_ = os.RemoveAll(runtimeDirFor(dev.ID))
	return os.RemoveAll(d.dir(dev))
}

func (d *Driver) Backup(ctx context.Context, dev mobile.Device, dir string) error {
	if d.proc(dev.ID) != nil {
		return errors.New("close the device first")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	inst := d.instDir(dev)
	for _, f := range instanceDisks {
		src := filepath.Join(inst, f)
		if !exists(src) {
			return fmt.Errorf("missing device disk %s", f)
		}
		if err := vm.CloneOrCopy(src, filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) Restore(ctx context.Context, dev mobile.Device, dir string) error {
	if d.proc(dev.ID) != nil {
		return errors.New("close the device first")
	}
	for _, f := range instanceDisks {
		if !exists(filepath.Join(dir, f)) {
			return fmt.Errorf("the backup is missing %s", f)
		}
	}
	inst := d.instDir(dev)
	if err := os.MkdirAll(inst, 0o700); err != nil {
		return err
	}
	for _, f := range instanceDisks {
		dst := filepath.Join(inst, f)
		tmp := dst + ".restore"
		_ = os.Remove(tmp)
		if err := vm.CloneOrCopy(filepath.Join(dir, f), tmp); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
	}
	return nil
}

// Adopt reattaches to a running anyprofile-vmm process that survived app restart.
func (d *Driver) Adopt(ctx context.Context, dev mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	if !launcher.SameProcess(l.PID, l.PIDStartTime) {
		return mobile.Run{}, errors.New("the virtual machine is gone")
	}
	rt, err := Locate(d.avoRoot)
	if err != nil {
		return mobile.Run{}, err
	}
	rtDir := runtimeDirFor(dev.ID)
	vsockSock := filepath.Join(rtDir, "vsock.sock")
	ctrlSock := filepath.Join(rtDir, "control.sock")
	bridge, err := startADBBridge(vsockSock)
	if err != nil {
		return mobile.Run{}, err
	}
	netSock := filepath.Join(rtDir, "net.sock")
	if !exists(ctrlSock) {
		_ = bridge.Close()
		return mobile.Run{}, errors.New("control socket missing")
	}
	netw, err := vnet.Start(vnet.Config{Dial: vm.DialerFor(up), MAC: vm.MACFor(dev.ID), Socket: netSock})
	if err != nil {
		_ = bridge.Close()
		return mobile.Run{}, err
	}
	exited := make(chan struct{})
	go func() {
		for launcher.SameProcess(l.PID, l.PIDStartTime) {
			time.Sleep(2 * time.Second)
		}
		close(exited)
	}()
	p := &proc{
		pid:      l.PID,
		started:  l.PIDStartTime,
		adb:      adb.Client{Bin: rt.ADB, Port: bridge.Port},
		bridge:   bridge,
		net:      netw,
		rt:       rt,
		rtDir:    rtDir,
		ctrlSock: ctrlSock,
		exited:   exited,
	}
	p.adb.Connect(ctx)
	d.mu.Lock()
	d.procs[dev.ID] = p
	d.mu.Unlock()
	return mobile.Run{
		Launch: mobile.Launch{PID: l.PID, PIDStartTime: l.PIDStartTime, ADBPort: bridge.Port},
		Done:   exited,
	}, nil
}

func logTail(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

// Keep unused io import guard unnecessary; ensure compile-time interface compliance.
var (
	_ mobile.Driver   = (*Driver)(nil)
	_ mobile.ADBer    = (*Driver)(nil)
	_ mobile.Pauser   = (*Driver)(nil)
	_ mobile.Shower   = (*Driver)(nil)
	_ mobile.Resetter = (*Driver)(nil)
	_ io.Closer       = (*adbBridge)(nil)
)
