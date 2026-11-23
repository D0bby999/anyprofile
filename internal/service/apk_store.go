// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/apk"
	"github.com/D0bby999/anyprofile/internal/store"
)

// APKStoreApp is a search result row shown in the New APK search dropdown.
type APKStoreApp struct {
	Package   string `json:"package"`
	Title     string `json:"title"`
	Developer string `json:"developer"`
	IconURL   string `json:"iconUrl"`
	Rating    string `json:"rating,omitempty"`
	Category  string `json:"category,omitempty"`
}

var popularAPKStoreApps = []APKStoreApp{
	{Package: "com.zhiliaoapp.musically", Title: "TikTok", Developer: "TikTok Pte. Ltd.", Category: "Social"},
	{Package: "com.instagram.android", Title: "Instagram", Developer: "Instagram", Category: "Social"},
	{Package: "com.facebook.katana", Title: "Facebook", Developer: "Meta Platforms, Inc.", Category: "Social"},
	{Package: "org.telegram.messenger", Title: "Telegram", Developer: "Telegram FZ-LLC", Category: "Communication"},
	{Package: "com.whatsapp", Title: "WhatsApp Messenger", Developer: "WhatsApp LLC", Category: "Communication"},
	{Package: "com.shopee.vn", Title: "Shopee", Developer: "Shopee", Category: "Shopping"},
	{Package: "com.twitter.android", Title: "X", Developer: "X Corp.", Category: "Social"},
	{Package: "com.zing.zalo", Title: "Zalo", Developer: "VNG Corporation", Category: "Communication"},
	{Package: "com.lemon.lvoverseas", Title: "CapCut", Developer: "Bytedance Pte. Ltd.", Category: "Video"},
	{Package: "com.ss.android.ugc.trill", Title: "TikTok Asia", Developer: "TikTok Pte. Ltd.", Category: "Social"},
}

var (
	apkPlaySearchURL = "https://play.google.com/store/search"
	apkSearchCacheMu sync.Mutex
	apkSearchCache   = map[string]apkSearchCacheEntry{}
	playDetailsIDRe  = regexp.MustCompile(`details\?id=([A-Za-z][A-Za-z0-9_.]+)`)
	playDS4Re        = regexp.MustCompile(`AF_initDataCallback\(\{key:\s*'ds:4',\s*hash:\s*'[^']+',\s*data:([\s\S]*?),\s*sideChannel:`)
)

type apkSearchCacheEntry struct {
	apps []APKStoreApp
	exp  time.Time
}

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

// SearchAPKStore searches public Google Play for Android apps matching query,
// returning icons, titles, developers and package IDs for the New APK dropdown.
// Empty queries return the curated popular list with zero network traffic;
// network or parse failures fall back to filtering the curated list.
func (s *Service) SearchAPKStore(ctx context.Context, query, proxyID string) ([]APKStoreApp, error) {
	if err := s.mobileAllowed(); err != nil {
		return nil, err
	}
	q := strings.TrimSpace(query)
	if q == "" {
		out := make([]APKStoreApp, len(popularAPKStoreApps))
		copy(out, popularAPKStoreApps)
		return out, nil
	}

	proxyID = strings.TrimSpace(proxyID)
	region := s.proxyRegion(proxyID)
	cacheKey := strings.ToLower(q) + "|" + proxyID + "|" + region
	apkSearchCacheMu.Lock()
	if hit, ok := apkSearchCache[cacheKey]; ok && time.Now().Before(hit.exp) {
		out := make([]APKStoreApp, len(hit.apps))
		copy(out, hit.apps)
		apkSearchCacheMu.Unlock()
		return out, nil
	}
	apkSearchCacheMu.Unlock()

	apps := s.fetchPlayStoreApps(ctx, q, proxyID, region)
	if len(apps) == 0 {
		apps = filterPopularAPKStoreApps(q)
	}
	if len(apps) > 8 {
		apps = apps[:8]
	}

	apkSearchCacheMu.Lock()
	if len(apkSearchCache) > 64 {
		apkSearchCache = map[string]apkSearchCacheEntry{}
	}
	apkSearchCache[cacheKey] = apkSearchCacheEntry{apps: apps, exp: time.Now().Add(5 * time.Minute)}
	apkSearchCacheMu.Unlock()

	out := make([]APKStoreApp, len(apps))
	copy(out, apps)
	return out, nil
}

func (s *Service) proxyRegion(proxyID string) string {
	if proxyID == "" {
		return ""
	}
	p, err := s.Store.GetProxy(proxyID)
	if err != nil {
		return ""
	}
	cc := strings.ToUpper(strings.TrimSpace(p.Country))
	if len(cc) != 2 {
		return ""
	}
	return cc
}

func (s *Service) fetchPlayStoreApps(ctx context.Context, query, proxyID, region string) []APKStoreApp {
	transport := apkHTTPTransport()
	if proxyID != "" {
		up, err := s.proxyUpstream(proxyID)
		if err != nil {
			return nil
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
			return up.DialTunnel(ctx, addr)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	u, err := url.Parse(apkPlaySearchURL)
	if err != nil {
		return nil
	}
	v := u.Query()
	v.Set("q", query)
	v.Set("c", "apps")
	if s.Language() == "vi" {
		v.Set("hl", "vi")
	} else {
		v.Set("hl", "en")
	}
	if region != "" {
		v.Set("gl", region)
	} else if s.Language() == "vi" {
		v.Set("gl", "VN")
	} else {
		v.Set("gl", "US")
	}
	u.RawQuery = v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil
	}
	return parsePlaySearchHTML(string(body))
}

func filterPopularAPKStoreApps(query string) []APKStoreApp {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []APKStoreApp
	for _, a := range popularAPKStoreApps {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Package), q) ||
			strings.Contains(strings.ToLower(a.Developer), q) {
			out = append(out, a)
		}
	}
	return out
}

func parsePlaySearchHTML(html string) []APKStoreApp {
	m := playDS4Re.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil
	}
	var root []any
	if err := json.Unmarshal([]byte(m[1]), &root); err != nil {
		return nil
	}
	sections := jsonSlice(jsonAt(root, 0, 1))
	if len(sections) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []APKStoreApp
	add := func(app APKStoreApp) {
		app.Package = strings.TrimSpace(app.Package)
		app.Title = strings.TrimSpace(app.Title)
		if app.Package == "" || app.Title == "" || !apk.ValidPackageName(app.Package) || seen[app.Package] {
			return
		}
		if app.IconURL != "" && strings.Contains(app.IconURL, "play-lh.googleusercontent.com") && !strings.Contains(app.IconURL, "=s") {
			app.IconURL += "=s96"
		}
		seen[app.Package] = true
		out = append(out, app)
	}

	for _, sec := range sections {
		// Hero top-match card (section[23][16][2]).
		if hero := jsonAt(sec, 23, 16, 2); hero != nil {
			title := jsonStr(jsonAt(hero, 0, 0))
			dev := jsonStr(jsonAt(hero, 68, 0))
			icon := jsonStr(jsonAt(hero, 95, 0, 3, 2))
			rating := jsonStr(jsonAt(hero, 51, 0, 0))
			pkg := findDetailsPackage(sec)
			if pkg != "" && title != "" {
				add(APKStoreApp{Package: pkg, Title: title, Developer: dev, IconURL: icon, Rating: rating})
			}
		}
		// Standard search result list (section[22][0]).
		for _, entry := range jsonSlice(jsonAt(sec, 22, 0)) {
			it := jsonAt(entry, 0)
			if it == nil {
				continue
			}
			pkg := jsonStr(jsonAt(it, 0, 0))
			title := jsonStr(jsonAt(it, 3))
			icon := jsonStr(jsonAt(it, 1, 3, 2))
			rating := jsonStr(jsonAt(it, 4, 0))
			cat := jsonStr(jsonAt(it, 5))
			dev := jsonStr(jsonAt(it, 14))
			if pkg != "" && title != "" {
				add(APKStoreApp{Package: pkg, Title: title, Developer: dev, IconURL: icon, Rating: rating, Category: cat})
			}
		}
	}
	return out
}

func jsonAt(v any, indices ...int) any {
	cur := v
	for _, idx := range indices {
		arr, ok := cur.([]any)
		if !ok || idx < 0 || idx >= len(arr) {
			return nil
		}
		cur = arr[idx]
	}
	return cur
}

func jsonSlice(v any) []any {
	arr, _ := v.([]any)
	return arr
}

func jsonStr(v any) string {
	s, _ := v.(string)
	return s
}

func findDetailsPackage(v any) string {
	switch x := v.(type) {
	case string:
		if m := playDetailsIDRe.FindStringSubmatch(x); len(m) == 2 {
			return m[1]
		}
	case []any:
		for _, child := range x {
			if pkg := findDetailsPackage(child); pkg != "" {
				return pkg
			}
		}
	}
	return ""
}
