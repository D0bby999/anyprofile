// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
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
	State mobile.State `json:"state"`
	Error string       `json:"error"`
}

// MobileInput is what the user sets on a device.
type MobileInput struct {
	Name    string   `json:"name"`
	ProxyID string   `json:"proxyId"` // AutoProxy picks the oldest free sticky proxy
	RAMMB   int      `json:"ramMb"`   // 0: the default
	CPUs    int      `json:"cpus"`    // 0: the default
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

// mobileAllowed is the Pro gate. It reads the state saved by the last Pro
// check and never calls the server.
func (s *Service) mobileAllowed() error {
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

// InstallMobileRuntime downloads the VM runtime and the Android image (macOS).
// It is part of Pro, as the devices are.
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
			row.State, row.Error = in.State, in.Error
		}
		out = append(out, row)
	}
	return out, nil
}

// checkMobileInput fills the defaults and refuses what a device cannot have.
func checkMobileInput(in *MobileInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.New("mobile_name_required", "device name is required")
	}
	if in.RAMMB == 0 {
		in.RAMMB = defaultMobileRAMMB
	}
	if in.CPUs == 0 {
		in.CPUs = defaultMobileCPUs
	}
	if in.RAMMB < minMobileRAMMB || in.RAMMB > maxMobileRAMMB {
		return apperr.Newf("mobile_bad_ram", "memory must be between %d and %d MB", minMobileRAMMB, maxMobileRAMMB)
	}
	if in.CPUs < 1 || in.CPUs > maxMobileCPUs {
		return apperr.Newf("mobile_bad_cpus", "between 1 and %d CPU cores", maxMobileCPUs)
	}
	return nil
}

func mobileDevice(d store.MobileDevice) mobile.Device {
	return mobile.Device{ID: d.ID, Name: d.Name, Ref: d.DriverRef, RAMMB: d.RAMMB, CPUs: d.CPUs}
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
		Platform: runtime.GOOS, ProxyID: in.ProxyID, RAMMB: in.RAMMB, CPUs: in.CPUs, Notes: in.Notes,
		Tags: in.Tags, Pinned: in.Pinned}
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
	d.Name, d.ProxyID, d.RAMMB, d.CPUs, d.Notes, d.Tags, d.Pinned =
		in.Name, in.ProxyID, in.RAMMB, in.CPUs, in.Notes, in.Tags, in.Pinned
	if err := s.Store.UpdateMobileDevice(d); err != nil {
		return err
	}
	s.notify("mobile")
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
	if err := s.mobileReady(); err != nil {
		return MobileStart{}, err
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return MobileStart{}, err
	}
	net, err := s.proxyNetwork(ctx, d.ProxyID)
	if err != nil {
		return MobileStart{}, err
	}
	o := mobile.StartOptions{Timezone: net.Timezone, Locale: androidLocale(net)}
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
	if err := s.Store.MarkMobileOpened(id); err != nil {
		s.Log.Warn("mark mobile device opened", "device", id, "err", err)
	}
	s.notify("mobile")
	return res, nil
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
	d, err := s.Store.GetMobileDevice(id)
	if err != nil {
		return err
	}
	if err := s.Mobile.Stop(ctx, mobileDevice(d)); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
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
	return sh.Show(ctx, mobileDevice(d))
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
