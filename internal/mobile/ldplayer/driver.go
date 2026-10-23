// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ldplayer is the Windows mobile driver: each device is an LDPlayer 9
// instance the user's own LDPlayer runs, driven with ldconsole.exe. Its
// network is a SOCKS5 relay in AnyProfile, reached through SocksTun inside
// Android (see tunnel.go). The code builds everywhere so its tests run on
// any machine; the app uses it on Windows only.
package ldplayer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

const (
	bootTimeout = 4 * time.Minute
	quitTimeout = time.Minute
	pollEvery   = 3 * time.Second
	tmpPrefs    = "/data/local/tmp/anyprofile-socksprefs.xml"
	backupFile  = "device.ldbk"
)

// tunnelTimeout is how long the tunnel gets to carry its first connection.
var tunnelTimeout = 30 * time.Second

// Install is where LDPlayer is.
type Install struct {
	Dir, Console, ADB string
}

var (
	errMissing = apperr.New("ldplayer_missing", "LDPlayer 9 is not installed on this computer")
	errGone    = apperr.New("mobile_missing", "this device is no longer in LDPlayer; it was deleted there")
)

type Driver struct {
	find    func() (Install, error)
	runtime string // where the tunnel app is kept
	log     *slog.Logger
	run     runFunc
	poll    time.Duration

	mu    sync.Mutex
	procs map[string]*proc
}

type proc struct {
	name  string
	relay *netproxy.SOCKSRelay
	adb   adb.Client
	stop  chan struct{} // closes the watcher early (Delete)
}

// New makes the driver; runtimeDir keeps the tunnel app between uses.
func New(runtimeDir string, log *slog.Logger) *Driver {
	return &Driver{find: Find, runtime: runtimeDir, log: log, run: execRun, poll: pollEvery, procs: map[string]*proc{}}
}

func (d *Driver) Name() string { return "ldplayer" }

func (d *Driver) Available() error {
	_, err := d.find()
	return err
}

func instanceName(dev mobile.Device) string {
	if dev.Ref != "" {
		return dev.Ref
	}
	return "ap-" + dev.ID
}

func (d *Driver) console() (console, Install, error) {
	in, err := d.find()
	if err != nil {
		return console{}, in, err
	}
	return console{bin: in.Console, run: d.run}, in, nil
}

func (d *Driver) Create(ctx context.Context, dev mobile.Device) (string, string, error) {
	c, _, err := d.console()
	if err != nil {
		return "", "", err
	}
	name := "ap-" + dev.ID
	if _, ok, err := c.find(ctx, name); err != nil {
		return "", "", err
	} else if !ok {
		if _, err := c.do(ctx, "add", "--name", name); err != nil {
			return "", "", err
		}
	}
	if err := d.modify(ctx, c, name, dev); err != nil {
		c.do(ctx, "remove", "--name", name)
		return "", "", err
	}
	return name, "ldplayer", nil
}

// modify sets the instance's hardware and turns on root, which the tunnel's
// setup needs. LDPlayer applies it at the next launch.
func (d *Driver) modify(ctx context.Context, c console, name string, dev mobile.Device) error {
	_, err := c.do(ctx, "modify", "--name", name, "--cpu", strconv.Itoa(dev.CPUs),
		"--memory", strconv.Itoa(dev.RAMMB), "--resolution", "1080,2400,420", "--root", "1")
	return err
}

func (d *Driver) Start(ctx context.Context, dev mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	c, inst, err := d.console()
	if err != nil {
		return mobile.Run{}, err
	}
	name := instanceName(dev)
	in, ok, err := c.find(ctx, name)
	if err != nil {
		return mobile.Run{}, err
	}
	if !ok {
		return mobile.Run{}, errGone
	}
	if !in.Started {
		if err := d.modify(ctx, c, name, dev); err != nil {
			return mobile.Run{}, err
		}
	}
	p := &proc{name: name, adb: adb.Client{Bin: inst.ADB, Port: adbPort(in.Index)}, stop: make(chan struct{})}
	if o.Upstream != nil {
		if p.relay, err = netproxy.StartSOCKS(*o.Upstream, 0); err != nil {
			return mobile.Run{}, err
		}
	}
	fail := func(err error) (mobile.Run, error) {
		c.do(context.Background(), "quit", "--name", name)
		if p.relay != nil {
			p.relay.Close()
		}
		p.adb.Disconnect()
		return mobile.Run{}, err
	}
	if _, err := c.do(ctx, "launch", "--name", name); err != nil {
		return fail(err)
	}
	bctx, cancel := context.WithTimeout(ctx, bootTimeout)
	err = p.adb.WaitBoot(bctx, nil)
	cancel()
	if err != nil {
		return fail(err)
	}
	if err := d.network(ctx, p); err != nil {
		return fail(fmt.Errorf("proxy tunnel: %w", err))
	}
	if err := d.locale(ctx, p.adb, o.Timezone, o.Locale); err != nil {
		d.log.Warn("ldplayer locale", "device", dev.ID, "err", err)
	}
	if in, _, err = c.find(ctx, name); err != nil {
		return fail(err)
	}
	return d.track(dev.ID, c, p, in.PID), nil
}

// network puts the device on its proxy through the tunnel, or on the plain
// network for a device without one.
func (d *Driver) network(ctx context.Context, p *proc) error {
	root, err := asRoot(ctx, p.adb)
	if err != nil {
		return err
	}
	if p.relay == nil {
		_, err := p.adb.Shell(ctx, root(tunnelOff()))
		return err
	}
	if err := d.ensureTunnel(ctx, p.adb); err != nil {
		return err
	}
	port := strconv.Itoa(p.relay.Port())
	if _, err := p.adb.Run(ctx, "reverse", "tcp:"+port, "tcp:"+port); err != nil {
		return err
	}
	prefs := filepath.Join(os.TempDir(), "anyprofile-socksprefs-"+port+".xml")
	if err := os.WriteFile(prefs, []byte(tunnelPrefsXML(p.relay.Port())), 0o600); err != nil {
		return err
	}
	defer os.Remove(prefs)
	if _, err := p.adb.Run(ctx, "push", prefs, tmpPrefs); err != nil {
		return err
	}
	if _, err := p.adb.Shell(ctx, root(tunnelSetup(tmpPrefs))); err != nil {
		return err
	}
	// Android checks its connection as soon as the VPN is up; that first
	// connection reaching the relay proves the tunnel carries traffic.
	select {
	case <-p.relay.Used():
		return nil
	case <-time.After(tunnelTimeout):
		return errors.New("the tunnel app did not connect to AnyProfile's proxy relay")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// asRoot returns how to run a line as root: LDPlayer's adb is root itself
// when root is on, older setups go through su. Lines never hold a single quote.
func asRoot(ctx context.Context, a adb.Client) (func(string) string, error) {
	out, err := a.Shell(ctx, "id -u")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "0" {
		return func(l string) string { return l }, nil
	}
	return func(l string) string { return "su -c '" + l + "'" }, nil
}

// ensureTunnel installs SocksTun when the device does not have it.
func (d *Driver) ensureTunnel(ctx context.Context, a adb.Client) error {
	if out, _ := a.Shell(ctx, "pm path "+tunnelPkg); strings.HasPrefix(out, "package:") {
		return nil
	}
	apk, err := d.tunnelAPK(ctx)
	if err != nil {
		return err
	}
	_, err = a.Run(ctx, "install", "-r", apk)
	return err
}

// tunnelAPK is SocksTun's release, downloaded once and checked by hash.
func (d *Driver) tunnelAPK(ctx context.Context) (string, error) {
	p := filepath.Join(d.runtime, "sockstun-"+tunnelVersion+".apk")
	if engine.VerifySHA256(p, tunnelSHA256) == nil {
		return p, nil
	}
	if err := os.MkdirAll(d.runtime, 0o700); err != nil {
		return "", err
	}
	tmp := p + ".part"
	if err := engine.Download(ctx, tunnelURL, tmp, nil); err != nil {
		return "", err
	}
	if err := engine.VerifySHA256(tmp, tunnelSHA256); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// locale sets the timezone at once and the language for the next start:
// LDPlayer 9 is Android 9, where the language is the persist.sys.locale
// property, read at boot.
func (d *Driver) locale(ctx context.Context, a adb.Client, tz, locale string) error {
	var cmds []string
	if tz != "" && mobile.ValidTimezone(tz) {
		cmds = append(cmds, "settings put global auto_time_zone 0", "setprop persist.sys.timezone "+tz,
			"am broadcast -a android.intent.action.TIMEZONE_CHANGED --es time-zone "+tz)
	}
	if locale != "" && mobile.ValidLocale(locale) {
		cmds = append(cmds, "setprop persist.sys.locale "+locale)
	}
	if len(cmds) == 0 {
		return nil
	}
	root, err := asRoot(ctx, a)
	if err != nil {
		return err
	}
	_, err = a.Shell(ctx, root(strings.Join(cmds, "; ")))
	return err
}

// track watches a running instance; Done closes once LDPlayer reports it stopped.
func (d *Driver) track(id string, c console, p *proc, pid int) mobile.Run {
	d.mu.Lock()
	d.procs[id] = p
	d.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-p.stop:
			case <-time.After(d.poll):
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				up := c.running(ctx, p.name)
				cancel()
				if up {
					continue
				}
			}
			break
		}
		d.mu.Lock()
		if d.procs[id] == p {
			delete(d.procs, id)
		}
		d.mu.Unlock()
		if p.relay != nil {
			p.relay.Close()
		}
		p.adb.Disconnect()
	}()
	l := mobile.Launch{PID: pid, ADBPort: p.adb.Port}
	if p.relay != nil {
		l.RelayPort = p.relay.Port()
	}
	return mobile.Run{Launch: l, Done: done}
}

func (d *Driver) proc(id string) *proc {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.procs[id]
}

func (d *Driver) Stop(ctx context.Context, dev mobile.Device) error {
	c, _, err := d.console()
	if err != nil {
		return err
	}
	name := instanceName(dev)
	if _, err := c.do(ctx, "quit", "--name", name); err != nil {
		return err
	}
	deadline := time.Now().Add(quitTimeout)
	for c.running(ctx, name) {
		if time.Now().After(deadline) {
			return errors.New("LDPlayer did not close the device")
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (d *Driver) Delete(ctx context.Context, dev mobile.Device) error {
	c, _, err := d.console()
	if err != nil {
		return err
	}
	name := instanceName(dev)
	if p := d.proc(dev.ID); p != nil {
		close(p.stop)
	}
	if _, ok, err := c.find(ctx, name); err != nil || !ok {
		return err // already gone in LDPlayer
	}
	if c.running(ctx, name) {
		c.do(ctx, "quit", "--name", name)
	}
	_, err = c.do(ctx, "remove", "--name", name)
	return err
}

func (d *Driver) Backup(ctx context.Context, dev mobile.Device, dir string) error {
	c, _, err := d.console()
	if err != nil {
		return err
	}
	_, err = c.do(ctx, "backup", "--name", instanceName(dev), "--file", filepath.Join(dir, backupFile))
	return err
}

func (d *Driver) Restore(ctx context.Context, dev mobile.Device, dir string) error {
	c, _, err := d.console()
	if err != nil {
		return err
	}
	f := filepath.Join(dir, backupFile)
	if _, err := os.Stat(f); err != nil {
		return fmt.Errorf("the backup is missing %s", backupFile)
	}
	_, err = c.do(ctx, "restore", "--name", instanceName(dev), "--file", f)
	return err
}

// Adopt takes back an instance that kept running while the app was away. Its
// tunnel points at the old relay port, so the relay comes back there; when
// that port is taken, the tunnel is set up again on a new one.
func (d *Driver) Adopt(ctx context.Context, dev mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	c, inst, err := d.console()
	if err != nil {
		return mobile.Run{}, err
	}
	name := instanceName(dev)
	in, ok, err := c.find(ctx, name)
	if err != nil || !ok || !c.running(ctx, name) {
		return mobile.Run{}, errors.New("the device is not running")
	}
	p := &proc{name: name, adb: adb.Client{Bin: inst.ADB, Port: adbPort(in.Index)}, stop: make(chan struct{})}
	p.adb.Connect(ctx)
	if up != nil {
		reused := false
		if l.RelayPort > 0 {
			if p.relay, err = netproxy.StartSOCKS(*up, l.RelayPort); err == nil {
				port := strconv.Itoa(p.relay.Port())
				if _, err := p.adb.Run(ctx, "reverse", "tcp:"+port, "tcp:"+port); err != nil {
					p.relay.Close()
					return mobile.Run{}, err
				}
				reused = true
			}
		}
		if !reused {
			if p.relay, err = netproxy.StartSOCKS(*up, 0); err != nil {
				return mobile.Run{}, err
			}
			if err := d.network(ctx, p); err != nil {
				p.relay.Close()
				return mobile.Run{}, err
			}
		}
	}
	return d.track(dev.ID, c, p, in.PID), nil
}
