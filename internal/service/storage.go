// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/secrets"
	"github.com/D0bby999/anyprofile/internal/store"
)

// Orphan is a profile folder with no profile row: left by a crash or a reset database.
type Orphan struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
}

type StorageInfo struct {
	ProfilesBytes  int64 `json:"profilesBytes"`
	SnapshotsBytes int64 `json:"snapshotsBytes"`
	// MobileBytes is the mobile devices and their backups; LDPlayer keeps its
	// own devices in its own folder, which is not counted.
	MobileBytes        int64    `json:"mobileBytes"`
	MobileRuntimeBytes int64    `json:"mobileRuntimeBytes"`
	EnginesBytes       int64    `json:"enginesBytes"`
	APKCacheBytes      int64    `json:"apkCacheBytes"`
	UploadsBytes       int64    `json:"uploadsBytes"`
	LogsBytes          int64    `json:"logsBytes"`
	TotalBytes         int64    `json:"totalBytes"`
	Orphans            []Orphan `json:"orphans"`
}

// cacheDirs are Chromium caches, safe to delete while the browser is closed.
var cacheDirs = []string{"Default/Cache", "Default/Code Cache", "Default/GPUCache",
	"GrShaderCache", "ShaderCache", "GraphiteDawnCache", "GPUPersistentCache",
	"Default/DawnGraphiteCache", "Default/DawnWebGPUCache",
	"Default/Service Worker/CacheStorage", "Default/Service Worker/ScriptCache",
	"Default/Shared Dictionary"}

// componentDirs are what Chromium's component updater downloads into every
// profile (speech, suggestions, certificate and safety lists, Widevine). It
// fetches each again when missing, through the profile's proxy.
var componentDirs = []string{"ActorSafetyLists", "AmountExtractionHeuristicRegexes",
	"CaptchaProviders", "CertificateRevocation", "Crowd Deny", "FileTypePolicies",
	"FirstPartySetsPreloaded", "MEIPreload", "OnDeviceHeadSuggestModel", "OptimizationHints",
	"OptimizationGuidePredictionModels", "OriginTrials", "PKIMetadata",
	"PrivacySandboxAttestationsPreloaded", "SSLErrorAssistant", "Safe Browsing", "SafetyTips",
	"Subresource Filter", "TrustTokenKeyCommitments", "WasmTtsEngine", "WidevineCdm", "ZxcvbnData"}

// downloadCaches keep a copy of every component and extension download, for
// updates; the installed copies are elsewhere. ~60 MB a profile.
var downloadCaches = []string{"component_crx_cache", "extensions_crx_cache"}

// dirSize sums the size of the regular files under path; errors are ignored
// (a vanished file mid-walk is simply not counted).
func dirSize(path string) int64 {
	var total int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil && d.Type().IsRegular() {
			total += fileDiskBytes(info)
		}
		return nil
	})
	return total
}

// knownIDs is every profile row, live and trashed.
func (s *Service) knownIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	for _, f := range []store.ProfileFilter{{}, {Trashed: true}} {
		ps, err := s.Store.ListProfiles(f)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			ids[p.ID] = true
		}
	}
	return ids, nil
}

// orphanIDs lists the folders in the profiles directory that no profile row owns.
func (s *Service) orphanIDs() ([]string, error) {
	known, err := s.knownIDs()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dirs.Profiles())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		// ".restore-x" and ".old-x" are a restore in progress, not orphans.
		if e.IsDir() && !known[e.Name()] && !strings.HasPrefix(e.Name(), ".") {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (s *Service) running() map[string]bool {
	m := map[string]bool{}
	for _, in := range s.Launcher.List() {
		m[in.ProfileID] = true
	}
	return m
}

// Storage measures every major data directory and finds orphan profile folders.
func (s *Service) Storage() (StorageInfo, error) {
	ids, err := s.orphanIDs()
	if err != nil {
		return StorageInfo{}, err
	}
	info := StorageInfo{
		Orphans:            []Orphan{},
		ProfilesBytes:      dirSize(s.Dirs.Profiles()),
		SnapshotsBytes:     dirSize(s.Dirs.Snapshots()),
		MobileBytes:        dirSize(s.Dirs.MobileDevices()) + dirSize(s.Dirs.MobileBackups()),
		MobileRuntimeBytes: dirSize(filepath.Join(s.Dirs.Mobile(), "avo")) + dirSize(s.Dirs.MobileRuntime()),
		EnginesBytes:       dirSize(s.Dirs.Engines()),
		APKCacheBytes:      s.apkInstallerCacheBytes(),
		UploadsBytes:       dirSize(s.Dirs.Uploads()),
		LogsBytes:          dirSize(s.Dirs.Logs()),
	}
	info.TotalBytes = info.ProfilesBytes + info.SnapshotsBytes + info.MobileBytes +
		info.MobileRuntimeBytes + info.EnginesBytes + info.APKCacheBytes + info.UploadsBytes + info.LogsBytes
	for _, id := range ids {
		info.Orphans = append(info.Orphans, Orphan{ID: id, Bytes: dirSize(s.Dirs.Profile(id))})
	}
	s.Log.Info("storage measured", "profilesBytes", info.ProfilesBytes, "totalBytes", info.TotalBytes, "orphans", len(info.Orphans))
	return info, nil
}

// DeleteOrphans removes the listed folders and their Keychain items. Only ids
// a fresh scan reports as orphans are touched, so no id can reach the
// profiles root, a live profile or anything outside it.
func (s *Service) DeleteOrphans(ids []string) error {
	scanned, err := s.orphanIDs()
	if err != nil {
		return err
	}
	orphan := map[string]bool{}
	for _, id := range scanned {
		orphan[id] = true
	}
	run := s.running()
	var siteIDs []string
	if all, err := s.Sites(); err == nil {
		for _, x := range all {
			siteIDs = append(siteIDs, x.ID)
		}
	}
	deleted := 0
	var bytes int64
	for _, id := range ids {
		if !orphan[id] || run[id] {
			continue
		}
		p := s.Dirs.Profile(id)
		bytes += dirSize(p)
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		// The rows are gone, so every platform's key is tried; deleting a
		// missing item is not an error.
		s.Secrets.Delete(secrets.ServiceCookieKey, id)
		s.Secrets.Delete(serviceChromiumSafeStorage, id)
		s.deleteAccountSecrets(id)
		for _, site := range siteIDs {
			s.deleteAccountSecrets(id + "/" + site)
		}
		deleted++
	}
	s.Log.Info("orphans deleted", "count", deleted, "bytes", bytes)
	return nil
}

// ClearCache frees the Chromium caches of every profile that is not running,
// cached APK installer bundles (keeping icons), orphan snapshot files, and
// stale trace/cookie-backup logs. Cookies, logins and uploads are kept.
func (s *Service) ClearCache() (int64, error) {
	known, err := s.knownIDs()
	if err != nil {
		return 0, err
	}
	run := s.running()
	var total int64
	cleared := 0
	for id := range known {
		if run[id] {
			continue
		}
		for _, d := range append(cacheDirs, downloadCaches...) {
			p := filepath.Join(s.Dirs.Profile(id), d)
			total += dirSize(p)
			if err := os.RemoveAll(p); err != nil {
				return total, err
			}
		}
		cleared++
	}
	total += s.clearAPKInstallerCache()
	total += s.PruneSnapshots()
	total += s.PruneLogs(7 * 24 * time.Hour)
	s.Log.Info("cache cleared", "profiles", cleared, "bytes", total)
	return total, nil
}

// ClearUploads deletes every staged file or pack inside the uploads directory.
func (s *Service) ClearUploads() (int64, error) {
	root := s.Dirs.Uploads()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var freed int64
	for _, e := range entries {
		p := filepath.Join(root, e.Name())
		if e.IsDir() {
			freed += dirSize(p)
		} else if info, err := e.Info(); err == nil {
			freed += fileDiskBytes(info)
		}
		if err := os.RemoveAll(p); err != nil {
			return freed, err
		}
	}
	s.Log.Info("uploads cleared", "bytes", freed)
	s.notify("settings")
	return freed, nil
}

func isAPKInstallerFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".apk", ".apks", ".xapk", ".apkm":
		return true
	default:
		return false
	}
}

func (s *Service) apkInstallerCacheBytes() int64 {
	dir := s.apkCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		if e.Name() == "icons" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if e.IsDir() && e.Name() == apkDownloadsSubd {
			total += dirSize(p)
		} else if !e.IsDir() && isAPKInstallerFile(e.Name()) {
			if info, err := e.Info(); err == nil {
				total += fileDiskBytes(info)
			}
		}
	}
	return total
}

func (s *Service) clearAPKInstallerCache() int64 {
	dir := s.apkCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var freed int64
	for _, e := range entries {
		if e.Name() == "icons" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if e.IsDir() && e.Name() == apkDownloadsSubd {
			freed += dirSize(p)
			_ = os.RemoveAll(p)
		} else if !e.IsDir() && isAPKInstallerFile(e.Name()) {
			if info, err := e.Info(); err == nil {
				freed += fileDiskBytes(info)
			}
			_ = os.Remove(p)
		}
	}
	return freed
}

const maxKeptTraces = 100

// PruneLogs removes stale API trace files and unreferenced cookie-jar backups
// older than maxAge. Active cookie-backup state files and their referenced jars
// are never deleted.
func (s *Service) PruneLogs(maxAge time.Duration) int64 {
	cutoff := time.Now().Add(-maxAge)
	var freed int64

	// 1. Prune logs/traces by age and total count cap.
	tracesDir := filepath.Join(s.Dirs.Logs(), "traces")
	if entries, err := os.ReadDir(tracesDir); err == nil {
		type traceFile struct {
			path string
			mod  time.Time
			size int64
		}
		var kept []traceFile
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			p := filepath.Join(tracesDir, e.Name())
			sz := fileDiskBytes(info)
			if info.ModTime().Before(cutoff) {
				freed += sz
				_ = os.Remove(p)
				continue
			}
			kept = append(kept, traceFile{path: p, mod: info.ModTime(), size: sz})
		}
		if len(kept) > maxKeptTraces {
			sort.Slice(kept, func(i, j int) bool { return kept[i].mod.After(kept[j].mod) })
			for _, old := range kept[maxKeptTraces:] {
				freed += old.size
				_ = os.Remove(old.path)
			}
		}
	}

	// 2. Prune stale, unreferenced files in logs/cookie-backups.
	cbDir := filepath.Join(s.Dirs.Logs(), "cookie-backups")
	if entries, err := os.ReadDir(cbDir); err == nil {
		active := map[string]bool{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(cbDir, e.Name()))
			if err != nil {
				continue
			}
			var st struct {
				Backup string `json:"backup"`
			}
			if json.Unmarshal(b, &st) == nil && st.Backup != "" {
				active[st.Backup] = true
			}
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") || active[e.Name()] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				freed += fileDiskBytes(info)
				_ = os.Remove(filepath.Join(cbDir, e.Name()))
			}
		}
	}

	if freed > 0 {
		s.Log.Info("stale logs pruned", "bytes", freed)
	}
	return freed
}
