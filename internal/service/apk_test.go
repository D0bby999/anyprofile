// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/store"
)

func TestAPKProGateAndInspect(t *testing.T) {
	s, _ := newMobileService(t, "") // Pro OFF
	ctx := context.Background()

	apkFile := writeMiniAPK(t, t.TempDir(), "com.example.demo", "Demo App", "lib/arm64-v8a/libdemo.so")
	if _, err := s.InspectAPK(ctx, apkFile); err == nil {
		t.Fatal("InspectAPK must be blocked without Pro")
	} else {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "pro_required" {
			t.Fatalf("expected pro_required, got %v", err)
		}
	}
	if _, err := s.AddAPK(ctx, APKInput{FilePath: apkFile}); err == nil {
		t.Fatal("AddAPK must be blocked without Pro")
	}
	if err := s.LaunchAPK(ctx, "com.example.demo", ""); err == nil {
		t.Fatal("LaunchAPK must be blocked without Pro")
	}

	// Lapsed Pro can still list and delete existing records.
	rec := store.MobileAPK{Package: "com.example.old", Label: "Old App"}
	if err := s.Store.UpsertMobileAPK(&rec); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAPKs()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAPKs without Pro = %v, %v", list, err)
	}
	if err := s.DeleteAPK(ctx, "com.example.old", false); err != nil {
		t.Fatalf("DeleteAPK without Pro: %v", err)
	}

	// With Pro enabled: InspectAPK parses offline and checks ABI.
	sPro, _ := newMobileService(t, "active")
	info, err := sPro.InspectAPK(ctx, apkFile)
	if err != nil {
		t.Fatalf("InspectAPK with Pro: %v", err)
	}
	if info.Package != "com.example.demo" || info.Label != "Demo App" || !info.ABICompatible {
		t.Fatalf("unexpected inspect result: %+v", info)
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		badFile := writeMiniAPK(t, t.TempDir(), "com.example.x86", "OldX86", "lib/x86_64/libold.so")
		badInfo, err := sPro.InspectAPK(ctx, badFile)
		if err != nil {
			t.Fatalf("InspectAPK bad ABI: %v", err)
		}
		if badInfo.ABICompatible {
			t.Fatal("expected x86_64-only APK flagged incompatible on darwin/arm64")
		}
	}

	// Dedicated APK runner host is created separately from user Mobile profiles
	// and stays hidden from ListMobileDevices.
	if _, err := sPro.CreateMobileDevice(ctx, MobileInput{Name: "User Phone"}); err != nil {
		t.Fatalf("CreateMobileDevice: %v", err)
	}
	hostID, err := sPro.ensureAPKDevice(ctx, "")
	if err != nil || hostID == "" {
		t.Fatalf("ensureAPKDevice: %q, %v", hostID, err)
	}
	hostID2, err := sPro.ensureAPKDevice(ctx, "")
	if err != nil || hostID2 != hostID {
		t.Fatalf("ensureAPKDevice not idempotent: %q vs %q (%v)", hostID, hostID2, err)
	}
	rows, err := sPro.ListMobileDevices(store.ProfileFilter{})
	if err != nil {
		t.Fatalf("ListMobileDevices: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "User Phone" || rows[0].ID == hostID {
		t.Fatalf("hidden APK runner leaked into Mobile list: %+v", rows)
	}

	// UpdateAPK validates and persists custom name, proxy, orientation, and notes.
	apkRec := store.MobileAPK{Package: "com.example.demo", Label: "Demo App", Orientation: "auto"}
	if err := sPro.Store.UpsertMobileAPK(&apkRec); err != nil {
		t.Fatal(err)
	}
	p := store.Proxy{ID: "px-us", Label: "US Exit", Scheme: "http", Host: "1.2.3.4", Port: 8080}
	if err := sPro.Store.CreateProxy(&p); err != nil {
		t.Fatal(err)
	}
	updated, err := sPro.UpdateAPK(ctx, APKUpdate{
		Package:          "com.example.demo",
		Label:            "Demo US",
		Orientation:      "landscape",
		ProxyID:          p.ID,
		Notes:            "Shop account",
		IdentityTemplate: "pixel8",
	})
	if err != nil {
		t.Fatalf("UpdateAPK: %v", err)
	}
	if updated.Label != "Demo US" || updated.ProxyID != p.ID || updated.Orientation != "landscape" || updated.Notes != "Shop account" {
		t.Fatalf("unexpected updated APK: %+v", updated)
	}
	if updated.IdentityJSON == "" || updated.AndroidID == "" {
		t.Fatalf("expected hardware identity and androidId, got %+v", updated)
	}
	// Rotating identity generates a new IMEI/Serial and Android ID.
	rotated, err := sPro.UpdateAPK(ctx, APKUpdate{
		Package:          "com.example.demo",
		Label:            "Demo US",
		Orientation:      "landscape",
		ProxyID:          p.ID,
		Notes:            "Shop account",
		IdentityTemplate: "regen:pixel8",
	})
	if err != nil {
		t.Fatalf("UpdateAPK regen: %v", err)
	}
	if rotated.IdentityJSON == updated.IdentityJSON || rotated.AndroidID == updated.AndroidID {
		t.Fatalf("expected regen to rotate identity and androidId: before=%+v after=%+v", updated, rotated)
	}
}

func writeMiniAPK(t *testing.T, dir, pkg, label, libEntry string) string {
	t.Helper()
	p := filepath.Join(dir, pkg+".apk")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("AndroidManifest.xml")
	_, _ = w.Write(miniManifestAXML(pkg, label))
	if libEntry != "" {
		lw, _ := zw.Create(libEntry)
		_, _ = lw.Write([]byte("elf"))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

func miniManifestAXML(pkg, label string) []byte {
	const ns = "http://schemas.android.com/apk/res/android"
	pool := []string{ns, "manifest", "package", pkg, "application", "label", label}
	var spData bytes.Buffer
	offs := make([]uint32, len(pool))
	for i, s := range pool {
		offs[i] = uint32(spData.Len())
		spData.WriteByte(byte(len(s)))
		spData.WriteByte(byte(len(s)))
		spData.WriteString(s)
		spData.WriteByte(0)
	}
	for spData.Len()%4 != 0 {
		spData.WriteByte(0)
	}
	var sp bytes.Buffer
	_ = binary.Write(&sp, binary.LittleEndian, uint16(0x0001))
	_ = binary.Write(&sp, binary.LittleEndian, uint16(28))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(28+len(pool)*4+spData.Len()))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(len(pool)))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0x100))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(28+len(pool)*4))
	_ = binary.Write(&sp, binary.LittleEndian, uint32(0))
	for _, o := range offs {
		_ = binary.Write(&sp, binary.LittleEndian, o)
	}
	sp.Write(spData.Bytes())

	var body bytes.Buffer
	startElem := func(nameIdx uint32, attrNS, attrName, attrVal uint32) {
		_ = binary.Write(&body, binary.LittleEndian, uint16(0x0102))
		_ = binary.Write(&body, binary.LittleEndian, uint16(16))
		_ = binary.Write(&body, binary.LittleEndian, uint32(56))
		_ = binary.Write(&body, binary.LittleEndian, uint32(1))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, nameIdx)
		_ = binary.Write(&body, binary.LittleEndian, uint16(20))
		_ = binary.Write(&body, binary.LittleEndian, uint16(20))
		_ = binary.Write(&body, binary.LittleEndian, uint16(1))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, uint16(0))
		_ = binary.Write(&body, binary.LittleEndian, attrNS)
		_ = binary.Write(&body, binary.LittleEndian, attrName)
		_ = binary.Write(&body, binary.LittleEndian, attrVal)
		_ = binary.Write(&body, binary.LittleEndian, uint16(8))
		body.WriteByte(0)
		body.WriteByte(0x03)
		_ = binary.Write(&body, binary.LittleEndian, attrVal)
	}
	endElem := func(nameIdx uint32) {
		_ = binary.Write(&body, binary.LittleEndian, uint16(0x0103))
		_ = binary.Write(&body, binary.LittleEndian, uint16(16))
		_ = binary.Write(&body, binary.LittleEndian, uint32(24))
		_ = binary.Write(&body, binary.LittleEndian, uint32(1))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, uint32(0xffffffff))
		_ = binary.Write(&body, binary.LittleEndian, nameIdx)
	}
	startElem(1, 0xffffffff, 2, 3)
	startElem(4, 0, 5, 6)
	endElem(4)
	endElem(1)

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x0003))
	_ = binary.Write(&out, binary.LittleEndian, uint16(8))
	_ = binary.Write(&out, binary.LittleEndian, uint32(8+sp.Len()+body.Len()))
	out.Write(sp.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}

func TestAPKStoreHelpersAndDownload(t *testing.T) {
	// 1. Deep-link command generation
	if cmd := storeDeepLinkCmd(""); cmd != "" {
		t.Fatalf("empty query cmd = %q", cmd)
	}
	if cmd := storeDeepLinkCmd("com.zhiliaoapp.musically"); !strings.Contains(cmd, "market://details?id=com.zhiliaoapp.musically") || !strings.Contains(cmd, "-p com.aurora.store") {
		t.Fatalf("package deep-link = %q", cmd)
	}
	if cmd := storeDeepLinkCmd("TikTok 'US'; rm -rf /"); !strings.Contains(cmd, "market://search?q=") || strings.Contains(cmd, "; rm") {
		t.Fatalf("search deep-link not escaped: %q", cmd)
	}

	// 2. Third-party package filter excludes Aurora, Play Store, known catalog entries, and invalid names
	raw := "package:com.aurora.store\npackage:com.android.vending\npackage:com.zhiliaoapp.musically\npackage:com.instagram.android\npackage:bad pkg!\n"
	got := parseThirdPartyPackages(raw, map[string]bool{"com.instagram.android": true})
	if len(got) != 1 || got[0] != "com.zhiliaoapp.musically" {
		t.Fatalf("parseThirdPartyPackages = %v, want [com.zhiliaoapp.musically]", got)
	}

	// 3. Split APK packing produces an .apks bundle readable by InspectAPK
	tmp := t.TempDir()
	baseAPK := writeMiniAPK(t, tmp, "com.example.split", "Split App", "lib/arm64-v8a/liba.so")
	cfgAPK := filepath.Join(tmp, "config.arm64_v8a.apk")
	if err := os.WriteFile(cfgAPK, []byte("PK\x03\x04"), 0o600); err != nil {
		t.Fatal(err)
	}
	packed := filepath.Join(tmp, "com.example.split.apks")
	if err := packSplitAPKs([]string{baseAPK, cfgAPK}, packed); err != nil {
		t.Fatalf("packSplitAPKs: %v", err)
	}

	sPro, _ := newMobileService(t, "active")
	ctx := context.Background()
	info, err := sPro.InspectAPK(ctx, packed)
	if err != nil {
		t.Fatalf("InspectAPK(packed .apks): %v", err)
	}
	if info.Package != "com.example.split" || info.Label != "Split App" || !info.IsSplitBundle {
		t.Fatalf("unexpected packed split inspect: %+v", info)
	}

	// 4. Pro gate on store/import/download methods
	sFree, _ := newMobileService(t, "")
	if err := sFree.OpenAPKStore(ctx, APKStoreRequest{Query: "TikTok"}); err == nil {
		t.Fatal("OpenAPKStore must require Pro")
	}
	if _, err := sFree.ListUncatalogedAPKs(ctx, ""); err == nil {
		t.Fatal("ListUncatalogedAPKs must require Pro")
	}
	if _, err := sFree.DownloadAPKFromURL(ctx, "https://example.com/app.apk", ""); err == nil {
		t.Fatal("DownloadAPKFromURL must require Pro")
	}

	// 5. DownloadAPKFromURL validates HTTPS, ZIP magic, and inspects valid APK
	apkBytes, err := os.ReadFile(baseAPK)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.apk":
			_, _ = w.Write(apkBytes)
		case "/html.apk":
			_, _ = w.Write([]byte("<html>not an apk</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	prevTransport := apkHTTPTransport
	apkHTTPTransport = func() *http.Transport {
		return srv.Client().Transport.(*http.Transport).Clone()
	}
	defer func() { apkHTTPTransport = prevTransport }()

	if _, err := sPro.DownloadAPKFromURL(ctx, "http://example.com/a.apk", ""); err == nil {
		t.Fatal("expected http:// URL to be rejected")
	}
	if _, err := sPro.DownloadAPKFromURL(ctx, srv.URL+"/html.apk", ""); err == nil {
		t.Fatal("expected HTML response to be rejected")
	}
	dlInfo, err := sPro.DownloadAPKFromURL(ctx, srv.URL+"/ok.apk", "")
	if err != nil {
		t.Fatalf("DownloadAPKFromURL valid apk: %v", err)
	}
	if dlInfo.Package != "com.example.split" || dlInfo.Label != "Split App" {
		t.Fatalf("unexpected DownloadAPKFromURL inspect: %+v", dlInfo)
	}
	if _, err := os.Stat(dlInfo.FilePath); err != nil {
		t.Fatalf("expected downloaded temp file to exist: %v", err)
	}
}

func TestSearchAPKStore(t *testing.T) {
	s, _ := newMobileService(t, "")
	ctx := context.Background()
	if _, err := s.SearchAPKStore(ctx, "tiktok", ""); err == nil {
		t.Fatal("SearchAPKStore must be Pro-gated")
	}

	sPro, _ := newMobileService(t, "active")

	// Empty query returns popular catalog with real icon URLs and zero network traffic.
	pop, err := sPro.SearchAPKStore(ctx, "", "")
	if err != nil || len(pop) < 8 {
		t.Fatalf("empty SearchAPKStore = %d, %v", len(pop), err)
	}
	for _, a := range pop {
		if !strings.HasPrefix(a.IconURL, "https://play-lh.googleusercontent.com/") {
			t.Fatalf("popular app %s missing verified icon URL: %q", a.Package, a.IconURL)
		}
	}
	if jp := sPro.popularAppsForRegion("JP"); len(jp) < 2 || jp[1].Package != "jp.naver.line.android" {
		t.Fatalf("expected LINE near top for JP popular apps, got %+v", jp[:2])
	}
	if vn := sPro.popularAppsForRegion("VN"); len(vn) < 4 || vn[2].Package != "com.zing.zalo" {
		t.Fatalf("expected Zalo near top for VN popular apps, got %+v", vn[:4])
	}

	// Build a minimal Play Store ds:4 HTML payload with a list item.
	item := make([]any, 16)
	item[0] = []any{"com.shopee.vn", 7}
	item[1] = []any{nil, 2, []any{512, 512}, []any{nil, nil, "https://play-lh.googleusercontent.com/abc123"}}
	item[3] = "Shopee VN"
	item[4] = []any{"4.6", 4.6}
	item[5] = "Shopping"
	item[14] = "Shopee"
	sec := make([]any, 24)
	sec[22] = []any{[]any{[]any{item}}}
	payload := []any{[]any{nil, []any{sec}}}
	raw, _ := json.Marshal(payload)
	html := `<script>AF_initDataCallback({key: 'ds:4', hash: '9', data:` + string(raw) + `, sideChannel: {}});</script>`

	var seenGL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenGL = r.URL.Query().Get("gl")
		if r.URL.Query().Get("q") == "offline-test" {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	prevURL := apkPlaySearchURL
	apkPlaySearchURL = srv.URL
	apkSearchCacheMu.Lock()
	apkSearchCache = map[string]apkSearchCacheEntry{}
	apkSearchCacheMu.Unlock()
	defer func() {
		apkPlaySearchURL = prevURL
		apkSearchCacheMu.Lock()
		apkSearchCache = map[string]apkSearchCacheEntry{}
		apkSearchCacheMu.Unlock()
	}()

	got, err := sPro.SearchAPKStore(ctx, "shopee", "")
	if err != nil || len(got) != 1 {
		t.Fatalf("SearchAPKStore live = %+v, %v", got, err)
	}
	if got[0].Package != "com.shopee.vn" || got[0].Title != "Shopee VN" || got[0].Developer != "Shopee" || !strings.HasSuffix(got[0].IconURL, "=s96") {
		t.Fatalf("unexpected parsed app: %+v", got[0])
	}
	if seenGL != "US" {
		t.Fatalf("expected default gl=US, got %q", seenGL)
	}

	// Region override from proxy country sends gl=<country> to Google Play.
	if gotRegion := sPro.fetchPlayStoreApps(ctx, "shopee", "", "DE"); len(gotRegion) != 1 || seenGL != "DE" {
		t.Fatalf("regional fetch = %d apps, gl=%q; want 1 app and gl=DE", len(gotRegion), seenGL)
	}
	if err := sPro.Store.CreateProxy(&store.Proxy{ID: "px-de", Label: "DE Exit", Scheme: "socks5", Host: "127.0.0.1", Port: 1080}); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}
	if err := sPro.Store.SaveProxyCheck(store.Proxy{ID: "px-de", ExitIP: "5.6.7.8", Country: "de"}); err != nil {
		t.Fatalf("SaveProxyCheck: %v", err)
	}
	if cc := sPro.proxyRegion("px-de"); cc != "DE" {
		t.Fatalf("proxyRegion = %q, want DE", cc)
	}

	// Network error falls back cleanly without erroring.
	fb, err := sPro.SearchAPKStore(ctx, "offline-test", "")
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if len(fb) != 0 {
		t.Fatalf("expected 0 fallback matches for unknown query, got %+v", fb)
	}
}

func TestDeleteAPKQueuesPendingUninstallWhenStopped(t *testing.T) {
	s, _ := newMobileService(t, "active")
	ctx := context.Background()
	rec := store.MobileAPK{Package: "org.fdroid.fdroid", Label: "F-Droid", DeviceID: "stopped-host"}
	if err := s.Store.UpsertMobileAPK(&rec); err != nil {
		t.Fatalf("UpsertMobileAPK: %v", err)
	}
	if err := s.DeleteAPK(ctx, "org.fdroid.fdroid", true); err != nil {
		t.Fatalf("DeleteAPK: %v", err)
	}
	pending := s.pendingAPKUninstalls()
	if len(pending) != 1 || pending[0] != "org.fdroid.fdroid" {
		t.Fatalf("pendingAPKUninstalls = %v, want [org.fdroid.fdroid]", pending)
	}
	// Re-adding or removing clears it cleanly.
	s.removePendingAPKUninstall("org.fdroid.fdroid")
	if got := s.pendingAPKUninstalls(); len(got) != 0 {
		t.Fatalf("expected pending list cleared, got %v", got)
	}
}

func TestResolveStoreAPKPartsAndFallback(t *testing.T) {
	s, _ := newMobileService(t, "active")
	ctx := context.Background()

	var srvURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/aptoide":
			pkg := r.URL.Query().Get("package_name")
			switch pkg {
			case "com.example.split":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"file": map[string]any{
							"path":    srvURL + "/dl/base.apk",
							"malware": map[string]any{"rank": "TRUSTED"},
						},
						"aab": map[string]any{
							"splits": []map[string]any{
								{"name": "config.arm64_v8a", "type": "ABI", "path": srvURL + "/dl/arm64.apk"},
								{"name": "config.armeabi_v7a", "type": "ABI", "path": srvURL + "/dl/arm32.apk"},
								{"name": "config.xhdpi", "type": "DENSITY", "path": srvURL + "/dl/xhdpi.apk"},
							},
						},
					},
				})
			case "com.example.bad":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"file": map[string]any{
							"path":    srvURL + "/dl/bad.apk",
							"malware": map[string]any{"rank": "CRITICAL"},
						},
					},
				})
			default:
				http.NotFound(w, r)
			}
		case r.URL.Path == "/fdroid/api/org.example.foss":
			_ = json.NewEncoder(w).Encode(map[string]any{"suggestedVersionCode": 42})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	prevAptoide, prevFDAPI, prevFDRepo := aptoideMetaBaseURL, fdroidAPIBaseURL, fdroidRepoBaseURL
	aptoideMetaBaseURL = srv.URL + "/aptoide"
	fdroidAPIBaseURL = srv.URL + "/fdroid/api"
	fdroidRepoBaseURL = srv.URL + "/fdroid/repo"
	defer func() {
		aptoideMetaBaseURL, fdroidAPIBaseURL, fdroidRepoBaseURL = prevAptoide, prevFDAPI, prevFDRepo
	}()

	client := srv.Client()

	// 1. Aptoide split app resolves base + matching ABI + density (drops incompatible arm32 on arm64 host).
	parts, err := s.resolveStoreAPKParts(ctx, client, "com.example.split")
	if err != nil {
		t.Fatalf("resolveStoreAPKParts(split): %v", err)
	}
	if len(parts) < 2 || parts[0].url != srvURL+"/dl/base.apk" {
		t.Fatalf("unexpected parts: %+v", parts)
	}
	foundArm64 := false
	for _, p := range parts {
		if strings.Contains(p.url, "arm64.apk") {
			foundArm64 = true
		}
	}
	if !foundArm64 {
		t.Fatalf("expected arm64 split in %+v", parts)
	}

	// 2. Malware-flagged Aptoide entry is rejected and falls back to apk_store_fallback error.
	if _, err := s.resolveStoreAPKParts(ctx, client, "com.example.bad"); err == nil || !strings.Contains(err.Error(), "direct download unavailable") {
		t.Fatalf("expected malware entry to be rejected, got %v", err)
	}

	// 3. F-Droid fallback resolves open-source app when Aptoide returns 404.
	fdParts, err := s.resolveStoreAPKParts(ctx, client, "org.example.foss")
	if err != nil || len(fdParts) != 1 || fdParts[0].url != srvURL+"/fdroid/repo/org.example.foss_42.apk" {
		t.Fatalf("F-Droid fallback = %+v, %v", fdParts, err)
	}
}
