// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Mobile devices are an AnyProfile Pro feature: creating and opening one
// needs a paid-up key. Listing, editing, backing up and deleting do not, so
// a lapsed subscription never holds a device's data hostage.

const (
	defaultMobileRAMMB = 3072
	defaultMobileCPUs  = 4
	minMobileRAMMB     = 2048
	maxMobileRAMMB     = 6144
	maxMobileCPUs      = 8
	minScreenW         = 360
	maxScreenW         = 1080
	minScreenH         = 640
	maxScreenH         = 2400
	minDensity         = 160
	maxDensity         = 480
	// mobileDiskNeed is the room a new device needs: about 1.5 GB after its
	// first boot, and some to spare.
	mobileDiskNeed = 3 << 30
)

// MobileAvailability tells the UI what stands between the user and a device.
type MobileAvailability struct {
	// Install is set when the driver downloads what it runs on (macOS).
	Install   *mobile.InstallStatus `json:"install"`
	Supported bool                  `json:"supported"` // this OS has a driver
	Driver    string                `json:"driver"`    // "vm" or "ldplayer"
	Pro       bool                  `json:"pro"`       // a paid-up AnyProfile Pro key
	Ready     bool                  `json:"ready"`     // the driver can create and open devices
	Reason    string                `json:"reason"`    // apperr code when not ready, like "ldplayer_missing"
	Message   string                `json:"message"`
}

// MobileRow is a device with its live state.
type MobileRow struct {
	store.MobileDevice
	State   mobile.State `json:"state"`
	Error   string       `json:"error"`
	ADBPort int          `json:"adbPort,omitempty"`
}

// MobileInput is what the user sets on a device.
type MobileInput struct {
	Name    string   `json:"name"`
	ProxyID string   `json:"proxyId"` // AutoProxy picks the oldest free sticky proxy
	RAMMB   int      `json:"ramMb"`   // 0: the preset's default
	CPUs    int      `json:"cpus"`    // 0: the preset's default
	Preset  string   `json:"preset"`  // automation, balanced or interactive
	ScreenW int      `json:"screenW"` // 0: the preset's default
	ScreenH int      `json:"screenH"`
	Density int      `json:"density"`
	Notes   string   `json:"notes"`
	Tags    []string `json:"tags"`
	Pinned  bool     `json:"pinned"`
}

// MobileStart is what opening a device found worth telling.
type MobileStart struct {
	// Warning is "ram_tight" when more devices run than this computer's
	// memory comfortably holds; the device opened anyway.
	Warning string `json:"warning"`
}

var errMobileUnsupported = apperr.New("mobile_unsupported", "mobile devices are not available on this computer yet")

// mobileAllowed is the Pro gate. A signed certificate is authoritative; the
// old unsigned pro_state is only a migration fallback on computers that have
// not reached the v2 server yet. It never calls the server.
func (s *Service) mobileAllowed() error {
	if cert, err := s.verifiedCert(); err == nil {
		if cert.Allowed(s.now()) && cert.Has("mobile") {
			return nil
		}
		return apperr.New("pro_required", "mobile devices need AnyProfile Pro")
	}
	if s.proCertToken() != "" {
		return apperr.New("pro_required", "mobile devices need AnyProfile Pro")
	}
	st := s.ProStatus()
	if st.Active && (st.State == "active" || st.State == "past_due") {
		return nil
	}
	return apperr.New("pro_required", "mobile devices need AnyProfile Pro")
}

func (s *Service) MobileAvailability() MobileAvailability {
	a := MobileAvailability{Pro: s.mobileAllowed() == nil}
	if s.Mobile == nil {
		a.Reason, a.Message = "mobile_unsupported", errMobileUnsupported.Msg
		return a
	}
	a.Supported, a.Driver = true, s.Mobile.Driver().Name()
	if in, ok := s.Mobile.Driver().(mobile.Installer); ok {
		st := in.InstallStatus()
		a.Install = &st
	}
	if err := s.Mobile.Driver().Available(); err != nil {
		a.Reason, a.Message = codeOf(err), err.Error()
		var ae *apperr.Error
		if errors.As(err, &ae) {
			a.Message = ae.Msg
		}
		return a
	}
	a.Ready = true
	return a
}

func codeOf(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return "mobile_unavailable"
}

// imageDiskNeed is the room installing the image takes: the 1.1 GB zip and
// the 2.3 GB it unpacks to, both on disk for a moment.
const imageDiskNeed = 4 << 30

// OfferMobileRuntime hands the mobile driver the runtimes the last Pro check
// offered (none without Pro), as releases it can install.
func (s *Service) OfferMobileRuntime() {
	if s.Mobile == nil {
		return
	}
	o, ok := s.Mobile.Driver().(mobile.RuntimeOfferer)
	if !ok {
		return
	}
	var rs []engine.Release
	if s.mobileAllowed() == nil {
		for _, r := range s.proSaved().Runtimes {
			rs = append(rs, engine.Release{Engine: r.Engine, Version: r.Version, Chromium: r.QEMU, Platform: r.Platform,
				URL: r.URL, SHA256: r.SHA256, Files: r.Files, Bin: r.Bin, Catalogue: r.Engine})
		}
	}
	if err := o.OfferRuntime(rs); err != nil && s.Log != nil {
		s.Log.Warn("mobile: runtime offer", "err", err)
	}
}

// MobileRuntimeSourceURL links to the complete source of the AnyMobile runtime
// Pro offers (QEMU, its libraries, the patches and build scripts), as the GPL
// asks; the link is fresh from the license server.
func (s *Service) MobileRuntimeSourceURL(ctx context.Context) (string, error) {
	if err := s.mobileAllowed(); err != nil {
		return "", err
	}
	if _, err := s.ProCheck(ctx, true); err != nil {
		return "", err
	}
	for _, r := range s.proSaved().Runtimes {
		if r.Platform == engine.HostPlatform() && r.SourceURL != "" {
			return r.SourceURL, nil
		}
	}
	return "", apperr.New("anymobile_runtime_unavailable", "the Mobile runtime is not published for this computer yet")
}

// InstallMobileRuntime downloads the VM runtime and the Android image (macOS).
// It is part of Pro, as the devices are: the runtime comes from the license
// server, asked again first because its links last minutes.
func (s *Service) InstallMobileRuntime(ctx context.Context, progress func(engine string, done, total int64)) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	if s.Mobile == nil {
		return errMobileUnsupported
	}
	in, ok := s.Mobile.Driver().(mobile.Installer)
	if !ok {
		return apperr.New("mobile_nothing_to_install", "this computer has nothing to download for mobile devices")
	}
	if _, ok := in.(mobile.RuntimeOfferer); ok {
		if _, err := s.ProCheck(ctx, true); err != nil {
			return err
		}
	}
	if st := in.InstallStatus(); st.Image.Installed == "" {
		if err := os.MkdirAll(s.Dirs.MobileRuntime(), 0o700); err != nil {
			return err
		}
		if free, err := mobile.FreeBytes(s.Dirs.MobileRuntime()); err == nil && free < imageDiskNeed {
			return apperr.New("disk_full_image", "free at least 4 GB of disk space to download Android")
		}
	}
	if err := in.Install(ctx, progress); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// mobileReady is the gate for creating and opening: Pro, a driver, and the
// driver ready.
func (s *Service) mobileReady() error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	if s.Mobile == nil {
		return errMobileUnsupported
	}
	return s.Mobile.Driver().Available()
}

func (s *Service) ListMobileDevices(f store.ProfileFilter) ([]MobileRow, error) {
	ds, err := s.Store.ListMobileDevices(f)
	if err != nil {
		return nil, err
	}
	out := make([]MobileRow, 0, len(ds))
	for _, d := range ds {
		row := MobileRow{MobileDevice: d, State: mobile.Stopped}
		if s.Mobile != nil {
			in := s.Mobile.Info(d.ID)
			row.State, row.Error, row.ADBPort = in.State, in.Error, in.ADBPort
		}
		out = append(out, row)
	}
	return out, nil
}

type mobilePreset struct {
	RAM, CPUs, ScreenW, ScreenH, Density int
}

var mobilePresets = map[string]mobilePreset{
	"automation":  {2048, 2, 540, 960, 240},
	"balanced":    {3072, 4, 720, 1600, 280},
	"interactive": {4096, 4, 720, 1600, 280},
}

// checkMobileInput fills the defaults and refuses what a device cannot have.
func checkMobileInput(in *MobileInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.New("mobile_name_required", "device name is required")
	}
	if in.Preset == "" {
		in.Preset = "balanced"
	}
	p, ok := mobilePresets[in.Preset]
	if !ok {
		return apperr.Newf("mobile_bad_preset", "preset is automation, balanced or interactive")
	}
	if in.RAMMB == 0 {
		in.RAMMB = p.RAM
	}
	if in.CPUs == 0 {
		in.CPUs = p.CPUs
	}
	if in.ScreenW == 0 {
		in.ScreenW = p.ScreenW
	}
	if in.ScreenH == 0 {
		in.ScreenH = p.ScreenH
	}
	if in.Density == 0 {
		in.Density = p.Density
	}
	if in.RAMMB < minMobileRAMMB || in.RAMMB > maxMobileRAMMB {
		return apperr.Newf("mobile_bad_ram", "memory must be between %d and %d MB", minMobileRAMMB, maxMobileRAMMB)
	}
	if in.CPUs < 1 || in.CPUs > maxMobileCPUs {
		return apperr.Newf("mobile_bad_cpus", "between 1 and %d CPU cores", maxMobileCPUs)
	}
	if in.ScreenW < minScreenW || in.ScreenW > maxScreenW || in.ScreenH < minScreenH || in.ScreenH > maxScreenH {
		return apperr.Newf("mobile_bad_screen", "screen must be between %dx%d and %dx%d", minScreenW, minScreenH, maxScreenW, maxScreenH)
	}
	if in.Density < minDensity || in.Density > maxDensity {
		return apperr.Newf("mobile_bad_density", "density must be between %d and %d", minDensity, maxDensity)
	}
	return nil
}

func mobileDevice(d store.MobileDevice) mobile.Device {
	return mobile.Device{ID: d.ID, Name: d.Name, Ref: d.DriverRef, RAMMB: d.RAMMB, CPUs: d.CPUs,
		Preset: d.Preset, ScreenW: d.ScreenW, ScreenH: d.ScreenH, Density: d.Density}
}

// CreateMobileDevice records the device, then has the driver make it; the
// first boot can take minutes. A device the driver could not make is removed.
func (s *Service) CreateMobileDevice(ctx context.Context, in MobileInput) (store.MobileDevice, error) {
	if err := checkMobileInput(&in); err != nil {
		return store.MobileDevice{}, err
	}
	if err := s.mobileReady(); err != nil {
		return store.MobileDevice{}, err
	}
	if free, err := mobile.FreeBytes(s.Dirs.Mobile()); err == nil && free < mobileDiskNeed {
		return store.MobileDevice{}, apperr.New("disk_full", "free at least 3 GB of disk space first")
	}
	if in.ProxyID == AutoProxy {
		id, err := s.nextFreeProxy()
		if err != nil {
			return store.MobileDevice{}, err
		}
		in.ProxyID = id
	}
	if in.ProxyID != "" {
		// The same checks as opening, so a device is not made for a proxy it cannot use.
		if _, err := s.proxyNetwork(ctx, in.ProxyID); err != nil {
			return store.MobileDevice{}, err
		}
	}
	d := store.MobileDevice{ID: randomHex(8), Name: in.Name, Driver: s.Mobile.Driver().Name(),
		Platform: runtime.GOOS, ProxyID: in.ProxyID, RAMMB: in.RAMMB, CPUs: in.CPUs,
		Preset: in.Preset, ScreenW: in.ScreenW, ScreenH: in.ScreenH, Density: in.Density,
		Notes: in.Notes, Tags: in.Tags, Pinned: in.Pinned}
	if err := s.Store.CreateMobileDevice(&d); err != nil {
		return d, err
	}
	s.notify("mobile")
	ref, img, err := s.Mobile.Create(ctx, mobileDevice(d))
	if err != nil {
		if perr := s.Store.PurgeMobileDevice(d.ID); perr != nil {
			s.Log.Warn("remove a device that could not be made", "device", d.ID, "err", perr)
		}
		os.RemoveAll(s.Dirs.MobileDevice(d.ID))
		s.notify("mobile")
		return d, err
	}
	if err := s.Store.SetMobileDriverState(d.ID, ref, img); err != nil {
		return d, err
	}
	d.DriverRef, d.ImageVersion = ref, img
	s.notify("mobile")
	return d, nil
}

// UpdateMobileDevice saves the editable fields. The proxy, memory and cores
// change only while the device is stopped.
func (s *Service) UpdateMobileDevice(ctx context.Context, id string, in MobileInput) error {
	if err := checkMobileInput(&in); err != nil {
		return err
	}
	d, err := s.Store.GetMobileDevice(id)
	if err != nil {
		return err
	}
	if in.ProxyID != d.ProxyID || in.RAMMB != d.RAMMB || in.CPUs != d.CPUs {
		if s.mobileRunning(id) {
			return apperr.New("mobile_running", "close the device first")
		}
		if in.ProxyID != d.ProxyID && in.ProxyID != "" {
			if _, err := s.proxyNetwork(ctx, in.ProxyID); err != nil {
				return err
			}
		}
	}
	d.Name, d.ProxyID, d.RAMMB, d.CPUs, d.Preset, d.ScreenW, d.ScreenH, d.Density, d.Notes, d.Tags, d.Pinned =
		in.Name, in.ProxyID, in.RAMMB, in.CPUs, in.Preset, in.ScreenW, in.ScreenH, in.Density,
		in.Notes, in.Tags, in.Pinned
	if err := s.Store.UpdateMobileDevice(d); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// mobileFastStartKey stores whether Stop saves the RAM state for a fast thaw.
const mobileFastStartKey = "mobile_fast_start"

// MobileFastStart reports the setting; unset means on.
func (s *Service) MobileFastStart() (bool, error) {
	v, err := s.Store.GetSetting(mobileFastStartKey)
	if err != nil {
		return true, err
	}
	return v != "0", nil
}

// SetMobileFastStart stores the setting and applies it to the driver. Turning
// it off deletes the saved states, since they are ~2.5 GB each.
func (s *Service) SetMobileFastStart(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := s.Store.SetSetting(mobileFastStartKey, v); err != nil {
		return err
	}
	s.applyMobileFastStart()
	s.notify("settings")
	return nil
}

func (s *Service) applyMobileFastStart() {
	if s.Mobile == nil {
		return
	}
	p, ok := s.Mobile.Driver().(interface{ SetFastStart(bool) })
	if !ok {
		return
	}
	on, err := s.MobileFastStart()
	if err != nil {
		return
	}
	p.SetFastStart(on)
}

// mobileIdlePause is how long a device may sit untouched before its vCPUs are
// frozen. An agent resets it on every tool call; the guest keeps its RAM and
// its apps, so Resume is a thaw (~60 ms), not a boot.
const mobileIdlePause = 2 * time.Minute

// scheduleMobilePause (re)arms the idle freeze for a headless device.
func (s *Service) scheduleMobilePause(id string) {
	if s.Mobile == nil {
		return
	}
	p, ok := s.Mobile.Driver().(mobile.Pauser)
	if !ok {
		return
	}
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if s.pauseTimers == nil {
		s.pauseTimers = map[string]*time.Timer{}
	}
	if t := s.pauseTimers[id]; t != nil {
		t.Stop()
	}
	s.pauseTimers[id] = time.AfterFunc(mobileIdlePause, func() {
		if s.Mobile.Info(id).State != mobile.Running {
			return
		}
		if p.Windowed(id) || s.mobileRecording(id) {
			// A window is open or the screen is being recorded: check again
			// next round. Once both end, the device can freeze.
			s.scheduleMobilePause(id)
			return
		}
		if err := p.Pause(id); err != nil {
			s.Log.Debug("mobile pause", "device", id, "err", err)
			return
		}
		s.Log.Info("mobile paused while idle", "device", id)
	})
}

// forgetMobilePause stops a pending freeze; a stopped device has nothing to
// freeze. It does not resume: the driver does that before a clean stop.
func (s *Service) forgetMobilePause(id string) {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if t := s.pauseTimers[id]; t != nil {
		t.Stop()
		delete(s.pauseTimers, id)
	}
}

// resumeMobile thaws a paused device before an action that talks to it.
func (s *Service) resumeMobile(id string) error {
	if s.Mobile == nil {
		return nil
	}
	if p, ok := s.Mobile.Driver().(mobile.Pauser); ok && p.Paused(id) {
		return p.Resume(id)
	}
	return nil
}

func (s *Service) mobileRunning(id string) bool {
	return s.Mobile != nil && s.Mobile.Info(id).State != mobile.Stopped
}

// liveMobile is a device that may be driven here: not trashed, made on this OS
// by this computer's driver.
func (s *Service) liveMobile(id string) (store.MobileDevice, error) {
	d, err := s.Store.GetMobileDevice(id)
	if err != nil {
		return d, err
	}
	if d.DeletedAt != "" {
		return d, apperr.New("mobile_trashed", "restore the device from the trash first")
	}
	if d.Platform != runtime.GOOS || s.Mobile == nil || d.Driver != s.Mobile.Driver().Name() {
		return d, apperr.New("mobile_other_os", "this device was made on another operating system")
	}
	return d, nil
}

// StartMobileDevice opens a device with its proxy, timezone and language,
// measured again on every open so a changed exit is followed.
func (s *Service) StartMobileDevice(ctx context.Context, id string) (MobileStart, error) {
	return s.startMobile(ctx, id, false)
}

// StartMobileDeviceHeadless opens a device for an agent or script: no scrcpy
// window, everything else is the same as the user's Open button.
func (s *Service) StartMobileDeviceHeadless(ctx context.Context, id string) (MobileStart, error) {
	return s.startMobile(ctx, id, true)
}

func (s *Service) startMobile(ctx context.Context, id string, headless bool) (MobileStart, error) {
	started := time.Now()
	s.applyMobileFastStart()
	if err := s.mobileReady(); err != nil {
		return MobileStart{}, err
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return MobileStart{}, err
	}
	proxyStarted := time.Now()
	net, err := s.proxyNetwork(ctx, d.ProxyID)
	if err != nil {
		return MobileStart{}, err
	}
	proxyMS := time.Since(proxyStarted).Milliseconds()
	o := mobile.StartOptions{Timezone: net.Timezone, Locale: androidLocale(net), Headless: headless}
	if d.ProxyID != "" {
		up, err := s.proxyUpstream(d.ProxyID)
		if err != nil {
			return MobileStart{}, err
		}
		o.Upstream = &up
	}
	var res MobileStart
	if tight, err := s.ramTight(); err == nil && tight {
		res.Warning = "ram_tight"
	}
	if err := s.Mobile.Start(ctx, mobileDevice(d), o); err != nil {
		return res, err
	}
	s.ctlMu.Lock()
	if s.mobileLocale == nil {
		s.mobileLocale = map[string]string{}
	}
	s.mobileLocale[id] = o.Locale
	s.ctlMu.Unlock()
	s.Log.Info("mobile open timing", "device", id, "proxy_ms", proxyMS, "total_ms", time.Since(started).Milliseconds())
	if err := s.Store.MarkMobileOpened(id); err != nil {
		s.Log.Warn("mark mobile device opened", "device", id, "err", err)
	}
	s.scheduleMobilePause(id)
	s.notify("mobile")
	return res, nil
}

// ResetMobileDevice returns a device to a clean state. soft clears the data
// of third-party apps on a running device; hard stops it, restores the clean
// disks saved when it was made, and leaves it stopped.
func (s *Service) ResetMobileDevice(ctx context.Context, id, mode string) error {
	if mode != "soft" && mode != "hard" {
		return mobile.ErrBadReset
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return err
	}
	if err := s.resumeMobile(id); err != nil {
		return err
	}
	r, ok := s.Mobile.Driver().(mobile.Resetter)
	if !ok {
		return errMobileUnsupported
	}
	if mode == "hard" {
		if s.mobileRunning(id) {
			if err := s.StopMobileDevice(ctx, id); err != nil {
				return err
			}
		}
		err = s.Mobile.Busy(id, mobile.Working, func() error { return r.Reset(ctx, mobileDevice(d), mode) })
	} else {
		if !s.mobileRunning(id) {
			return mobile.ErrNotRunning
		}
		err = r.Reset(ctx, mobileDevice(d), mode)
	}
	if err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// MobileMetrics returns what this app run measured for a device.
func (s *Service) MobileMetrics(id string) (mobile.Metrics, error) {
	if s.Mobile == nil {
		return mobile.Metrics{}, errMobileUnsupported
	}
	m, ok := s.Mobile.Driver().(interface{ Metrics(string) mobile.Metrics })
	if !ok {
		return mobile.Metrics{}, errMobileUnsupported
	}
	out := m.Metrics(id)
	s.ctlMu.Lock()
	if c := s.controls[id]; c != nil {
		cm := c.Metrics()
		out.Actions, out.LastActionMS, out.SnapshotMS = cm.Actions, cm.LastActionMS, cm.SnapshotMS
	}
	s.ctlMu.Unlock()
	return out, nil
}

// forgetControl closes a cached agent handle when its device stops.
func (s *Service) forgetControl(id string) {
	s.ctlMu.Lock()
	defer s.ctlMu.Unlock()
	if c := s.controls[id]; c != nil {
		c.Close()
		delete(s.controls, id)
	}
}

// androidLocale is the exit's first language, like "ja-JP".
func androidLocale(n exitNetwork) string {
	if len(n.Languages) > 0 {
		return n.Languages[0]
	}
	return n.UILocale
}

// mobileSlots is how many devices this much memory runs without swapping hard.
func mobileSlots(ram int64) int {
	switch gib := ram >> 30; {
	case gib <= 8:
		return 1
	case gib <= 16:
		return 2
	case gib <= 24:
		return 3
	default:
		return 4
	}
}

// ramTight is true when one more device exceeds mobileSlots.
func (s *Service) ramTight() (bool, error) {
	ram, err := mobile.TotalRAM()
	if err != nil {
		return false, err
	}
	return s.Mobile.Running()+1 > mobileSlots(ram), nil
}

func (s *Service) StopMobileDevice(ctx context.Context, id string) error {
	if s.Mobile == nil {
		return nil
	}
	s.applyMobileFastStart()
	d, err := s.Store.GetMobileDevice(id)
	if err != nil {
		return err
	}
	s.forgetMobilePause(id)
	if err := s.Mobile.Stop(ctx, mobileDevice(d)); err != nil {
		return err
	}
	s.forgetControl(id)
	s.recMu.Lock()
	delete(s.recordings, id) // the recording died with the device
	s.recMu.Unlock()
	s.notify("mobile")
	return nil
}

// MobilePreviewFrame returns the screen as base64 JPEG for the in-app viewer.
// It is a preview (about 2-3 frames a second, depending on the device), not
// the smooth scrcpy window.
func (s *Service) MobilePreviewFrame(ctx context.Context, id string) (string, error) {
	c, err := s.MobileControl(id)
	if err != nil {
		return "", err
	}
	img, err := c.Screenshot(ctx)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(img), nil
}

// TapMobileDevice taps a point in the device's 720x1600 screen space.
func (s *Service) TapMobileDevice(ctx context.Context, id string, x, y int) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	return c.Tap(ctx, "", x, y)
}

// PressMobileKey presses home, back or recents from the in-app viewer.
func (s *Service) PressMobileKey(ctx context.Context, id, key string) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	return c.PressKey(ctx, key)
}

// ScreenshotMobileDevice saves a JPEG of the Android screen under
// ~/Pictures/AnyProfile and opens it.
func (s *Service) ScreenshotMobileDevice(ctx context.Context, id string) (string, error) {
	c, err := s.MobileControl(id)
	if err != nil {
		return "", err
	}
	img, err := c.Screenshot(ctx)
	if err != nil {
		return "", err
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Pictures", "AnyProfile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := safeFileName(d.Name) + "-" + time.Now().Format("20060102-150405") + ".jpg"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, img, 0o644); err != nil {
		return "", err
	}
	openPath(path)
	return path, nil
}

// MobileWindowOnTop reports the stored always-on-top flag.
func (s *Service) MobileWindowOnTop(id string) (bool, error) {
	d, err := s.liveMobile(id)
	if err != nil {
		return false, err
	}
	w, ok := s.Mobile.Driver().(interface {
		WindowOnTop(mobile.Device) bool
	})
	if !ok {
		return false, nil
	}
	return w.WindowOnTop(mobileDevice(d)), nil
}

// SetMobileWindowOnTop stores the flag and reopens the window when one is open.
func (s *Service) SetMobileWindowOnTop(ctx context.Context, id string, on bool) error {
	d, err := s.liveMobile(id)
	if err != nil {
		return err
	}
	w, ok := s.Mobile.Driver().(interface {
		SetWindowOnTop(context.Context, mobile.Device, bool) error
	})
	if !ok {
		return errMobileUnsupported
	}
	return w.SetWindowOnTop(ctx, mobileDevice(d), on)
}

func safeFileName(s string) string {
	r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	s = strings.TrimSpace(r.Replace(s))
	if s == "" {
		return "device"
	}
	return s
}

// openPath shows a saved file in the OS default app.
func openPath(path string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", path).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	}
}

// ShowMobileDevice opens a running device's window again.
func (s *Service) ShowMobileDevice(ctx context.Context, id string) error {
	if s.Mobile == nil {
		return errMobileUnsupported
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return err
	}
	sh, ok := s.Mobile.Driver().(mobile.Shower)
	if !ok || s.Mobile.Info(id).State != mobile.Running {
		return apperr.New("mobile_not_running", "open the device first")
	}
	if err := s.resumeMobile(id); err != nil {
		return err
	}
	if err := sh.Show(ctx, mobileDevice(d)); err != nil {
		return err
	}
	s.scheduleMobilePause(id)
	return nil
}

// InstallMobileApps installs APK or split bundle (.apks, .xapk, .apkm) files
// from this computer on a running device, in order, and stops at the first
// that fails.
func (s *Service) InstallMobileApps(ctx context.Context, id string, apks []string) (int, error) {
	for _, f := range apks {
		if !control.IsAppPackage(f) {
			return 0, apperr.New("mobile_apk", "only .apk, .apks, .xapk and .apkm files install")
		}
	}
	c, err := s.MobileControl(id)
	if err != nil {
		return 0, err
	}
	// An install can outlive the idle window; do not freeze it halfway.
	s.forgetMobilePause(id)
	defer s.scheduleMobilePause(id)
	for i, f := range apks {
		if err := c.Install(ctx, f); err != nil {
			if strings.Contains(err.Error(), "NO_MATCHING_ABIS") {
				return i, apperr.New("mobile_apk_abi", filepath.Base(f)+" is not made for ARM phones")
			}
			return i, apperr.New("mobile_install", filepath.Base(f)+": "+err.Error())
		}
	}
	return len(apks), nil
}

// InstallMobileAurora downloads Aurora Store from F-Droid, checks its SHA-256,
// and installs and opens it on a running device.
func (s *Service) InstallMobileAurora(ctx context.Context, id string) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	s.forgetMobilePause(id)
	defer s.scheduleMobilePause(id)
	if err := c.InstallAurora(ctx); err != nil {
		return apperr.New("mobile_install", "Aurora Store: "+err.Error())
	}
	return nil
}

// MobileControl is the agent's handle on a running device; it keeps the
// device's refs between calls.
func (s *Service) MobileControl(id string) (*control.Device, error) {
	if s.Mobile == nil {
		return nil, errMobileUnsupported
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return nil, err
	}
	ab, ok := s.Mobile.Driver().(mobile.ADBer)
	if !ok {
		return nil, errMobileUnsupported
	}
	a, ok := ab.ADB(mobileDevice(d))
	if !ok || s.Mobile.Info(id).State != mobile.Running {
		return nil, apperr.New("mobile_not_running", "open the device first")
	}
	if err := s.resumeMobile(id); err != nil {
		return nil, err
	}
	s.scheduleMobilePause(id)
	s.ctlMu.Lock()
	defer s.ctlMu.Unlock()
	if s.controls == nil {
		s.controls = map[string]*control.Device{}
	}
	if c := s.controls[id]; c != nil && c.ADB == a {
		return c, nil
	} else if c != nil {
		c.Close()
	}
	// shortcut: a device adopted after an app restart has no locale here, so new apps stay in English until its next open.
	c := &control.Device{ADB: a, Runtime: s.Dirs.MobileRuntime(), Locale: s.mobileLocale[id]}
	s.controls[id] = c
	return c, nil
}

// StopAllMobile closes every running device, in parallel, when the app quits:
// a VM's network lives in this process.
func (s *Service) StopAllMobile(ctx context.Context) {
	if s.Mobile == nil {
		return
	}
	var wg sync.WaitGroup
	for _, in := range s.Mobile.List() {
		if in.State == mobile.Stopped {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := s.StopMobileDevice(ctx, id); err != nil {
				s.Log.Warn("stop mobile device", "device", id, "err", err)
			}
		}(in.DeviceID)
	}
	wg.Wait()
}

// TrashMobileDevice closes the device and moves it to the trash.
func (s *Service) TrashMobileDevice(ctx context.Context, id string) error {
	if err := s.StopMobileDevice(ctx, id); err != nil {
		return err
	}
	if err := s.Store.TrashMobileDevice(id); err != nil {
		return err
	}
	// A saved RAM state is ~2.5 GB; the disks stay, so a restored device just cold-boots.
	if d, err := s.Store.GetMobileDevice(id); err == nil && s.Mobile != nil && d.Platform == runtime.GOOS && d.Driver == s.Mobile.Driver().Name() {
		if r, ok := s.Mobile.Driver().(interface{ DropRAM(mobile.Device) }); ok {
			r.DropRAM(mobileDevice(d))
		}
	}
	s.notify("mobile")
	return nil
}

func (s *Service) RestoreMobileDevice(id string) error {
	if err := s.Store.RestoreMobileDevice(id); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// PurgeMobileDevice deletes a trashed device for good: the driver's copy,
// its folder, its backup and its row. A device made on another OS only
// loses its row and folders here.
func (s *Service) PurgeMobileDevice(ctx context.Context, id string) error {
	d, err := s.Store.GetMobileDevice(id)
	if err != nil {
		return err
	}
	if d.DeletedAt == "" {
		return apperr.New("mobile_not_trashed", "move the device to the trash first")
	}
	if s.Mobile != nil && d.Platform == runtime.GOOS && d.Driver == s.Mobile.Driver().Name() {
		if err := s.Mobile.Stop(ctx, mobileDevice(d)); err != nil {
			return err
		}
		err := s.Mobile.Busy(id, mobile.Working, func() error {
			if d.DriverRef == "" {
				return nil // never made
			}
			return s.Mobile.Driver().Delete(ctx, mobileDevice(d))
		})
		if err != nil {
			return err
		}
	}
	for _, dir := range []string{s.Dirs.MobileDevice(id), s.Dirs.MobileBackup(id)} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := s.Store.PurgeMobileDevice(id); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// BackupMobileDevice copies a stopped device into its backup folder,
// replacing the previous backup only once the new one is complete.
func (s *Service) BackupMobileDevice(ctx context.Context, id string) error {
	d, err := s.liveMobile(id)
	if err != nil {
		return err
	}
	dir := s.Dirs.MobileBackup(id)
	tmp := dir + ".new"
	err = s.Mobile.Busy(id, mobile.Working, func() error {
		if err := os.RemoveAll(tmp); err != nil {
			return err
		}
		if err := os.MkdirAll(tmp, 0o700); err != nil {
			return err
		}
		if err := s.Mobile.Driver().Backup(ctx, mobileDevice(d), tmp); err != nil {
			os.RemoveAll(tmp)
			return err
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		return os.Rename(tmp, dir)
	})
	if errors.Is(err, mobile.ErrBusy) {
		return apperr.New("mobile_running", "close the device first")
	}
	if err != nil {
		return err
	}
	if err := s.Store.MarkMobileBackedUp(id); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// RestoreMobileBackup puts a stopped device back as it was at its last backup.
func (s *Service) RestoreMobileBackup(ctx context.Context, id string) error {
	d, err := s.liveMobile(id)
	if err != nil {
		return err
	}
	dir := s.Dirs.MobileBackup(id)
	if _, err := os.Stat(dir); err != nil {
		return apperr.New("mobile_no_backup", "this device has no backup yet")
	}
	err = s.Mobile.Busy(id, mobile.Working, func() error {
		return s.Mobile.Driver().Restore(ctx, mobileDevice(d), dir)
	})
	if errors.Is(err, mobile.ErrBusy) {
		return apperr.New("mobile_running", "close the device first")
	}
	if err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

// AdoptMobileDevices takes back the devices that outlived the app, with
// their proxies, at startup.
func (s *Service) AdoptMobileDevices(ctx context.Context) {
	if s.Mobile == nil {
		return
	}
	ds, err := s.Store.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		s.Log.Error("list mobile devices", "err", err)
		return
	}
	devs := map[string]mobile.AdoptDevice{}
	for _, d := range ds {
		if d.Platform != runtime.GOOS || d.Driver != s.Mobile.Driver().Name() {
			continue
		}
		a := mobile.AdoptDevice{Device: mobileDevice(d)}
		if d.ProxyID != "" {
			up, err := s.proxyUpstream(d.ProxyID)
			if err != nil {
				// Without its proxy the device must not be adopted onto a direct route.
				s.Log.Warn("mobile device proxy", "device", d.ID, "err", err)
				continue
			}
			a.Upstream = &up
		}
		devs[d.ID] = a
	}
	s.Mobile.Adopt(ctx, devs)
	for id := range devs {
		if s.Mobile.Info(id).State == mobile.Running {
			s.scheduleMobilePause(id)
		}
	}
}

// mobileUsesProxy is the name of a running device that goes through proxyID.
func (s *Service) mobileUsesProxy(proxyID string) (string, error) {
	if s.Mobile == nil {
		return "", nil
	}
	ds, err := s.Store.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		return "", err
	}
	for _, d := range ds {
		if d.ProxyID == proxyID && s.mobileRunning(d.ID) {
			return d.Name, nil
		}
	}
	return "", nil
}
