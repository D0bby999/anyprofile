// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"context"
	"encoding/base64"
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
	{Package: "com.zhiliaoapp.musically", Title: "TikTok", Developer: "TikTok Pte. Ltd.", Category: "Social", Rating: "4.4", IconURL: "https://play-lh.googleusercontent.com/2o7iuqPe-0msP3DvNhzoOgNo4eZ6GkL_RtGVeKyBn5UBQAW5cw-GW5V3wDOHB1yTQuqWzrFLFOv2u4_5ea1uhg=s96"},
	{Package: "com.instagram.android", Title: "Instagram", Developer: "Instagram", Category: "Social", Rating: "4.4", IconURL: "https://play-lh.googleusercontent.com/yHi59jmO_lVamcyJ1i3rM1_E8bAiAspShnGjjURq05ipQQSUksO3QVEsXTegRSqul038-4YNA7O644XAcx251Q=s96"},
	{Package: "com.facebook.katana", Title: "Facebook", Developer: "Meta Platforms, Inc.", Category: "Social", Rating: "4.3", IconURL: "https://play-lh.googleusercontent.com/12KEJDhgk6oyE1VgmfCuFzFrQripZ_endElbvEhU1rZawQgV3RI-v3II8fDslI1sdC_WKBkmtxS3jxuIihk33Q=s96"},
	{Package: "com.twitter.android", Title: "X", Developer: "X Corp.", Category: "Social", Rating: "4.2", IconURL: "https://play-lh.googleusercontent.com/IsLsCD4eLC2CYvlkgTI6Z5TROj0BJIBgUF7BZ5eliTzyyTjqG_mbofeG4kw2s7oG_JZMSi5ErLz_qsDi169C8w=s96"},
	{Package: "com.whatsapp", Title: "WhatsApp Messenger", Developer: "WhatsApp LLC", Category: "Communication", Rating: "4.3", IconURL: "https://play-lh.googleusercontent.com/Gqxk4T0uZsDwFp07DE-508hkyvcNmgFuRwPiwTEfF7D7OzGv1FdHDzEyMxNsSBZLOJlGpe3ULvVM2RgrRAlBqA=s96"},
	{Package: "org.telegram.messenger", Title: "Telegram", Developer: "Telegram FZ-LLC", Category: "Communication", Rating: "4.5", IconURL: "https://play-lh.googleusercontent.com/PyZ3akMGXPV0tKKirKNwfO--PSQW3FHR6rD_H9mAaukZ8LiHyYFuBLeU8UZ2ok6r5SP79-3prkfybWKh98AZAD0=s96"},
	{Package: "com.lemon.lvoverseas", Title: "CapCut", Developer: "Bytedance Pte. Ltd.", Category: "Video", Rating: "4.4", IconURL: "https://play-lh.googleusercontent.com/M78HyakHaxKrjoeqYx41E9DXfVYYtx67nvc7Ks4G4zFQeaAJdGCi8gzzGSrHIwlrmnJS6zD9S4fAXqdEwfuHQAQ=s96"},
	{Package: "com.instagram.barcelona", Title: "Threads", Developer: "Instagram", Category: "Social", Rating: "4.4", IconURL: "https://play-lh.googleusercontent.com/6PzXaP0KaRU4DifAfsXKbWNVA3JPrP9uklvEh5EDhzPRdPfHV4tzszhAavQa4bq0YGdbnphBucz9vEUXiu1Ebg=s96"},
	{Package: "com.google.android.youtube", Title: "YouTube", Developer: "Google LLC", Category: "Video", Rating: "4.3", IconURL: "https://play-lh.googleusercontent.com/QNmuZQc9I6Zbe3mWnSr0hycnENqGFCI5p3yE29Hkxtf22T0IWS6zTrpxULLyyjWpB7ONAXDsDQXnXcVWokl3eg=s96"},
	{Package: "com.discord", Title: "Discord", Developer: "Discord Inc.", Category: "Communication", Rating: "4.5", IconURL: "https://play-lh.googleusercontent.com/w2DdjDHGSj3DQElQQM0_J3PsNsALS-WD3CBeH0aXOYceR-F3aT02gFonhoFzz-Tc9ec8Dx9cLqh48-tzIFcP=s96"},
	{Package: "com.reddit.frontpage", Title: "Reddit", Developer: "reddit Inc.", Category: "Social", Rating: "4.6", IconURL: "https://play-lh.googleusercontent.com/8FVS0kvM60JwqZWRNzBIlwrZZxJa8vpli8IWaaTo8QXRe1PHmCNJ8UpWoXkGPV3jKEaTY2OVAeYoisOnGJXY=s96"},
	{Package: "com.snapchat.android", Title: "Snapchat", Developer: "Snap Inc", Category: "Social", Rating: "4.2", IconURL: "https://play-lh.googleusercontent.com/nOJWJoDAzuRSHmlFfpzHXAj8UbTJwk_JGYf0uAPTKPtiiIX_2Y7r5ATO1ZtxHRqaFTqfIDZ40PnBK4hukeJh=s96"},
	{Package: "com.pinterest", Title: "Pinterest", Developer: "Pinterest", Category: "Lifestyle", Rating: "4.6", IconURL: "https://play-lh.googleusercontent.com/VExMwhu9Fe5kz72YnPWP6mHrNA5WfgZ-PJMnBnQmXFpLncTAOaaHtK4b8w7b2xxGxtqycDatYslVyNSLucl9QA=s96"},
	{Package: "com.amazon.mShop.android.shopping", Title: "Amazon Shopping", Developer: "Amazon Mobile LLC", Category: "Shopping", Rating: "4.5", IconURL: "https://play-lh.googleusercontent.com/UIYZ9hTFNg-zZUg_uzLS9YdyDpvoo1VHNFb6VlzHy5JqNMq9Uq-p2S1VPwzXCipHTFacfGKhJkyAuKL19VVlcWA=s96"},
	{Package: "com.shopee.vn", Title: "Shopee", Developer: "Shopee", Category: "Shopping", Rating: "4.6", IconURL: "https://play-lh.googleusercontent.com/mlS7AEDM9Ef-bEd_kc25xhtmJQN6hgpEEd3BQm20kIWMJNKWYfO93tuNJBT9WVcs8oZ9tpHdAPbEXI6FSsyw=s96"},
	{Package: "com.zing.zalo", Title: "Zalo", Developer: "VNG Corporation", Category: "Communication", Rating: "4.2", IconURL: "https://play-lh.googleusercontent.com/D3QF_jSgL9vmGcZ4GwaaqoyIRi-KqXh10aeHkMNZyNO4PT2MoGqmfsW63u-4CLCNQ-BdCPbKofBg8V7Rh8wgWQ=s96"},
	{Package: "jp.naver.line.android", Title: "LINE", Developer: "LY Corporation", Category: "Communication", Rating: "4.2", IconURL: "https://play-lh.googleusercontent.com/7PeaZXtTu7EuCuWzGpkXWAnT9pLs_Ly9MbVSeZUULlIc5H02wGWzvrbmqxj86UnkM9gjUaGSeAcKbYbfbegTJw=s96"},
}

var regionalPopularOrder = map[string][]string{
	"VN": {
		"com.zhiliaoapp.musically", "com.facebook.katana", "com.zing.zalo", "com.shopee.vn",
		"com.instagram.android", "org.telegram.messenger", "com.lemon.lvoverseas", "com.whatsapp",
		"com.instagram.barcelona", "com.google.android.youtube",
	},
	"JP": {
		"com.zhiliaoapp.musically", "jp.naver.line.android", "com.twitter.android", "com.instagram.android",
		"com.google.android.youtube", "com.facebook.katana", "org.telegram.messenger", "com.discord",
		"com.lemon.lvoverseas", "com.instagram.barcelona",
	},
	"TW": {
		"com.zhiliaoapp.musically", "jp.naver.line.android", "com.instagram.android", "com.facebook.katana",
		"com.instagram.barcelona", "com.google.android.youtube", "org.telegram.messenger", "com.twitter.android",
	},
	"TH": {
		"com.zhiliaoapp.musically", "jp.naver.line.android", "com.facebook.katana", "com.instagram.android",
		"com.lemon.lvoverseas", "org.telegram.messenger", "com.twitter.android", "com.google.android.youtube",
	},
}

func (s *Service) popularAppsForRegion(region string) []APKStoreApp {
	byPkg := make(map[string]APKStoreApp, len(popularAPKStoreApps))
	for _, a := range popularAPKStoreApps {
		byPkg[a.Package] = a
	}
	cc := strings.ToUpper(strings.TrimSpace(region))
	if cc == "" && s.Language() == "vi" {
		cc = "VN"
	}
	order := regionalPopularOrder[cc]
	if len(order) == 0 {
		// Global / US / EU order: social, messaging, creator & shopping leaders.
		order = []string{
			"com.zhiliaoapp.musically", "com.instagram.android", "com.facebook.katana", "com.twitter.android",
			"com.whatsapp", "org.telegram.messenger", "com.lemon.lvoverseas", "com.instagram.barcelona",
			"com.discord", "com.reddit.frontpage", "com.snapchat.android", "com.amazon.mShop.android.shopping",
		}
	}
	out := make([]APKStoreApp, 0, len(order))
	for _, pkg := range order {
		if a, ok := byPkg[pkg]; ok {
			out = append(out, a)
		}
	}
	return out
}

var (
	apkPlaySearchURL      = "https://play.google.com/store/search"
	aptoideMetaBaseURL    = "https://ws75.aptoide.com/api/7/app/getMeta"
	fdroidAPIBaseURL      = "https://f-droid.org/api/v1/packages"
	fdroidRepoBaseURL     = "https://f-droid.org/repo"
	directOfficialAPKURLs = map[string]string{
		"org.telegram.messenger": "https://telegram.org/dl/android/apk",
	}
	// Packages whose Aptoide aab=1 response omits required dynamic-feature DEX splits
	// (causing immediate ClassNotFoundException on startup); route these to Aurora Store.
	aptoideIncompleteSplitPkgs = map[string]bool{
		"com.zhiliaoapp.musically": true,
	}
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

// APKStoreInstallInput downloads a store package directly through the optional
// proxy (single APK or split APKs), installs it onto the APK host, catalogs it,
// and optionally launches it immediately without requiring Aurora Store UI.
type APKStoreInstallInput struct {
	Package          string `json:"package"`
	Label            string `json:"label"`
	ProxyID          string `json:"proxyId"`
	Orientation      string `json:"orientation"`
	IdentityTemplate string `json:"identityTemplate"`
	Notes            string `json:"notes"`
	CreateShortcut   bool   `json:"createShortcut"`
	LaunchAfter      bool   `json:"launchAfter"`
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
	s.flushPendingAPKUninstalls(ctx, c)
	if err := c.EnsureAurora(ctx); err != nil {
		return apperr.New("mobile_store", fmt.Sprintf("could not prepare Aurora Store: %v", err))
	}
	// Only deep-link straight to an app page if Aurora Store has already finished
	// first-time onboarding / anonymous sign-in; deep-linking with null authData
	// causes Aurora's AppDetailsHelper to throw NullPointerException.
	auroraReady := false
	if out, err := c.ADB.Shell(ctx, "su 0 grep -q PREFERENCE_AUTH_DATA /data/data/com.aurora.store/shared_prefs/*.xml 2>/dev/null && echo OK"); err == nil && strings.Contains(out, "OK") {
		auroraReady = true
	}
	if auroraReady {
		if cmd := storeDeepLinkCmd(req.Query); cmd != "" {
			if out, err := c.ADB.Shell(ctx, cmd); err != nil || strings.Contains(out, "Error:") {
				if lerr := c.Launch(ctx, auroraStorePkg); lerr != nil {
					return lerr
				}
			}
		} else if err := c.Launch(ctx, auroraStorePkg); err != nil {
			return err
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
	// If this is the dedicated APK host, clear any pending deletions before listing
	// so previously deleted apps can never reappear in the catalog.
	if apkHost, ok := s.apkHostID(""); ok && apkHost == hostID {
		s.flushPendingAPKUninstalls(ctx, c)
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
	for _, p := range s.pendingAPKUninstalls() {
		known[p] = true
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
	idn, aid := s.ensureAPKIdentityFields(info.Package, "", "", in.IdentityTemplate)
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
	s.removePendingAPKUninstall(rec.Package)
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

type storeAPKPart struct {
	name   string
	url    string
	altURL string
	size   int64
}

func (s *Service) emitAPKProgress(pkg, phase string, done, total int64) {
	if s.OnAPKProgress != nil {
		s.OnAPKProgress(pkg, phase, done, total)
	}
}

func (s *Service) apkHTTPClient(proxyID string, timeout time.Duration) (*http.Client, error) {
	transport := apkHTTPTransport()
	proxyID = strings.TrimSpace(proxyID)
	if proxyID != "" {
		up, err := s.proxyUpstream(proxyID)
		if err != nil {
			return nil, err
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
			return up.DialTunnel(ctx, addr)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" {
				return errors.New("refusing non-https redirect")
			}
			return nil
		},
	}, nil
}

// InstallStoreAPK downloads a package (single APK or split APK bundle) directly
// through req.ProxyID, installs and catalogs it on the APK host, and launches
// it directly when req.LaunchAfter is true (Pro-gated).
func (s *Service) InstallStoreAPK(ctx context.Context, req APKStoreInstallInput) (store.MobileAPK, error) {
	if err := s.mobileAllowed(); err != nil {
		return store.MobileAPK{}, err
	}
	pkg := strings.TrimSpace(req.Package)
	if !apk.ValidPackageName(pkg) {
		return store.MobileAPK{}, apperr.New("mobile_package", "invalid package name")
	}
	proxyID := strings.TrimSpace(req.ProxyID)
	if proxyID != "" {
		if _, err := s.Store.GetProxy(proxyID); err != nil {
			return store.MobileAPK{}, apperr.New("proxy_not_found", "selected proxy does not exist")
		}
	}

	s.emitAPKProgress(pkg, "resolving", 0, 0)
	metaClient, err := s.apkHTTPClient(proxyID, 15*time.Second)
	if err != nil {
		return store.MobileAPK{}, err
	}
	parts, err := s.resolveStoreAPKParts(ctx, metaClient, pkg)
	if err != nil {
		return store.MobileAPK{}, err
	}

	dlClient, err := s.apkHTTPClient(proxyID, 10*time.Minute)
	if err != nil {
		return store.MobileAPK{}, err
	}
	dlDir := filepath.Join(s.apkCacheDir(), apkDownloadsSubd)
	if err := os.MkdirAll(dlDir, 0o700); err != nil {
		return store.MobileAPK{}, err
	}
	tmpDir, err := os.MkdirTemp(dlDir, "store-*")
	if err != nil {
		return store.MobileAPK{}, err
	}
	defer os.RemoveAll(tmpDir)

	var expectedTotal int64
	for _, p := range parts {
		expectedTotal += p.size
	}
	var doneTotal int64
	var lastEmit time.Time
	reportDL := func(partDone, partTotal int64) {
		tot := expectedTotal
		if tot <= 0 && partTotal > 0 {
			tot = partTotal
		}
		now := time.Now()
		if now.Sub(lastEmit) < 150*time.Millisecond && (tot <= 0 || doneTotal+partDone < tot) {
			return
		}
		lastEmit = now
		s.emitAPKProgress(pkg, "downloading", doneTotal+partDone, tot)
	}
	s.emitAPKProgress(pkg, "downloading", 0, expectedTotal)

	var remaining int64 = maxAPKURLBytes
	downloaded := make([]string, 0, len(parts))
	for i, part := range parts {
		dst := filepath.Join(tmpDir, fmt.Sprintf("%02d_%s", i, part.name))
		n, dlErr := downloadAPKPart(ctx, dlClient, part.url, dst, remaining, reportDL)
		if dlErr != nil && part.altURL != "" {
			n, dlErr = downloadAPKPart(ctx, dlClient, part.altURL, dst, remaining, reportDL)
		}
		if dlErr != nil {
			return store.MobileAPK{}, apperr.New("mobile_apk", fmt.Sprintf("download failed for %s: %v", pkg, dlErr))
		}
		remaining -= n
		doneTotal += n
		if expectedTotal > 0 && doneTotal > expectedTotal {
			expectedTotal = doneTotal
		}
		downloaded = append(downloaded, dst)
	}
	s.emitAPKProgress(pkg, "installing", doneTotal, doneTotal)

	pkgPath := downloaded[0]
	if len(downloaded) > 1 {
		pkgPath = filepath.Join(tmpDir, pkg+".apks")
		if err := packSplitAPKs(downloaded, pkgPath); err != nil {
			return store.MobileAPK{}, err
		}
	}

	rec, err := s.addAPK(ctx, APKInput{
		FilePath:         pkgPath,
		Label:            req.Label,
		ProxyID:          proxyID,
		Notes:            req.Notes,
		Orientation:      req.Orientation,
		IdentityTemplate: req.IdentityTemplate,
		CreateShortcut:   req.CreateShortcut,
	}, false)
	if err != nil {
		return store.MobileAPK{}, err
	}
	if req.LaunchAfter {
		s.emitAPKProgress(pkg, "launching", doneTotal, doneTotal)
		if err := s.LaunchAPK(ctx, rec.Package, ""); err != nil {
			return rec, err
		}
		if fresh, ferr := s.Store.GetMobileAPK(rec.Package); ferr == nil {
			rec = fresh
		}
	}
	return rec, nil
}

func (s *Service) resolveStoreAPKParts(ctx context.Context, client *http.Client, pkg string) ([]storeAPKPart, error) {
	if direct, ok := directOfficialAPKURLs[pkg]; ok && isHTTPSURL(direct) {
		return []storeAPKPart{{name: "base.apk", url: direct}}, nil
	}
	if parts, ok := s.fetchAptoideAPKParts(ctx, client, pkg); ok {
		return parts, nil
	}
	if parts, ok := s.fetchFDroidAPKParts(ctx, client, pkg); ok {
		return parts, nil
	}
	return nil, apperr.New("apk_store_fallback", fmt.Sprintf("direct download unavailable for %s; open in Aurora Store instead", pkg))
}

type aptoideMetaResp struct {
	Data struct {
		File struct {
			Path     string `json:"path"`
			PathAlt  string `json:"path_alt"`
			FileSize int64  `json:"filesize"`
			Malware  struct {
				Rank string `json:"rank"`
			} `json:"malware"`
		} `json:"file"`
		AAB *struct {
			Splits []struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Path     string `json:"path"`
				FileSize int64  `json:"filesize"`
			} `json:"splits"`
		} `json:"aab"`
	} `json:"data"`
}

func (s *Service) fetchAptoideAPKParts(ctx context.Context, client *http.Client, pkg string) ([]storeAPKPart, bool) {
	if aptoideIncompleteSplitPkgs[pkg] {
		return nil, false
	}
	abis := s.apkHostABIs()
	hw := base64.StdEncoding.EncodeToString([]byte("myCPU=" + strings.Join(abis, ",") + "&sdk=35&myService=gms"))
	u, err := url.Parse(aptoideMetaBaseURL)
	if err != nil {
		return nil, false
	}
	q := u.Query()
	q.Set("package_name", pkg)
	q.Set("aab", "1")
	q.Set("q", hw)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "AnyProfile/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var parsed aptoideMetaResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&parsed); err != nil {
		return nil, false
	}
	rank := strings.ToUpper(strings.TrimSpace(parsed.Data.File.Malware.Rank))
	if rank == "CRITICAL" || rank == "WARNING" {
		return nil, false
	}
	baseURL := strings.TrimSpace(parsed.Data.File.Path)
	altURL := strings.TrimSpace(parsed.Data.File.PathAlt)
	if !isHTTPSURL(baseURL) {
		if !isHTTPSURL(altURL) {
			return nil, false
		}
		baseURL, altURL = altURL, ""
	}
	if !isHTTPSURL(altURL) {
		altURL = ""
	}
	parts := []storeAPKPart{{name: "base.apk", url: baseURL, altURL: altURL, size: parsed.Data.File.FileSize}}
	if parsed.Data.AAB != nil {
		for i, sp := range parsed.Data.AAB.Splits {
			spURL := strings.TrimSpace(sp.Path)
			if !isHTTPSURL(spURL) {
				continue
			}
			if strings.EqualFold(sp.Type, "ABI") && !splitABIMatches(sp.Name, abis) {
				continue
			}
			safe := sanitizeAPKPartName(sp.Name)
			if safe == "" {
				safe = fmt.Sprintf("split_%d", i)
			}
			parts = append(parts, storeAPKPart{name: safe + ".apk", url: spURL, size: sp.FileSize})
		}
	}
	return parts, true
}

func (s *Service) fetchFDroidAPKParts(ctx context.Context, client *http.Client, pkg string) ([]storeAPKPart, bool) {
	metaURL := strings.TrimRight(fdroidAPIBaseURL, "/") + "/" + url.PathEscape(pkg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "AnyProfile/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var meta struct {
		SuggestedVersionCode int64 `json:"suggestedVersionCode"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta); err != nil || meta.SuggestedVersionCode <= 0 {
		return nil, false
	}
	apkURL := fmt.Sprintf("%s/%s_%d.apk", strings.TrimRight(fdroidRepoBaseURL, "/"), pkg, meta.SuggestedVersionCode)
	if !isHTTPSURL(apkURL) {
		return nil, false
	}
	return []storeAPKPart{{name: "base.apk", url: apkURL}}, true
}

type progressWriter struct {
	w      io.Writer
	done   int64
	total  int64
	report func(done, total int64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.done += int64(n)
	if pw.report != nil && n > 0 {
		pw.report(pw.done, pw.total)
	}
	return n, err
}

func downloadAPKPart(ctx context.Context, client *http.Client, rawURL, dstPath string, limit int64, report func(done, total int64)) (int64, error) {
	if limit < 4 {
		return 0, errors.New("APK download exceeds the 2 GB size limit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "AnyProfile/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var magic [4]byte
	n, err := io.ReadFull(resp.Body, magic[:])
	if err != nil || n < 4 || !isZIPMagic(magic[:]) {
		return 0, errors.New("response is not a valid APK archive")
	}
	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(dstPath)
	}
	if _, err := f.Write(magic[:]); err != nil {
		cleanup()
		return 0, err
	}
	pw := &progressWriter{w: f, done: 4, total: resp.ContentLength, report: report}
	written, err := io.Copy(pw, io.LimitReader(resp.Body, limit-4+1))
	if err != nil {
		cleanup()
		return 0, err
	}
	if written > limit-4 {
		cleanup()
		return 0, errors.New("APK download exceeds the 2 GB size limit")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(dstPath)
		return 0, err
	}
	return written + 4, nil
}

func isHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func splitABIMatches(name string, hostABIs []string) bool {
	norm := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	for _, abi := range hostABIs {
		if strings.Contains(norm, strings.ToLower(strings.ReplaceAll(abi, "-", "_"))) {
			return true
		}
	}
	return false
}

func sanitizeAPKPartName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), ".")
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
	proxyID = strings.TrimSpace(proxyID)
	region := s.proxyRegion(proxyID)
	if q == "" {
		return s.popularAppsForRegion(region), nil
	}

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
	knownIcons := make(map[string]string, len(popularAPKStoreApps))
	for _, a := range popularAPKStoreApps {
		if a.IconURL != "" {
			knownIcons[a.Package] = a.IconURL
		}
	}
	seen := map[string]bool{}
	var out []APKStoreApp
	add := func(app APKStoreApp) {
		app.Package = strings.TrimSpace(app.Package)
		app.Title = strings.TrimSpace(app.Title)
		if app.Package == "" || app.Title == "" || !apk.ValidPackageName(app.Package) || seen[app.Package] {
			return
		}
		if app.IconURL == "" {
			app.IconURL = knownIcons[app.Package]
		}
		if app.IconURL != "" && strings.Contains(app.IconURL, "play-lh.googleusercontent.com") && !strings.Contains(app.IconURL, "=s") && !strings.Contains(app.IconURL, "=w") {
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
