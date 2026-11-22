// SPDX-License-Identifier: MIT

package apk

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Orientation describes an app's declared display orientation.
type Orientation string

const (
	OrientationAuto      Orientation = "auto"
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
)

// Metadata holds everything parsed offline from an .apk or split bundle.
type Metadata struct {
	FilePath       string      `json:"filePath"`
	SizeBytes      int64       `json:"sizeBytes"`
	Package        string      `json:"package"`
	Label          string      `json:"label"`
	VersionName    string      `json:"versionName"`
	VersionCode    int32       `json:"versionCode"`
	MinSDK         int32       `json:"minSdk"`
	TargetSDK      int32       `json:"targetSdk"`
	LaunchActivity string      `json:"launchActivity"`
	Orientation    Orientation `json:"orientation"`
	ABIs           []string    `json:"abis"`
	IsSplitBundle  bool        `json:"isSplitBundle"`
	IconBytes      []byte      `json:"-"`
	IconExt        string      `json:"iconExt,omitempty"`
}

var (
	validPkgRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$`)
	validCmpRe = regexp.MustCompile(`^[A-Za-z0-9_.$]+$`)
)

// ValidPackageName enforces safe Android application IDs before path/shell use.
func ValidPackageName(pkg string) bool {
	return len(pkg) <= 180 && validPkgRe.MatchString(pkg)
}

// ValidComponent enforces safe activity class names before am start.
func ValidComponent(cmp string) bool {
	return len(cmp) > 0 && len(cmp) <= 256 && validCmpRe.MatchString(cmp)
}

// IsSupportedPackage reports whether path has an .apk/.apks/.xapk/.apkm extension.
func IsSupportedPackage(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".apk", ".apks", ".xapk", ".apkm":
		return true
	}
	return false
}

// Parse inspects an .apk or split bundle (.xapk, .apks, .apkm) completely offline.
func Parse(filePath string) (*Metadata, error) {
	if !IsSupportedPackage(filePath) {
		return nil, errors.New("only .apk, .apks, .xapk, and .apkm files are supported")
	}
	st, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if st.Size() < 128 {
		return nil, errors.New("file is too small to be a valid Android package")
	}
	if strings.EqualFold(filepath.Ext(filePath), ".apk") {
		meta, err := parseSingleAPK(filePath)
		if err != nil {
			return nil, err
		}
		meta.SizeBytes = st.Size()
		return meta, nil
	}
	return parseSplitBundle(filePath, st.Size())
}

func parseSingleAPK(apkPath string) (*Metadata, error) {
	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open apk zip: %w", err)
	}
	defer zr.Close()

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		entries[f.Name] = f
	}
	manifestBytes, err := readZipEntry(entries["AndroidManifest.xml"], 16<<20)
	if err != nil {
		return nil, errors.New("missing or unreadable AndroidManifest.xml")
	}
	manifest, err := ParseAXML(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("parse AndroidManifest.xml: %w", err)
	}

	pkgAttr := manifest.Attr("package", "")
	if pkgAttr == nil || !ValidPackageName(pkgAttr.StrValue) {
		return nil, errors.New("invalid or missing package name in AndroidManifest.xml")
	}
	pkg := pkgAttr.StrValue

	versionName := "1.0"
	if a := manifest.AndroidAttr("versionName"); a != nil && a.StrValue != "" {
		versionName = a.StrValue
	}
	var versionCode int32
	if a := manifest.AndroidAttr("versionCode"); a != nil && a.HasInt {
		versionCode = a.IntValue
	}
	var minSDK, targetSDK int32
	if sdks := manifest.ChildNodes("uses-sdk"); len(sdks) > 0 {
		if a := sdks[0].AndroidAttr("minSdkVersion"); a != nil && a.HasInt {
			minSDK = a.IntValue
		}
		if a := sdks[0].AndroidAttr("targetSdkVersion"); a != nil && a.HasInt {
			targetSDK = a.IntValue
		}
	}

	var res *ResTable
	if rf := entries["resources.arsc"]; rf != nil {
		if rb, err := readZipEntry(rf, 64<<20); err == nil {
			res, _ = ParseARSC(rb)
		}
	}

	appNode := &XMLNode{}
	if apps := manifest.ChildNodes("application"); len(apps) > 0 {
		appNode = apps[0]
	}

	label := ""
	if a := appNode.AndroidAttr("label"); a != nil {
		if a.RefID != 0 && res != nil {
			label = res.ResolveString(a.RefID, "string")
		} else {
			label = a.StrValue
		}
	}
	label = strings.Join(strings.Fields(label), " ")
	if label == "" {
		parts := strings.Split(pkg, ".")
		last := parts[len(parts)-1]
		if len(last) > 0 {
			label = strings.ToUpper(last[:1]) + last[1:]
		} else {
			label = pkg
		}
	}
	if len([]rune(label)) > 60 {
		label = string([]rune(label)[:60])
	}

	acts := append(appNode.ChildNodes("activity"), appNode.ChildNodes("activity-alias")...)
	var launcher *XMLNode
	for _, a := range acts {
		if isLauncherActivity(a) {
			launcher = a
			break
		}
	}
	launchActivity := resolveLaunchActivity(pkg, launcher)
	orientation := DetectOrientation(acts, launcher)

	iconBytes, iconExt := extractIcon(entries, appNode, res)
	abis := scanZipABIs(zr.File)

	return &Metadata{
		FilePath:       apkPath,
		Package:        pkg,
		Label:          label,
		VersionName:    versionName,
		VersionCode:    versionCode,
		MinSDK:         minSDK,
		TargetSDK:      targetSDK,
		LaunchActivity: launchActivity,
		Orientation:    orientation,
		ABIs:           abis,
		IconBytes:      iconBytes,
		IconExt:        iconExt,
	}, nil
}

func parseSplitBundle(bundlePath string, sizeBytes int64) (*Metadata, error) {
	tmp, err := os.MkdirTemp("", "anyapk-inspect-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	splits, err := ExtractSplitAPKs(bundlePath, tmp)
	if err != nil {
		return nil, err
	}

	var base *Metadata
	abiSet := map[string]struct{}{}
	for _, sp := range splits {
		if zr, err := zip.OpenReader(sp); err == nil {
			for _, a := range scanZipABIs(zr.File) {
				abiSet[a] = struct{}{}
			}
			_ = zr.Close()
		}
		m, err := parseSingleAPK(sp)
		if err != nil {
			continue
		}
		// Prefer the split that declares a launcher activity (base.apk).
		if base == nil || (base.LaunchActivity == "" && m.LaunchActivity != "") || (len(base.IconBytes) == 0 && len(m.IconBytes) > 0) {
			base = m
		}
	}
	if base == nil {
		return nil, errors.New("bundle contains no parseable base APK")
	}
	abis := make([]string, 0, len(abiSet))
	for a := range abiSet {
		abis = append(abis, a)
	}
	sort.Strings(abis)
	base.FilePath = bundlePath
	base.SizeBytes = sizeBytes
	base.IsSplitBundle = true
	base.ABIs = abis
	return base, nil
}

// ExtractSplitAPKs safely unpacks .apk entries from an .apks/.xapk/.apkm bundle.
// Enforces a 64-file cap, 2 GiB total byte limit, and rejects .obb bundles.
func ExtractSplitAPKs(bundlePath, dir string) ([]string, error) {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, errors.New("corrupted or encrypted bundle; use an unencrypted .apk, .apks, .xapk or .apkm file")
	}
	defer zr.Close()

	const maxSplits = 64
	var rem int64 = 2 << 30
	var out []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") || strings.HasPrefix(filepath.Base(f.Name), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext == ".obb" {
			return nil, errors.New("bundles with .obb data files are not supported")
		}
		if ext != ".apk" {
			continue
		}
		if len(out) >= maxSplits {
			return nil, errors.New("bundle has too many split APKs")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(dir, fmt.Sprintf("split-%02d.apk", len(out)))
		wf, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		n, copyErr := io.Copy(wf, io.LimitReader(rc, rem+1))
		_ = rc.Close()
		closeErr := wf.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		rem -= n
		if rem < 0 {
			return nil, errors.New("bundle exceeds 2 GB uncompressed limit")
		}
		out = append(out, dst)
	}
	if len(out) == 0 {
		return nil, errors.New("bundle contains no .apk files")
	}
	return out, nil
}

// CheckABICompatible returns nil if apkABIs is empty (universal) or overlaps hostABIs.
func CheckABICompatible(apkABIs, hostABIs []string) error {
	if len(apkABIs) == 0 || len(hostABIs) == 0 {
		return nil
	}
	for _, a := range apkABIs {
		if slices.Contains(hostABIs, a) {
			return nil
		}
	}
	return fmt.Errorf("incompatible ABI: package only includes %s (engine supports %s)",
		strings.Join(apkABIs, ", "), strings.Join(hostABIs, ", "))
}

// DetectOrientation computes whether an app declares a fixed portrait or
// landscape layout, weighting the MAIN+LAUNCHER activity 4x.
func DetectOrientation(acts []*XMLNode, launcher *XMLNode) Orientation {
	var portrait, landscape int
	for _, a := range acts {
		attr := a.AndroidAttr("screenOrientation")
		if attr == nil || !attr.HasInt {
			continue
		}
		w := 1
		if a == launcher {
			w = 4
		}
		switch attr.IntValue {
		case 1, 5, 7, 13:
			portrait += w
		case 2, 4, 6, 12:
			landscape += w
		}
	}
	if portrait == 0 && landscape == 0 || portrait == landscape {
		return OrientationAuto
	}
	if portrait > landscape {
		return OrientationPortrait
	}
	return OrientationLandscape
}

func isLauncherActivity(node *XMLNode) bool {
	for _, f := range node.ChildNodes("intent-filter") {
		hasMain, hasLauncher := false, false
		for _, a := range f.ChildNodes("action") {
			if attr := a.AndroidAttr("name"); attr != nil && attr.StrValue == "android.intent.action.MAIN" {
				hasMain = true
			}
		}
		for _, c := range f.ChildNodes("category") {
			if attr := c.AndroidAttr("name"); attr != nil && attr.StrValue == "android.intent.category.LAUNCHER" {
				hasLauncher = true
			}
		}
		if hasMain && hasLauncher {
			return true
		}
	}
	return false
}

func resolveLaunchActivity(pkg string, launcher *XMLNode) string {
	if launcher == nil {
		return ""
	}
	n := ""
	if a := launcher.AndroidAttr("name"); a != nil {
		n = a.StrValue
	}
	if launcher.Name == "activity-alias" {
		if t := launcher.AndroidAttr("targetActivity"); t != nil && t.StrValue != "" {
			n = t.StrValue
		}
	}
	if n == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(n, "."):
		n = pkg + n
	case !strings.Contains(n, "."):
		n = pkg + "." + n
	}
	if !ValidComponent(n) {
		return ""
	}
	return n
}

func scanZipABIs(files []*zip.File) []string {
	seen := map[string]struct{}{}
	for _, f := range files {
		parts := strings.Split(f.Name, "/")
		if len(parts) >= 3 && parts[0] == "lib" && parts[1] != "" && strings.HasSuffix(strings.ToLower(f.Name), ".so") {
			seen[parts[1]] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func readZipEntry(f *zip.File, maxBytes int64) ([]byte, error) {
	if f == nil {
		return nil, os.ErrNotExist
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxBytes))
}

func isBitmapExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".webp", ".jpg", ".jpeg":
		return true
	}
	return false
}

func extractIcon(entries map[string]*zip.File, app *XMLNode, res *ResTable) ([]byte, string) {
	iconAttr := app.AndroidAttr("icon")
	if iconAttr == nil {
		iconAttr = app.AndroidAttr("roundIcon")
	}
	if iconAttr != nil {
		file := iconAttr.StrValue
		if iconAttr.RefID != 0 && res != nil {
			file = res.ResolveString(iconAttr.RefID, "file")
		}
		if file != "" {
			if data, ext := loadIconFile(entries, file, res, 0); len(data) > 0 {
				return data, ext
			}
		}
	}
	return fallbackIconFromZip(entries)
}

func loadIconFile(entries map[string]*zip.File, file string, res *ResTable, depth int) ([]byte, string) {
	f := entries[file]
	if f == nil {
		return nil, ""
	}
	if isBitmapExt(file) {
		b, err := readZipEntry(f, 8<<20)
		if err == nil && len(b) > 64 {
			return b, strings.ToLower(filepath.Ext(file))
		}
		return nil, ""
	}
	if strings.EqualFold(filepath.Ext(file), ".xml") && depth < 3 {
		xmlBytes, err := readZipEntry(f, 2<<20)
		if err != nil {
			return nil, ""
		}
		root, err := ParseAXML(xmlBytes)
		if err != nil {
			return nil, ""
		}
		for _, key := range []string{"foreground", "background", "monochrome"} {
			for _, c := range root.ChildNodes(key) {
				if data, ext := iconFromNode(entries, c, res, depth+1); len(data) > 0 {
					return data, ext
				}
			}
		}
		return iconFromNode(entries, root, res, depth+1)
	}
	return nil, ""
}

func iconFromNode(entries map[string]*zip.File, node *XMLNode, res *ResTable, depth int) ([]byte, string) {
	for _, attrName := range []string{"drawable", "src"} {
		a := node.AndroidAttr(attrName)
		if a == nil {
			continue
		}
		file := a.StrValue
		if a.RefID != 0 && res != nil {
			file = res.ResolveString(a.RefID, "file")
		}
		if file != "" {
			if data, ext := loadIconFile(entries, file, res, depth); len(data) > 0 {
				return data, ext
			}
		}
	}
	for _, c := range node.Children {
		if data, ext := iconFromNode(entries, c, res, depth); len(data) > 0 {
			return data, ext
		}
	}
	return nil, ""
}

func fallbackIconFromZip(entries map[string]*zip.File) ([]byte, string) {
	type cand struct {
		name  string
		score float64
	}
	var cands []cand
	for name, f := range entries {
		if !strings.HasPrefix(name, "res/") || !isBitmapExt(name) {
			continue
		}
		s := float64(f.UncompressedSize64)
		low := strings.ToLower(name)
		if strings.Contains(low, "ic_launcher") || strings.Contains(low, "app_icon") || strings.Contains(low, "ic_app") {
			s *= 6
		}
		if strings.Contains(low, "_round") || strings.Contains(low, "_foreground") || strings.Contains(low, "_background") {
			s *= 0.4
		}
		switch {
		case strings.Contains(low, "xxxhdpi"):
			s *= 1.6
		case strings.Contains(low, "xxhdpi"):
			s *= 1.4
		case strings.Contains(low, "xhdpi"):
			s *= 1.2
		}
		cands = append(cands, cand{name: name, score: s})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	for i := 0; i < len(cands) && i < 20; i++ {
		if b, err := readZipEntry(entries[cands[i].name], 8<<20); err == nil && len(b) > 64 {
			return b, strings.ToLower(filepath.Ext(cands[i].name))
		}
	}
	return nil, ""
}
