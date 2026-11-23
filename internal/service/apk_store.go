// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/apk"
	"github.com/D0bby999/anyprofile/internal/store"
)

const (
	auroraStorePkg   = "com.aurora.store"
	maxAPKURLBytes   = 2 << 30 // 2 GiB safety cap for direct URL downloads
	apkDownloadsSubd = ".downloads"
)

// APKStoreRequest opens Aurora Store inside the APK host (or a selected Mobile
// device), deep-linking straight to an app page or search query when provided.
type APKStoreRequest struct {
	Query    string `json:"query"`
	ProxyID  string `json:"proxyId"`
	DeviceID string `json:"deviceId"`
}

// UncatalogedAPK represents a 3rd-party app installed on an Android device
// that is not yet saved in AnyProfile's standalone APK list.
type UncatalogedAPK struct {
	Package  string `json:"package"`
	DeviceID string `json:"deviceId"`
}

// APKImportInput imports an already-installed Android app from the APK Runner
// (or a running Mobile device) into AnyProfile's APK catalog and local cache.
type APKImportInput struct {
	Package          string `json:"package"`
	DeviceID         string `json:"deviceId"`
	Label            string `json:"label"`
	ProxyID          string `json:"proxyId"`
	Notes            string `json:"notes"`
	Orientation      string `json:"orientation"`
	IdentityTemplate string `json:"identityTemplate"`
	CreateShortcut   bool   `json:"createShortcut"`
}

var ignoredStorePackages = map[string]bool{
	auroraStorePkg:                    true,
	"com.android.vending":             true,
	"com.google.android.gms":          true,
	"com.google.android.gsf":          true,
	"com.google.android.partnersetup": true,
}

// OpenAPKStore boots or wakes the APK host, applies the optional proxy, ensures
// Aurora Store is installed, and opens Aurora directly to req.Query (Pro-gated).
func (s *Service) OpenAPKStore(ctx context.Context, req APKStoreRequest) error {
	if err := s.mobileAllowed(); err != nil {
		return err
	}
	proxyID := strings.TrimSpace(req.ProxyID)
	if proxyID != "" {
		if _, err := s.Store.GetProxy(proxyID); err != nil {
			return apperr.New("proxy_not_found", "selected proxy does not exist")
		}
	}
	targetID, err := s.ensureAPKDevice(ctx, strings.TrimSpace(req.DeviceID))
	if err != nil {
		return err
	}
	if !s.mobileRunning(targetID) {
		if _, err := s.StartMobileDeviceHeadless(ctx, targetID); err != nil {
			return err
		}
	}
	if err := s.applyAPKProxy(ctx, targetID, proxyID); err != nil {
		return err
	}
	if s.Mobile != nil {
		if resumer, ok := s.Mobile.Driver().(interface{ Resume(id string) error }); ok {
			_ = resumer.Resume(targetID)
		}
	}
	c, err := s.MobileControl(targetID)
	if err != nil {
		return err
	}
	_, _ = c.ADB.Shell(ctx, "dumpsys deviceidle unforce 2>/dev/null || true; settings put system screen_off_timeout 2147483647; svc power stayon true; input keyevent 224; wm dismiss-keyguard")
	if err := c.EnsureAurora(ctx); err != nil {
		return apperr.New("mobile_store", fmt.Sprintf("could not prepare Aurora Store: %v", err))
	}
	if cmd := storeDeepLinkCmd(req.Query); cmd != "" {
		if out, err := c.ADB.Shell(ctx, cmd); err != nil || strings.Contains(out, "Error:") {
			if lerr := c.Launch(ctx, auroraStorePkg); lerr != nil {
				return lerr
			}
		}
	} else if err := c.Launch(ctx, auroraStorePkg); err != nil {
		return err
	}
	return s.showMobileDeviceWithTitle(ctx, targetID, "Aurora Store", true)
}

func storeDeepLinkCmd(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if apk.ValidPackageName(q) {
		return fmt.Sprintf("am start -a android.intent.action.VIEW -d 'market://details?id=%s' -p %s", q, auroraStorePkg)
	}
	return fmt.Sprintf("am start -a android.intent.action.VIEW -d 'market://search?q=%s' -p %s", url.QueryEscape(q), auroraStorePkg)
}

// ListUncatalogedAPKs returns 3rd-party packages installed on the running APK
// host (or specified running Mobile device) that are not yet in the APK catalog.
// Never boots or wakes a stopped/paused VM so background polling costs ~0 CPU.
func (s *Service) ListUncatalogedAPKs(ctx context.Context, deviceID string) ([]UncatalogedAPK, error) {
	if err := s.mobileAllowed(); err != nil {
		return nil, err
	}
	hostID, ok := s.apkHostID(strings.TrimSpace(deviceID))
	if !ok || !s.mobileRunning(hostID) {
		return []UncatalogedAPK{}, nil
	}
	if s.Mobile != nil {
		if p, ok := s.Mobile.Driver().(mobile.Pauser); ok && p.Paused(hostID) {
			return []UncatalogedAPK{}, nil
		}
	}
	c, err := s.MobileControl(hostID)
	if err != nil {
		return []UncatalogedAPK{}, nil
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := c.ADB.Shell(pctx, "pm list packages -3")
	if err != nil {
		return []UncatalogedAPK{}, nil
	}
	existing, err := s.Store.ListMobileAPKs()
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(existing))
	for _, a := range existing {
		known[a.Package] = true
	}
	pkgs := parseThirdPartyPackages(out, known)
	res := make([]UncatalogedAPK, 0, len(pkgs))
	for _, pkg := range pkgs {
		res = append(res, UncatalogedAPK{Package: pkg, DeviceID: hostID})
	}
	return res, nil
}

func parseThirdPartyPackages(out string, known map[string]bool) []string {
	seen := map[string]bool{}
	var pkgs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "package:") {
			continue
		}
		pkg := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		if pkg == "" || ignoredStorePackages[pkg] || known[pkg] || seen[pkg] || !apk.ValidPackageName(pkg) {
			continue
		}
		seen[pkg] = true
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	return pkgs
}

// ImportInstalledAPK pulls an app (including split APKs) already installed on
// the APK Runner or a running Mobile device, caches it locally, inspects its
// metadata/icon, and saves it into AnyProfile's APK list (Pro-gated).
func (s *Service) ImportInstalledAPK(ctx context.Context, in APKImportInput) (store.MobileAPK, error) {
	if err := s.mobileAllowed(); err != nil {
		return store.MobileAPK{}, err
	}
	pkg := strings.TrimSpace(in.Package)
	if !apk.ValidPackageName(pkg) {
		return store.MobileAPK{}, apperr.New("mobile_package", "invalid package name")
	}
	proxyID := strings.TrimSpace(in.ProxyID)
	if proxyID != "" {
		if _, err := s.Store.GetProxy(proxyID); err != nil {
			return store.MobileAPK{}, apperr.New("proxy_not_found", "selected proxy does not exist")
		}
	}
	srcID, ok := s.apkHostID(strings.TrimSpace(in.DeviceID))
	if !ok || !s.mobileRunning(srcID) {
		return store.MobileAPK{}, apperr.New("apk_not_running", "source Android device is not running")
	}
	if s.Mobile != nil {
		if resumer, ok := s.Mobile.Driver().(interface{ Resume(id string) error }); ok {
			_ = resumer.Resume(srcID)
		}
	}
	c, err := s.MobileControl(srcID)
	if err != nil {
		return store.MobileAPK{}, err
	}
	out, err := c.ADB.Shell(ctx, "pm path "+pkg)
	if err != nil {
		return store.MobileAPK{}, apperr.New("mobile_apk", fmt.Sprintf("could not locate %s on device: %v", pkg, err))
	}
	remotePaths := parsePMPaths(out)
	if len(remotePaths) == 0 {
		return store.MobileAPK{}, apperr.New("mobile_apk", fmt.Sprintf("%s is not installed on device", pkg))
	}

	tmpDir, err := os.MkdirTemp("", "anyprofile-pull-*")
	if err != nil {
		return store.MobileAPK{}, err
	}
	defer os.RemoveAll(tmpDir)

	var pulled []string
	for i, rp := range remotePaths {
		name := filepath.Base(rp)
		if name == "" || name == "." || name == "/" || !strings.HasSuffix(strings.ToLower(name), ".apk") {
			name = fmt.Sprintf("split_%d.apk", i)
		}
		dst := filepath.Join(tmpDir, fmt.Sprintf("%02d_%s", i, name))
		if _, err := c.ADB.Run(ctx, "pull", rp, dst); err != nil {
			return store.MobileAPK{}, apperr.New("mobile_apk", fmt.Sprintf("failed to pull %s: %v", pkg, err))
		}
		pulled = append(pulled, dst)
	}

	cacheDir := s.apkCacheDir()
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return store.MobileAPK{}, err
	}
	var cachedPath string
	if len(pulled) == 1 {
		cachedPath = filepath.Join(cacheDir, pkg+".apk")
		if err := copyAPKFile(pulled[0], cachedPath); err != nil {
			return store.MobileAPK{}, err
		}
	} else {
		cachedPath = filepath.Join(cacheDir, pkg+".apks")
		if err := packSplitAPKs(pulled, cachedPath); err != nil {
			return store.MobileAPK{}, err
		}
	}

	info, err := s.InspectAPK(ctx, cachedPath)
	if err != nil {
		_ = os.Remove(cachedPath)
		return store.MobileAPK{}, err
	}

	devID, err := s.ensureAPKDevice(ctx, "")
	if err != nil {
		return store.MobileAPK{}, err
	}
	// If imported from a user's Mobile device instead of APK Runner, also install
	// onto APK Runner so standalone launches work immediately.
	if srcID != devID {
		if !s.mobileRunning(devID) {
			if _, err := s.StartMobileDeviceHeadless(ctx, devID); err != nil {
				return store.MobileAPK{}, err
			}
		}
		if _, err := s.InstallMobileApps(ctx, devID, []string{cachedPath}); err != nil {
			return store.MobileAPK{}, err
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
	idn, aid := ensureAPKIdentityFields(info.Package, "", "", in.IdentityTemplate)
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
		ProxyID:        proxyID,
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

func parsePMPaths(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "package:") {
			continue
		}
		p := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		if strings.HasPrefix(p, "/") {
			paths = append(paths, p)
		}
	}
	return paths
}

// packSplitAPKs bundles split APK files into an uncompressed .apks ZIP archive
// atomically via a temporary file so apk.Parse and install-multiple read it cleanly.
func packSplitAPKs(srcPaths []string, dstPath string) error {
	tmp := dstPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for i, sp := range srcPaths {
		in, err := os.Open(sp)
		if err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmp)
			return err
		}
		name := filepath.Base(sp)
		if name == "" {
			name = fmt.Sprintf("split_%d.apk", i)
		}
		hdr := &zip.FileHeader{Name: name, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			in.Close()
			zw.Close()
			f.Close()
			_ = os.Remove(tmp)
			return err
		}
		_, err = io.Copy(w, in)
		in.Close()
		if err != nil {
			zw.Close()
			f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dstPath)
}

var apkHTTPTransport = func() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

// DownloadAPKFromURL downloads an .apk/.apks/.xapk/.apkm file from an HTTPS URL
// (routed strictly through proxyID when set), validates its ZIP/APK header, and
// returns APKInspect for the existing confirmation dialog (Pro-gated).
func (s *Service) DownloadAPKFromURL(ctx context.Context, rawURL, proxyID string) (APKInspect, error) {
	if err := s.mobileAllowed(); err != nil {
		return APKInspect{}, err
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return APKInspect{}, apperr.New("mobile_apk", "enter a valid https:// URL pointing to an .apk, .apks, .xapk or .apkm file")
	}

	transport := apkHTTPTransport()
	proxyID = strings.TrimSpace(proxyID)
	if proxyID != "" {
		up, err := s.proxyUpstream(proxyID)
		if err != nil {
			return APKInspect{}, err
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
			return up.DialTunnel(ctx, addr)
		}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" {
				return errors.New("refusing non-https redirect")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return APKInspect{}, apperr.New("mobile_apk", err.Error())
	}
	req.Header.Set("User-Agent", "AnyProfile/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return APKInspect{}, apperr.New("mobile_apk", fmt.Sprintf("download failed: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return APKInspect{}, apperr.New("mobile_apk", fmt.Sprintf("download returned HTTP %d", resp.StatusCode))
	}

	ext := apkExtFromURL(resp.Request.URL.Path)
	dlDir := filepath.Join(s.apkCacheDir(), apkDownloadsSubd)
	if err := os.MkdirAll(dlDir, 0o700); err != nil {
		return APKInspect{}, err
	}
	tmp, err := os.CreateTemp(dlDir, "dl-*"+ext)
	if err != nil {
		return APKInspect{}, err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	var magic [4]byte
	n, err := io.ReadFull(resp.Body, magic[:])
	if err != nil || n < 4 || !isZIPMagic(magic[:]) {
		cleanup()
		return APKInspect{}, apperr.New("mobile_apk", "URL did not return a valid APK or split bundle file")
	}
	if _, err := tmp.Write(magic[:]); err != nil {
		cleanup()
		return APKInspect{}, err
	}
	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxAPKURLBytes-4+1))
	if err != nil {
		cleanup()
		return APKInspect{}, apperr.New("mobile_apk", fmt.Sprintf("download interrupted: %v", err))
	}
	if written > maxAPKURLBytes-4 {
		cleanup()
		return APKInspect{}, apperr.New("mobile_apk", "APK file exceeds the 2 GB download limit")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return APKInspect{}, err
	}

	info, err := s.InspectAPK(ctx, tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return APKInspect{}, err
	}
	return info, nil
}

func isZIPMagic(b []byte) bool {
	return len(b) >= 4 && b[0] == 'P' && b[1] == 'K' && b[2] == 0x03 && b[3] == 0x04
}

func apkExtFromURL(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".apk", ".apks", ".xapk", ".apkm":
		return strings.ToLower(filepath.Ext(p))
	default:
		return ".apk"
	}
}
