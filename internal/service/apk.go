// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/apk"
	"github.com/D0bby999/anyprofile/internal/mobile/control"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

// APKInspect is the offline preview returned right after choosing an APK/split bundle.
type APKInspect struct {
	FilePath       string   `json:"filePath"`
	SizeBytes      int64    `json:"sizeBytes"`
	Package        string   `json:"package"`
	Label          string   `json:"label"`
	VersionName    string   `json:"versionName"`
	VersionCode    int32    `json:"versionCode"`
	MinSDK         int32    `json:"minSdk"`
	TargetSDK      int32    `json:"targetSdk"`
	LaunchActivity string   `json:"launchActivity"`
	Orientation    string   `json:"orientation"`
	ABIs           []string `json:"abis"`
	IsSplitBundle  bool     `json:"isSplitBundle"`
	IconData       string   `json:"iconData,omitempty"`
	ABICompatible  bool     `json:"abiCompatible"`
	ABIWarning     string   `json:"abiWarning,omitempty"`
}

// APKInput configures adding and installing an APK into AnyProfile's Pro APK tab.
type APKInput struct {
	FilePath         string `json:"filePath"`
	Label            string `json:"label"`
	DeviceID         string `json:"deviceId"`
	ProxyID          string `json:"proxyId"`
	Notes            string `json:"notes"`
	Orientation      string `json:"orientation"`
	IdentityTemplate string `json:"identityTemplate"`
	CreateShortcut   bool   `json:"createShortcut"`
}

// APKUpdate updates editable metadata, network and hardware fingerprint settings for a cataloged APK.
type APKUpdate struct {
	Package          string `json:"package"`
	Label            string `json:"label"`
	Orientation      string `json:"orientation"`
	ProxyID          string `json:"proxyId"`
	Notes            string `json:"notes"`
	IdentityTemplate string `json:"identityTemplate"`
}

const (
	defaultAPKDeviceName       = "APK Runner"
	apkHostNotes               = "__anyprofile_apk_runner__"
	settingAPKPendingUninstall = "apk_pending_uninstall"
)

func (s *Service) apkHostABIs() []string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return []string{"arm64-v8a"}
	}
	return []string{"x86_64", "arm64-v8a", "armeabi-v7a"}
}

func (s *Service) apkCacheDir() string {
	if s.Dirs.Root != "" {
		return filepath.Join(s.Dirs.Root, "mobile", "apks")
	}
	return filepath.Join(os.TempDir(), "anyprofile-apks")
}

// ListAPKs returns all saved standalone APK entries. Readable even if Pro has
// lapsed so users can see and remove previously added apps. Lazily backfills a
// unique hardware identity & android_id for legacy entries so every APK has its
// own anti-detect fingerprint.
func (s *Service) ListAPKs() ([]store.MobileAPK, error) {
	list, err := s.Store.ListMobileAPKs()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].IdentityJSON == "" || list[i].AndroidID == "" {
			idn, aid := s.ensureAPKIdentityFields(list[i].Package, list[i].IdentityJSON, list[i].AndroidID, "")
			list[i].IdentityJSON, list[i].AndroidID = idn, aid
			_ = s.Store.SetMobileAPKIdentity(list[i].Package, idn, aid)
		}
	}
	return list, nil
}

func (s *Service) pickAPKIdentity(seed, template, excludePkg string) vm.Identity {
	tpl := strings.TrimSpace(template)
	if tpl == "auto" {
		tpl = ""
	}
	avoProfiles := s.loadAVOProfiles()
	if tpl != "" {
		if p, ok := avoProfiles[tpl]; ok {
			if id, ok := avoProfileToIdentity(seed, p); ok {
				return id
			}
		}
		return vm.IdentityFor(seed, tpl)
	}
	used := map[string]int{}
	if s.Store != nil {
		if list, err := s.Store.ListMobileAPKs(); err == nil {
			for _, a := range list {
				if a.Package == excludePkg {
					continue
				}
				if id, ok := vm.ParseIdentity(a.IdentityJSON); ok && id.Template != "" {
					used[id.Template]++
					if c := canonicalMobileTemplate(id.Template); c != id.Template {
						used[c]++
					}
				}
			}
		}
	}
	if len(avoProfiles) > 0 {
		cands := make([]string, 0, len(avoProfiles))
		for id := range avoProfiles {
			cands = append(cands, id)
		}
		sort.Strings(cands)
		picked := vm.PickDistinctTemplate(seed, cands, used)
		if id, ok := avoProfileToIdentity(seed, avoProfiles[picked]); ok {
			return id
		}
	}
	return vm.IdentityFor(seed, vm.PickDistinctTemplate(seed, vm.IdentityTemplates(), used))
}

func (s *Service) ensureAPKIdentityFields(pkg, curIdentity, curAndroidID, template string) (string, string) {
	idn := curIdentity
	aid := curAndroidID
	tpl := strings.TrimSpace(template)
	if tpl != "" {
		seed := pkg + ":" + randomHex(4)
		isRegen := tpl == "regen" || strings.HasPrefix(tpl, "regen:")
		if isRegen {
			tpl = strings.TrimPrefix(tpl, "regen:")
			if tpl == "regen" {
				if cur, ok := vm.ParseIdentity(curIdentity); ok {
					tpl = cur.Template
				} else {
					tpl = ""
				}
			}
			aid = randomHex(8)
		} else if tpl == "auto" {
			tpl = ""
		}
		idn = vm.MarshalIdentity(s.pickAPKIdentity(seed, tpl, pkg))
	} else if idn == "" {
		idn = vm.MarshalIdentity(s.pickAPKIdentity(pkg+":"+randomHex(4), "", pkg))
	}
	if aid == "" {
		aid = randomHex(8)
	}
	return idn, aid
}

// InspectAPK parses an .apk/.xapk/.apks/.apkm completely offline (Pro-gated).
func (s *Service) InspectAPK(_ context.Context, pkgPath string) (APKInspect, error) {
	if err := s.mobileAllowed(); err != nil {
		return APKInspect{}, err
	}
	meta, err := apk.Parse(pkgPath)
	if err != nil {
		return APKInspect{}, apperr.New("mobile_apk", err.Error())
	}
	res := APKInspect{
		FilePath:       meta.FilePath,
		SizeBytes:      meta.SizeBytes,
		Package:        meta.Package,
		Label:          meta.Label,
		VersionName:    meta.VersionName,
		VersionCode:    meta.VersionCode,
		MinSDK:         meta.MinSDK,
		TargetSDK:      meta.TargetSDK,
		LaunchActivity: meta.LaunchActivity,
		Orientation:    string(meta.Orientation),
		ABIs:           meta.ABIs,
		IsSplitBundle:  meta.IsSplitBundle,
		ABICompatible:  true,
	}
	if res.ABIs == nil {
		res.ABIs = []string{}
	}
	if len(meta.IconBytes) > 0 {
		mime := "image/png"
		switch meta.IconExt {
		case ".webp":
			mime = "image/webp"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		}
		res.IconData = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(meta.IconBytes)
	}
	if err := apk.CheckABICompatible(meta.ABIs, s.apkHostABIs()); err != nil {
		res.ABICompatible = false
		res.ABIWarning = err.Error()
	}
	return res, nil
}

// AddAPK inspects, caches, installs onto the chosen (or shared) Android device,
// catalogs the APK, and optionally creates a desktop shortcut (Pro-gated).
func (s *Service) AddAPK(ctx context.Context, in APKInput) (store.MobileAPK, error) {
	return s.addAPK(ctx, in, true)
}

func (s *Service) addAPK(ctx context.Context, in APKInput, cacheFile bool) (store.MobileAPK, error) {
	if err := s.mobileAllowed(); err != nil {
		return store.MobileAPK{}, err
	}
	info, err := s.InspectAPK(ctx, in.FilePath)
	if err != nil {
		return store.MobileAPK{}, err
	}
	if !info.ABICompatible {
		return store.MobileAPK{}, apperr.New("mobile_apk_abi", info.ABIWarning)
	}

	devID, err := s.ensureAPKDevice(ctx, "")
	if err != nil {
		return store.MobileAPK{}, err
	}
	idn, aid := s.ensureAPKIdentityFields(info.Package, "", "", in.IdentityTemplate)
	// Align the APK host's hardware identity BEFORE booting it so an immediate
	// LaunchAPK right after AddAPK doesn't need to reboot the VM mid-dexopt.
	if !s.mobileRunning(devID) {
		if hostDev, herr := s.liveMobile(devID); herr == nil {
			hostDev.IdentityJSON = idn
			hostDev.ProxyID = strings.TrimSpace(in.ProxyID)
			_ = s.Store.UpdateMobileDevice(hostDev)
		}
		if _, err := s.StartMobileDeviceHeadless(ctx, devID); err != nil {
			return store.MobileAPK{}, err
		}
	}
	s.resumeAPKHost(devID)
	s.removePendingAPKUninstall(info.Package)
	if c, cerr := s.MobileControl(devID); cerr == nil {
		s.flushPendingAPKUninstalls(ctx, c)
	}
	if _, err := s.InstallMobileApps(ctx, devID, []string{in.FilePath}); err != nil {
		return store.MobileAPK{}, err
	}

	// Local file installs keep a cached copy so re-installing on another device
	// survives moving the original file; Store downloads can be re-fetched on
	// demand and skip caching to avoid piling up hundreds of MB in mobile/apks/.
	cachedPath := ""
	if cacheFile {
		cachedPath = in.FilePath
		if dir := s.apkCacheDir(); os.MkdirAll(dir, 0o700) == nil {
			dst := filepath.Join(dir, info.Package+strings.ToLower(filepath.Ext(in.FilePath)))
			if copyAPKFile(in.FilePath, dst) == nil {
				cachedPath = dst
				if dlDir := filepath.Join(dir, apkDownloadsSubd) + string(os.PathSeparator); strings.HasPrefix(in.FilePath, dlDir) {
					_ = os.Remove(in.FilePath)
				}
			}
		}
	} else if dir := s.apkCacheDir(); dir != "" {
		// Remove any stale cached installer from a previous install of this package.
		for _, ext := range []string{".apk", ".apks", ".xapk", ".apkm"} {
			_ = os.Remove(filepath.Join(dir, info.Package+ext))
		}
	}

	if in.ProxyID != "" {
		if _, err := s.Store.GetProxy(in.ProxyID); err != nil {
			return store.MobileAPK{}, apperr.New("proxy_not_found", "selected proxy does not exist")
		}
	}

	ori := info.Orientation
	if in.Orientation == "auto" || in.Orientation == "portrait" || in.Orientation == "landscape" {
		ori = in.Orientation
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = info.Label
	}
	rec := store.MobileAPK{
		Package:        info.Package,
		Label:          label,
		VersionName:    info.VersionName,
		VersionCode:    info.VersionCode,
		LaunchActivity: info.LaunchActivity,
		Orientation:    ori,
		ABIs:           info.ABIs,
		FilePath:       cachedPath,
		IconData:       info.IconData,
		DeviceID:       devID,
		ProxyID:        strings.TrimSpace(in.ProxyID),
		Notes:          strings.TrimSpace(in.Notes),
		IdentityJSON:   idn,
		AndroidID:      aid,
	}
	if in.CreateShortcut {
		if sc, err := s.makeAPKShortcut(rec); err == nil {
			rec.ShortcutPath = sc
		}
	}
	if err := s.Store.UpsertMobileAPK(&rec); err != nil {
		return store.MobileAPK{}, err
	}
	s.notify("apk")
	return s.Store.GetMobileAPK(rec.Package)
}

// LaunchAPK opens a cataloged Android app directly on its assigned (or override)
// Mobile device, booting or waking the VM first if needed (Pro-gated).
func (s *Service) LaunchAPK(ctx context.Context, pkg, deviceID string) error {
	return s.launchAPK(ctx, pkg, deviceID, true)
}

// LaunchAPKHeadless launches a cataloged Android app on the dedicated APK host
// without opening a viewport window, for AI agents and automation (Pro-gated).
func (s *Service) LaunchAPKHeadless(ctx context.Context, pkg string) error {
	return s.launchAPK(ctx, pkg, "", false)
}

func (s *Service) launchAPK(ctx context.Context, pkg, deviceID string, showWindow bool) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	if !apk.ValidPackageName(pkg) {
		return apperr.New("mobile_package", "invalid package name")
	}
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return err
	}
	targetID, err := s.ensureAPKDevice(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}
	if rec.DeviceID != targetID {
		_ = s.Store.SetMobileAPKDevice(rec.Package, targetID)
		rec.DeviceID = targetID
	}
	if rec.IdentityJSON == "" || rec.AndroidID == "" {
		rec.IdentityJSON, rec.AndroidID = s.ensureAPKIdentityFields(rec.Package, rec.IdentityJSON, rec.AndroidID, "")
		_ = s.Store.SetMobileAPKIdentity(rec.Package, rec.IdentityJSON, rec.AndroidID)
	}
	// Ensure the dedicated APK host boots with THIS APK's exact hardware identity
	// (ro.product.model, IMEI, serial, build fingerprint, GPU vendor/renderer, MIDR).
	// Same identity = instant (~1s) launch; different identity = clean reboot with new props.
	if hostDev, err := s.liveMobile(targetID); err == nil {
		idChanged := hostDev.IdentityJSON != rec.IdentityJSON
		if idChanged || hostDev.ProxyID != rec.ProxyID {
			hostDev.IdentityJSON = rec.IdentityJSON
			hostDev.ProxyID = rec.ProxyID
			_ = s.Store.UpdateMobileDevice(hostDev)
		}
		if idChanged && s.mobileRunning(targetID) {
			_ = s.StopMobileDevice(ctx, targetID)
		}
	}
	if !s.mobileRunning(targetID) {
		if _, err := s.StartMobileDeviceHeadless(ctx, targetID); err != nil {
			return err
		}
	}

	// Hot-swap the userspace vnet upstream proxy on the running APK host so each
	// standalone APK routes through its own configured proxy (or direct) in 0ms.
	if err := s.applyAPKProxy(ctx, targetID, rec.ProxyID); err != nil {
		return err
	}

	// If the APK host was previously paused by anyprofile-view on window close,
	// thaw its vCPUs before any ADB shell command (otherwise ADB hangs on frozen guest).
	if s.Mobile != nil {
		if resumer, ok := s.Mobile.Driver().(interface{ Resume(id string) error }); ok {
			_ = resumer.Resume(targetID)
		}
	}

	c, err := s.MobileControl(targetID)
	if err != nil {
		return err
	}
	// Wake Android from Doze/sleep (headless boot sets screen_off_timeout=15000 & stayon=false,
	// which otherwise leaves SurfaceFlinger producing black frames), apply per-APK runtime
	// identity (SSAID android_id, device_name, battery), and tombstone other cataloged APKs.
	_, _ = c.ADB.Shell(ctx, "dumpsys deviceidle unforce 2>/dev/null || true; settings put system screen_off_timeout 2147483647; svc power stayon true; input keyevent 224; wm dismiss-keyguard")
	s.flushPendingAPKUninstalls(ctx, c)
	s.apkApplyRuntimeIdentity(ctx, c, rec)
	s.apkTrimBloat(ctx, c)
	s.apkTombstoneOthers(ctx, c, rec.Package)

	// If the APK is not yet installed on the dedicated APK host, install from cache.
	if out, err := c.ADB.Shell(ctx, "pm path "+rec.Package); err != nil || !strings.Contains(out, "package:") {
		if rec.FilePath != "" {
			if _, stErr := os.Stat(rec.FilePath); stErr == nil {
				if _, ierr := s.InstallMobileApps(ctx, targetID, []string{rec.FilePath}); ierr != nil {
					return ierr
				}
			}
		}
	}

	// Apply orientation preference before opening the clean app-mode window.
	switch rec.Orientation {
	case "portrait":
		_, _ = c.ADB.Shell(ctx, "settings put system accelerometer_rotation 0; settings put system user_rotation 0")
	case "landscape":
		_, _ = c.ADB.Shell(ctx, "settings put system accelerometer_rotation 0; settings put system user_rotation 1")
	}

	if rec.LaunchActivity != "" && apk.ValidComponent(rec.LaunchActivity) {
		out, err := c.ADB.Shell(ctx, fmt.Sprintf("am start -W -n %s/%s", rec.Package, rec.LaunchActivity))
		if err != nil || strings.Contains(out, "Error:") {
			if err := c.Launch(ctx, rec.Package); err != nil {
				return err
			}
		}
	} else if err := c.Launch(ctx, rec.Package); err != nil {
		return err
	}
	if showWindow {
		_ = s.showMobileDeviceWithTitle(ctx, targetID, rec.Label, true)
	}
	_ = s.Store.TouchMobileAPKOpened(rec.Package)
	s.notify("apk")
	return nil
}

// ShowAPK opens the clean standalone app-mode viewport window for a cataloged APK
// when its host is already running (Pro-gated).
func (s *Service) ShowAPK(ctx context.Context, pkg string) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return err
	}
	hostID, ok := s.apkHostID(rec.DeviceID)
	if !ok || !s.mobileRunning(hostID) {
		return apperr.New("apk_not_running", "open the APK first")
	}
	return s.showMobileDeviceWithTitle(ctx, hostID, rec.Label, true)
}

// CloseAPK force-stops pkg on the APK host and either pauses the host (~0% idle CPU)
// or powers it off completely when stopHost is true.
func (s *Service) CloseAPK(ctx context.Context, pkg string, stopHost bool) error {
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return err
	}
	hostID, ok := s.apkHostID(rec.DeviceID)
	if !ok || !s.mobileRunning(hostID) {
		return nil
	}
	if apk.ValidPackageName(rec.Package) {
		if c, err := s.MobileControl(hostID); err == nil {
			_, _ = c.ADB.Shell(ctx, "am force-stop "+rec.Package)
		}
	}
	if stopHost {
		return s.StopMobileDevice(ctx, hostID)
	}
	if s.Mobile != nil {
		if h, ok := s.Mobile.Driver().(interface{ Hide(id string) }); ok {
			h.Hide(hostID)
		}
		if p, ok := s.Mobile.Driver().(mobile.Pauser); ok {
			_ = p.Pause(hostID)
		}
	}
	return nil
}

// APKControl returns the ADB UI controller for the running APK host.
func (s *Service) APKControl(pkg string) (*control.Device, error) {
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return nil, err
	}
	hostID, ok := s.apkHostID(rec.DeviceID)
	if !ok || !s.mobileRunning(hostID) {
		return nil, apperr.New("apk_not_running", "open the APK first")
	}
	if s.Mobile != nil {
		if resumer, ok := s.Mobile.Driver().(interface{ Resume(id string) error }); ok {
			_ = resumer.Resume(hostID)
		}
	}
	return s.MobileControl(hostID)
}

// APKHostRunning reports whether the dedicated standalone APK host VM is running.
func (s *Service) APKHostRunning() bool {
	id, ok := s.apkHostID("")
	return ok && s.mobileRunning(id)
}

func (s *Service) apkHostID(preferredID string) (string, bool) {
	if preferredID != "" {
		if _, err := s.liveMobile(preferredID); err == nil {
			return preferredID, true
		}
	}
	devs, err := s.Store.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		return "", false
	}
	for _, d := range devs {
		if d.DeletedAt == "" && d.Notes == apkHostNotes {
			return d.ID, true
		}
	}
	return "", false
}

// UpdateAPK saves custom label, orientation, proxy and notes for a cataloged APK (Pro-gated).
func (s *Service) UpdateAPK(ctx context.Context, in APKUpdate) (store.MobileAPK, error) {
	if err := s.mobileAllowed(); err != nil {
		return store.MobileAPK{}, err
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return store.MobileAPK{}, apperr.New("mobile_apk", "app name is required")
	}
	ori := in.Orientation
	if ori == "" {
		ori = "auto"
	}
	switch ori {
	case "auto", "portrait", "landscape":
	default:
		return store.MobileAPK{}, apperr.New("mobile_apk", "orientation must be auto, portrait, or landscape")
	}
	proxyID := strings.TrimSpace(in.ProxyID)
	if proxyID != "" {
		if _, err := s.Store.GetProxy(proxyID); err != nil {
			return store.MobileAPK{}, apperr.New("proxy_not_found", "selected proxy does not exist")
		}
	}
	cur, err := s.Store.GetMobileAPK(in.Package)
	if err != nil {
		return store.MobileAPK{}, err
	}
	idn, aid := s.ensureAPKIdentityFields(cur.Package, cur.IdentityJSON, cur.AndroidID, in.IdentityTemplate)
	if err := s.Store.UpdateMobileAPK(in.Package, label, ori, proxyID, strings.TrimSpace(in.Notes), idn, aid); err != nil {
		return store.MobileAPK{}, err
	}
	rec, err := s.Store.GetMobileAPK(in.Package)
	if err != nil {
		return store.MobileAPK{}, err
	}
	// If the APK host is currently running with this APK's identity and proxy changed,
	// hot-swap vnet immediately (0ms) so the running app picks up the new route.
	if rec.ProxyID != cur.ProxyID {
		if hostID, ok := s.apkHostID(rec.DeviceID); ok && s.mobileRunning(hostID) {
			if hostDev, herr := s.liveMobile(hostID); herr == nil && hostDev.IdentityJSON == rec.IdentityJSON {
				_ = s.applyAPKProxy(ctx, hostID, rec.ProxyID)
			}
		}
	}
	// If the shortcut already existed, refresh it so its name stays in sync.
	if rec.ShortcutPath != "" {
		_ = os.RemoveAll(rec.ShortcutPath)
		if sc, serr := s.makeAPKShortcut(rec); serr == nil {
			_ = s.Store.SetMobileAPKShortcut(rec.Package, sc)
			rec.ShortcutPath = sc
		}
	}
	s.notify("apk")
	return rec, nil
}

func (s *Service) applyAPKProxy(ctx context.Context, devID, proxyID string) error {
	if s.Mobile == nil {
		return nil
	}
	swapper, ok := s.Mobile.Driver().(interface {
		SetUpstream(id string, up *netproxy.Upstream) error
	})
	if !ok {
		return nil
	}
	var up *netproxy.Upstream
	if proxyID != "" {
		u, err := s.proxyUpstream(proxyID)
		if err != nil {
			return err
		}
		up = &u
	}
	if err := swapper.SetUpstream(devID, up); err != nil {
		return err
	}
	// Best-effort timezone and carrier sync when the proxy has an exit region.
	if proxyID != "" {
		if net, err := s.proxyNetwork(ctx, proxyID); err == nil {
			if c, cerr := s.MobileControl(devID); cerr == nil {
				var script strings.Builder
				if net.Timezone != "" {
					fmt.Fprintf(&script, "setprop persist.sys.timezone %s\n", net.Timezone)
				}
				if carrier := vm.CarrierShellScript(net.Country); carrier != "" {
					script.WriteString(carrier)
					script.WriteByte('\n')
				}
				if script.Len() > 0 {
					_, _ = c.ADB.Shell(ctx, script.String())
				}
			}
		}
	}
	return nil
}

// apkApplyRuntimeIdentity sets the per-APK SSAID (android_id), friendly device
// name, and deterministic battery level inside Android.
func (s *Service) apkApplyRuntimeIdentity(ctx context.Context, c *control.Device, rec store.MobileAPK) {
	var script strings.Builder
	if rec.AndroidID != "" {
		fmt.Fprintf(&script, "settings put secure android_id %s >/dev/null 2>&1 || true\n", rec.AndroidID)
	}
	if idn, ok := vm.ParseIdentity(rec.IdentityJSON); ok && idn.Model != "" {
		name := strings.ReplaceAll(idn.Model, "'", "")
		fmt.Fprintf(&script, "settings put global device_name '%s' >/dev/null 2>&1 || true\n", name)
		fmt.Fprintf(&script, "settings put secure bluetooth_name '%s' >/dev/null 2>&1 || true\n", name)
	}
	script.WriteString(vm.BatteryShellScript(rec.Package + ":" + rec.AndroidID))
	_, _ = c.ADB.Shell(ctx, script.String())
}

// SetAPKOrientation saves the preferred window orientation for pkg (Pro-gated).
func (s *Service) SetAPKOrientation(pkg, orientation string) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	switch orientation {
	case "auto", "portrait", "landscape":
	default:
		return apperr.New("mobile_apk", "orientation must be auto, portrait, or landscape")
	}
	if err := s.Store.SetMobileAPKOrientation(pkg, orientation); err != nil {
		return err
	}
	s.notify("apk")
	return nil
}

// SetAPKDevice assigns pkg to a specific Mobile device (or empty for auto) (Pro-gated).
func (s *Service) SetAPKDevice(pkg, deviceID string) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	if deviceID != "" {
		if _, err := s.liveMobile(deviceID); err != nil {
			return err
		}
	}
	if err := s.Store.SetMobileAPKDevice(pkg, deviceID); err != nil {
		return err
	}
	s.notify("apk")
	return nil
}

// CreateAPKShortcut creates an OS launcher shortcut (.app / .lnk / .desktop) (Pro-gated).
func (s *Service) CreateAPKShortcut(pkg string) (string, error) {
	if err := s.mobileAllowed(); err != nil {
		return "", err
	}
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return "", err
	}
	sc, err := s.makeAPKShortcut(rec)
	if err != nil {
		return "", err
	}
	_ = s.Store.SetMobileAPKShortcut(pkg, sc)
	s.notify("apk")
	return sc, nil
}

// DeleteAPK removes an APK entry from AnyProfile, deletes its shortcut & cached
// file, and optionally uninstalls it from its target device (immediately when
// running/paused, or queued for next boot so deleted apps never resurrect).
func (s *Service) DeleteAPK(ctx context.Context, pkg string, uninstall bool) error {
	rec, err := s.Store.GetMobileAPK(pkg)
	if err != nil {
		return err
	}
	if uninstall && apk.ValidPackageName(rec.Package) {
		s.addPendingAPKUninstall(rec.Package)
		hostID, ok := s.apkHostID(rec.DeviceID)
		if ok && s.mobileRunning(hostID) {
			wasPaused := false
			if s.Mobile != nil {
				if p, ok := s.Mobile.Driver().(mobile.Pauser); ok && p.Paused(hostID) {
					wasPaused = true
					s.resumeAPKHost(hostID)
				}
			}
			if c, cerr := s.MobileControl(hostID); cerr == nil {
				if uninstallAPKOnDevice(ctx, c, rec.Package) {
					s.removePendingAPKUninstall(rec.Package)
				}
			}
			if wasPaused && s.Mobile != nil {
				if p, ok := s.Mobile.Driver().(mobile.Pauser); ok {
					_ = p.Pause(hostID)
				}
			}
		}
	}
	if rec.ShortcutPath != "" {
		_ = os.RemoveAll(rec.ShortcutPath)
	}
	if rec.FilePath != "" && strings.HasPrefix(rec.FilePath, s.apkCacheDir()) {
		_ = os.Remove(rec.FilePath)
	}
	if err := s.Store.DeleteMobileAPK(pkg); err != nil {
		return err
	}
	s.notify("apk")
	return nil
}

func (s *Service) resumeAPKHost(hostID string) {
	if s.Mobile == nil || hostID == "" {
		return
	}
	if resumer, ok := s.Mobile.Driver().(interface{ Resume(id string) error }); ok {
		_ = resumer.Resume(hostID)
	}
}

func (s *Service) pendingAPKUninstalls() []string {
	if s.Store == nil {
		return nil
	}
	raw, err := s.Store.GetSetting(settingAPKPendingUninstall)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var pkgs []string
	if err := json.Unmarshal([]byte(raw), &pkgs); err != nil {
		return nil
	}
	return pkgs
}

func (s *Service) setPendingAPKUninstalls(pkgs []string) {
	if s.Store == nil {
		return
	}
	if len(pkgs) == 0 {
		_ = s.Store.SetSetting(settingAPKPendingUninstall, "")
		return
	}
	b, err := json.Marshal(pkgs)
	if err != nil {
		return
	}
	_ = s.Store.SetSetting(settingAPKPendingUninstall, string(b))
}

func (s *Service) addPendingAPKUninstall(pkg string) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return
	}
	cur := s.pendingAPKUninstalls()
	for _, p := range cur {
		if p == pkg {
			return
		}
	}
	s.setPendingAPKUninstalls(append(cur, pkg))
}

func (s *Service) removePendingAPKUninstall(pkg string) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return
	}
	cur := s.pendingAPKUninstalls()
	if len(cur) == 0 {
		return
	}
	var next []string
	for _, p := range cur {
		if p != pkg {
			next = append(next, p)
		}
	}
	if len(next) != len(cur) {
		s.setPendingAPKUninstalls(next)
	}
}

func uninstallAPKOnDevice(ctx context.Context, c *control.Device, pkg string) bool {
	uctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, _ = c.ADB.Run(uctx, "uninstall", pkg)
	// `pm path` exits non-zero when pkg is absent, so append a sentinel that only
	// prints if ADB shell itself succeeded and pkg is gone.
	out, err := c.ADB.Shell(uctx, "pm path "+pkg+" 2>/dev/null || echo __ABSENT__")
	return err == nil && !strings.Contains(out, "package:") && strings.Contains(out, "__ABSENT__")
}

func (s *Service) flushPendingAPKUninstalls(ctx context.Context, c *control.Device) {
	pending := s.pendingAPKUninstalls()
	if len(pending) == 0 || c == nil {
		return
	}
	var remain []string
	for _, pkg := range pending {
		if !apk.ValidPackageName(pkg) {
			continue
		}
		if !uninstallAPKOnDevice(ctx, c, pkg) {
			remain = append(remain, pkg)
		}
	}
	s.setPendingAPKUninstalls(remain)
}

func (s *Service) makeAPKShortcut(rec store.MobileAPK) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return apk.CreateShortcut(apk.ShortcutSpec{
		Package:     rec.Package,
		Label:       rec.Label,
		AnyProfBin:  exe,
		IconDataURL: rec.IconData,
		IconsDir:    filepath.Join(s.apkCacheDir(), "icons"),
	})
}

func (s *Service) ensureAPKDevice(ctx context.Context, preferredID string) (string, error) {
	if preferredID != "" {
		if _, err := s.liveMobile(preferredID); err == nil {
			return preferredID, nil
		}
	}
	devs, err := s.Store.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		return "", err
	}
	// Standalone APKs always run on a dedicated hidden Android host so they
	// never collide with or leak into the user's Mobile profiles.
	for _, d := range devs {
		if d.DeletedAt == "" && d.Notes == apkHostNotes {
			return d.ID, nil
		}
	}
	// Lightweight headless-first host: 2 GB RAM, 2 vCPUs, no GMS overhead.
	withGMS := false
	created, err := s.CreateMobileDevice(ctx, MobileInput{
		Name:    defaultAPKDeviceName,
		Preset:  "automation",
		WithGMS: &withGMS,
		Notes:   apkHostNotes,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// apkTrimBloat disables unused Android background packages on the dedicated APK
// host (mirroring apkstart's container trim list) so memory and idle CPU stay low.
func (s *Service) apkTrimBloat(ctx context.Context, c *control.Device) {
	const cmd = "for p in com.android.printspooler com.android.wallpapercropper " +
		"com.android.dreams.basic com.android.providers.calendar com.android.providers.contacts " +
		"com.android.managedprovisioning com.android.statementservice com.android.backupconfirm " +
		"com.google.android.setupwizard; do pm disable-user --user 0 \"$p\" >/dev/null 2>&1 || true; done; " +
		"settings put global restricted_background_data 1 >/dev/null 2>&1 || true"
	_, _ = c.ADB.Shell(ctx, cmd)
}

// apkTombstoneOthers force-stops other previously cataloged user APKs so only
// the active app holds memory inside the shared 2 GB host.
func (s *Service) apkTombstoneOthers(ctx context.Context, c *control.Device, activePkg string) {
	list, err := s.Store.ListMobileAPKs()
	if err != nil {
		return
	}
	for _, a := range list {
		if a.Package != "" && a.Package != activePkg && apk.ValidPackageName(a.Package) {
			_, _ = c.ADB.Shell(ctx, "am force-stop "+a.Package)
		}
	}
}

func copyAPKFile(src, dst string) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// Keep mobile import referenced for state constants if needed in future hooks.
var _ = mobile.Running
