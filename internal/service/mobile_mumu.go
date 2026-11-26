// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
	"github.com/D0bby999/anyprofile/internal/mobile/macro"
	mobilesync "github.com/D0bby999/anyprofile/internal/mobile/sync"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/store"
)

// bulkStaggerGap spaces out simultaneous VM starts so host CPU/disk do not
// spike all at once; tests may set it to 0.
var bulkStaggerGap = 5 * time.Second

func (s *Service) syncHub() *mobilesync.Hub {
	s.ctlMu.Lock()
	defer s.ctlMu.Unlock()
	if s.mobileSync == nil {
		s.mobileSync = mobilesync.NewHub()
	}
	return s.mobileSync
}

func (s *Service) macroStore() macro.Store {
	return macro.Store{Dir: filepath.Join(s.Dirs.Mobile(), "macros")}
}

// syncTarget adapts a running device to both mobilesync.Target and macro.PlayerTarget.
type syncTarget struct {
	svc  *Service
	id   string
	w, h int
}

func (t *syncTarget) ID() string             { return t.id }
func (t *syncTarget) ScreenSize() (int, int) { return t.w, t.h }
func (t *syncTarget) Tap(ctx context.Context, x, y int) error {
	c, err := t.svc.MobileControl(t.id)
	if err != nil {
		return err
	}
	return c.Tap(ctx, "", x, y)
}
func (t *syncTarget) Swipe(ctx context.Context, x1, y1, x2, y2, ms int) error {
	c, err := t.svc.MobileControl(t.id)
	if err != nil {
		return err
	}
	return c.Drag(ctx, x1, y1, x2, y2, ms)
}
func (t *syncTarget) PressKey(ctx context.Context, key string) error {
	c, err := t.svc.MobileControl(t.id)
	if err != nil {
		return err
	}
	return c.PressKey(ctx, key)
}

func (s *Service) deviceDims(id string) (int, int) {
	if d, err := s.Store.GetMobileDevice(id); err == nil && d.ScreenW > 0 && d.ScreenH > 0 {
		return d.ScreenW, d.ScreenH
	}
	return 780, 1688
}

func (s *Service) mirrorTap(ctx context.Context, id string, x, y int) {
	w, h := s.deviceDims(id)
	rx, ry := float64(x)/float64(w), float64(y)/float64(h)
	s.syncHub().DispatchTap(ctx, id, rx, ry)
	s.mobileRec.Record(id, macro.Step{Kind: "tap", RX: rx, RY: ry})
}

func (s *Service) mirrorSwipe(ctx context.Context, id string, x1, y1, x2, y2, ms int) {
	w, h := s.deviceDims(id)
	rx1, ry1 := float64(x1)/float64(w), float64(y1)/float64(h)
	rx2, ry2 := float64(x2)/float64(w), float64(y2)/float64(h)
	s.syncHub().DispatchSwipe(ctx, id, rx1, ry1, rx2, ry2, ms)
	s.mobileRec.Record(id, macro.Step{Kind: "swipe", RX: rx1, RY: ry1, RX2: rx2, RY2: ry2, DurationMS: ms})
}

func (s *Service) mirrorKey(ctx context.Context, id, key string) {
	s.syncHub().DispatchKey(ctx, id, key)
	s.mobileRec.Record(id, macro.Step{Kind: "key", Key: key})
}

// SwipeMobileDevice performs a touch swipe in screen coordinates, mirroring to
// synchronized followers and recording into an active macro when enabled.
func (s *Service) SwipeMobileDevice(ctx context.Context, id string, x1, y1, x2, y2, ms int) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	if ms <= 0 {
		ms = 260
	}
	if err := c.Drag(ctx, x1, y1, x2, y2, ms); err != nil {
		return err
	}
	s.mirrorSwipe(ctx, id, x1, y1, x2, y2, ms)
	return nil
}

// RotateMobileDevice toggles portrait / landscape orientation on a running device.
func (s *Service) RotateMobileDevice(ctx context.Context, id string) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	out, err := c.ADB.Shell(ctx, "settings get system user_rotation")
	if err != nil {
		return err
	}
	next := "1"
	if strings.TrimSpace(out) == "1" {
		next = "0"
	}
	_, err = c.ADB.Shell(ctx, "settings put system accelerometer_rotation 0; settings put system user_rotation "+next)
	return err
}

// ArrangeMobileWindows tiles all open phone windows using layout
// ("row", "grid2x2", "grid3x2", "cascade") and returns how many were arranged.
func (s *Service) ArrangeMobileWindows(ctx context.Context, layout string) (int, error) {
	if s.Mobile == nil {
		return 0, errMobileUnsupported
	}
	arr, ok := s.Mobile.Driver().(interface {
		ArrangeWindows(context.Context, string, []mobile.Device) int
	})
	if !ok {
		return 0, errMobileUnsupported
	}
	ds, err := s.Store.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		return 0, err
	}
	devs := make([]mobile.Device, 0, len(ds))
	for _, d := range ds {
		devs = append(devs, mobileDevice(d))
	}
	return arr.ArrangeWindows(ctx, layout, devs), nil
}

// BulkMobileResult summarizes a batch operation across multiple devices.
type BulkMobileResult struct {
	OK     int      `json:"ok"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors,omitempty"`
}

// BulkStartMobileDevices starts devices with smart staggered spacing so host
// CPU and RAM do not spike simultaneously, then tiles open windows cleanly.
func (s *Service) BulkStartMobileDevices(ctx context.Context, ids []string) BulkMobileResult {
	var res BulkMobileResult
	var mu sync.Mutex
	maxConc := 2
	if s.MobileEcoMode() {
		maxConc = 1
	} else if ram, err := mobile.TotalRAM(); err == nil && (ram>>30) <= 16 {
		maxConc = 1
	}
	sem := make(chan struct{}, maxConc)
	var wg sync.WaitGroup
loop:
	for i, id := range ids {
		if i > 0 && bulkStaggerGap > 0 && maxConc > 1 {
			select {
			case <-ctx.Done():
				break loop
			case <-time.After(bulkStaggerGap):
			}
		}
		wg.Add(1)
		go func(devID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if _, err := s.startMobileBehind(ctx, devID); err != nil {
				mu.Lock()
				res.Failed++
				res.Errors = append(res.Errors, devID+": "+err.Error())
				mu.Unlock()
				return
			}
			mu.Lock()
			res.OK++
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	if res.OK >= 2 {
		_, _ = s.ArrangeMobileWindows(ctx, "row")
	}
	return res
}

// FixMobilePlayProtect clears Google Services (GSF, GMS, Play Store) cache and
// registration state on a running device without touching installed user apps
// such as TikTok or CapCut.
func (s *Service) FixMobilePlayProtect(ctx context.Context, id string) error {
	if _, err := s.liveMobile(id); err != nil {
		return err
	}
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	const cmd = "pm clear com.google.android.gsf && pm clear com.google.android.gms && pm clear com.android.vending"
	if _, err := c.ADB.Shell(ctx, "su 0 sh -c '"+cmd+"'"); err != nil {
		if _, err2 := c.ADB.Shell(ctx, cmd); err2 != nil {
			return apperr.New("mobile_play_protect", err2.Error())
		}
	}
	return nil
}

// CloneMobileDevice duplicates a stopped Golden Master device using APFS
// copy-on-write cloning (<1s, ~0 extra bytes initially) and assigns every clone
// a fresh hardware identity (Model/Serial/IMEI/MAC) plus first-boot Android ID
// regeneration.
func (s *Service) CloneMobileDevice(ctx context.Context, sourceID, baseName string, count int, resetGoogleIDs bool) ([]store.MobileDevice, error) {
	if err := s.mobileReady(); err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 1
	}
	if count > 10 {
		return nil, apperr.New("mobile_clone_limit", "clone at most 10 devices at once")
	}
	src, err := s.liveMobile(sourceID)
	if err != nil {
		return nil, err
	}
	if s.mobileRunning(sourceID) {
		return nil, apperr.New("mobile_running", "stop the source device before cloning")
	}
	cloner, ok := s.Mobile.Driver().(interface {
		CloneDevice(context.Context, mobile.Device, mobile.Device, bool) (string, string, error)
	})
	if !ok {
		return nil, errMobileUnsupported
	}
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		baseName = src.Name + " Copy"
	}
	var srcTemplate string
	if idObj, ok := vm.ParseIdentity(src.IdentityJSON); ok {
		srcTemplate = idObj.Template
	}
	created := make([]store.MobileDevice, 0, count)
	for i := 0; i < count; i++ {
		name := baseName
		if count > 1 {
			name = fmt.Sprintf("%s #%d", baseName, i+1)
		}
		devID := randomHex(8)
		identityJSON := ""
		if src.WithGMS || src.IdentityJSON != "" {
			identityJSON = vm.MarshalIdentity(s.pickMobileIdentity(devID, srcTemplate, ""))
		}
		dst := store.MobileDevice{
			ID:           devID,
			Name:         name,
			Driver:       s.Mobile.Driver().Name(),
			Platform:     runtime.GOOS,
			RAMMB:        src.RAMMB,
			CPUs:         src.CPUs,
			Preset:       src.Preset,
			ScreenW:      src.ScreenW,
			ScreenH:      src.ScreenH,
			Density:      src.Density,
			WithGMS:      src.WithGMS,
			IdentityJSON: identityJSON,
			Notes:        src.Notes,
			Tags:         src.Tags,
		}
		if err := s.Store.CreateMobileDevice(&dst); err != nil {
			return created, err
		}
		ref, img, err := cloner.CloneDevice(ctx, mobileDevice(src), mobileDevice(dst), resetGoogleIDs)
		if err != nil {
			_ = s.Store.PurgeMobileDevice(dst.ID)
			_ = os.RemoveAll(s.Dirs.MobileDevice(dst.ID))
			return created, err
		}
		if err := s.Store.SetMobileDriverState(dst.ID, ref, img); err != nil {
			return created, err
		}
		dst.DriverRef, dst.ImageVersion = ref, img
		created = append(created, dst)
	}
	s.notify("mobile")
	return created, nil
}

// CompactDiskResult summarizes physical APFS space reclaimed from a device.
type CompactDiskResult struct {
	BeforeMB int64 `json:"beforeMb"`
	AfterMB  int64 `json:"afterMb"`
	FreedMB  int64 `json:"freedMb"`
}

// CompactMobileDisk trims app caches / runs fstrim (when running) or punches
// APFS sparse holes (when stopped) and reports physical space reclaimed.
func (s *Service) CompactMobileDisk(ctx context.Context, id string) (CompactDiskResult, error) {
	if s.Mobile == nil {
		return CompactDiskResult{}, errMobileUnsupported
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return CompactDiskResult{}, err
	}
	compactor, ok := s.Mobile.Driver().(interface {
		CompactDisk(context.Context, mobile.Device) (int64, int64, error)
	})
	if !ok {
		return CompactDiskResult{}, errMobileUnsupported
	}
	before, after, err := compactor.CompactDisk(ctx, mobileDevice(d))
	if err != nil {
		return CompactDiskResult{}, err
	}
	freed := before - after
	if freed < 0 {
		freed = 0
	}
	return CompactDiskResult{
		BeforeMB: before >> 20,
		AfterMB:  after >> 20,
		FreedMB:  freed >> 20,
	}, nil
}

// BulkStopMobileDevices stops the selected devices in parallel.
func (s *Service) BulkStopMobileDevices(ctx context.Context, ids []string) BulkMobileResult {
	var res BulkMobileResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(devID string) {
			defer wg.Done()
			if err := s.StopMobileDevice(ctx, devID); err != nil {
				mu.Lock()
				res.Failed++
				res.Errors = append(res.Errors, devID+": "+err.Error())
				mu.Unlock()
				return
			}
			mu.Lock()
			res.OK++
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return res
}

// BulkInstallMobileApps installs apks onto every running device in ids concurrently.
func (s *Service) BulkInstallMobileApps(ctx context.Context, ids, apks []string) (BulkMobileResult, error) {
	for _, f := range apks {
		if !control.IsAppPackage(f) {
			return BulkMobileResult{}, apperr.New("mobile_apk", "only .apk, .apks, .xapk and .apkm files install")
		}
	}
	var res BulkMobileResult
	var mu sync.Mutex
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(devID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if _, err := s.InstallMobileApps(ctx, devID, apks); err != nil {
				mu.Lock()
				res.Failed++
				res.Errors = append(res.Errors, devID+": "+err.Error())
				mu.Unlock()
				return
			}
			mu.Lock()
			res.OK++
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return res, nil
}

// --- Phase 3: Multi-Instance Synchronizer & Operation Recorder ---

func (s *Service) StartMobileSync(masterID string, followerIDs []string, j mobilesync.Jitter) (mobilesync.Status, error) {
	if masterID == "" || len(followerIDs) == 0 {
		return mobilesync.Status{}, apperr.New("mobile_sync_args", "select a master device and at least one follower")
	}
	targets := make([]mobilesync.Target, 0, len(followerIDs))
	for _, fid := range followerIDs {
		if fid == masterID {
			continue
		}
		w, h := s.deviceDims(fid)
		targets = append(targets, &syncTarget{svc: s, id: fid, w: w, h: h})
	}
	if len(targets) == 0 {
		return mobilesync.Status{}, apperr.New("mobile_sync_args", "select at least one follower different from master")
	}
	prev := s.syncHub().Status()
	if prev.Active {
		s.clearMobileSyncRoles(prev.MasterID, prev.Followers)
	}
	s.syncHub().Start(masterID, targets, j)
	st := s.syncHub().Status()
	s.pushMobileSyncRoles(st.MasterID, st.Followers)
	s.notify("mobile")
	return st, nil
}

func (s *Service) StopMobileSync() mobilesync.Status {
	prev := s.syncHub().Status()
	s.syncHub().Stop()
	if prev.Active {
		s.clearMobileSyncRoles(prev.MasterID, prev.Followers)
	}
	s.notify("mobile")
	return s.syncHub().Status()
}

func (s *Service) MobileSyncStatus() mobilesync.Status {
	return s.syncHub().Status()
}

func (s *Service) StartMacroRecord(deviceID, name string) error {
	if _, err := s.MobileControl(deviceID); err != nil {
		return err
	}
	s.mobileRec.Start(deviceID, name)
	s.notify("mobile")
	return nil
}

func (s *Service) StopMacroRecord() (macro.Script, error) {
	sc, ok := s.mobileRec.Stop(randomHex(6))
	if !ok {
		return macro.Script{}, apperr.New("mobile_macro_idle", "no macro recording is active")
	}
	if err := s.macroStore().Save(sc); err != nil {
		return macro.Script{}, err
	}
	s.notify("mobile")
	return sc, nil
}

func (s *Service) MobileMacroRecording() string {
	return s.mobileRec.ActiveDevice()
}

func (s *Service) ListMobileMacros() ([]macro.Script, error) {
	return s.macroStore().List()
}

func (s *Service) DeleteMobileMacro(id string) error {
	if err := s.macroStore().Delete(id); err != nil {
		return err
	}
	s.notify("mobile")
	return nil
}

func (s *Service) RunMobileMacro(ctx context.Context, macroID string, deviceIDs []string, opts macro.PlayOptions) error {
	sc, err := s.macroStore().Load(macroID)
	if err != nil {
		return err
	}
	if len(deviceIDs) == 0 {
		return apperr.New("mobile_macro_devices", "select at least one running device")
	}
	s.macroMu.Lock()
	if s.macroCancel != nil {
		s.macroCancel()
	}
	pctx, cancel := context.WithCancel(ctx)
	s.macroCancel = cancel
	s.macroMu.Unlock()

	var wg sync.WaitGroup
	for _, id := range deviceIDs {
		w, h := s.deviceDims(id)
		tgt := &syncTarget{svc: s, id: id, w: w, h: h}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = macro.Play(pctx, sc, tgt, opts)
		}()
	}
	go func() {
		wg.Wait()
		cancel()
	}()
	return nil
}

func (s *Service) StopMobileMacro() {
	s.macroMu.Lock()
	if s.macroCancel != nil {
		s.macroCancel()
		s.macroCancel = nil
	}
	s.macroMu.Unlock()
}

// --- Phase 4: Virtual GPS & 2-Way Shared Folder ---

// SetMobileLocation injects a WGS-84 GPS coordinate into a running device.
func (s *Service) SetMobileLocation(ctx context.Context, id string, lat, lon float64) error {
	if !vm.ValidCoords(lat, lon) {
		return apperr.New("mobile_bad_gps", "latitude must be -90..90 and longitude -180..180")
	}
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	if s.Mobile != nil {
		if loc, ok := s.Mobile.Driver().(interface {
			SetLocation(context.Context, string, float64, float64)
		}); ok {
			loc.SetLocation(ctx, id, lat, lon)
		}
	}
	return vm.ApplyLocation(ctx, c.ADB, lat, lon)
}

// ResetMobileLocationToProxy re-applies the GPS coordinates of the device's proxy.
func (s *Service) ResetMobileLocationToProxy(ctx context.Context, id string) (string, error) {
	d, err := s.liveMobile(id)
	if err != nil {
		return "", err
	}
	net, err := s.proxyNetwork(ctx, d.ProxyID)
	if err != nil {
		return "", err
	}
	if net.Lat == nil || net.Lon == nil {
		return "", apperr.New("mobile_no_proxy_gps", "proxy has no geolocation coordinates")
	}
	jLat, jLon := vm.JitterCoords(d.ID, *net.Lat, *net.Lon)
	if err := s.SetMobileLocation(ctx, id, jLat, jLon); err != nil {
		return "", err
	}
	return fmt.Sprintf("%.4f, %.4f", jLat, jLon), nil
}

func (s *Service) mobileSharedDir(d store.MobileDevice) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Documents", "AnyProfile Shared", safeFileName(d.Name))
	return dir, os.MkdirAll(dir, 0o755)
}

// OpenMobileSharedFolder creates (if needed) and opens the device's Mac<->Android
// shared folder in Finder / Explorer.
func (s *Service) OpenMobileSharedFolder(id string) (string, error) {
	d, err := s.liveMobile(id)
	if err != nil {
		return "", err
	}
	dir, err := s.mobileSharedDir(d)
	if err != nil {
		return "", err
	}
	openPath(dir)
	return dir, nil
}

// SyncMobileSharedFolder syncs files between the Mac shared folder and
// /sdcard/Download/AnyProfile on the running device. direction is "push" or "pull".
func (s *Service) SyncMobileSharedFolder(ctx context.Context, id, direction string) (int, error) {
	d, err := s.liveMobile(id)
	if err != nil {
		return 0, err
	}
	dir, err := s.mobileSharedDir(d)
	if err != nil {
		return 0, err
	}
	c, err := s.MobileControl(id)
	if err != nil {
		return 0, err
	}
	const remoteDir = "/sdcard/Download/AnyProfile"
	_, _ = c.ADB.Shell(ctx, "mkdir -p "+remoteDir)
	switch direction {
	case "pull":
		if _, err := c.ADB.Run(ctx, "pull", remoteDir+"/.", dir); err != nil {
			return 0, apperr.New("mobile_shared_pull", err.Error())
		}
		entries, _ := os.ReadDir(dir)
		return len(entries), nil
	default: // "push"
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 0, err
		}
		n := 0
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			local := filepath.Join(dir, e.Name())
			remote := remoteDir + "/" + safeFileName(e.Name())
			if _, err := c.ADB.Run(ctx, "push", local, remote); err != nil {
				return n, apperr.New("mobile_shared_push", e.Name()+": "+err.Error())
			}
			_, _ = c.ADB.Shell(ctx, "am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE -d 'file://"+remote+"'")
			n++
		}
		return n, nil
	}
}
