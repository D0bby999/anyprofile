// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ShortcutSpec describes an OS launcher shortcut for an Android app inside AnyProfile.
type ShortcutSpec struct {
	Package     string
	Label       string
	AnyProfBin  string
	IconDataURL string
	IconsDir    string
	TargetDir   string
}

var unsafeShortcutRe = regexp.MustCompile(`[\\/:*?"<>|]+`)

func safeShortcutName(label string) string {
	s := unsafeShortcutRe.ReplaceAllString(label, "_")
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60])
	}
	if s == "" || s == "." || s == ".." {
		return "Android App"
	}
	return s
}

func defaultShortcutDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Applications", "AnyProfile Apps")
	case "windows":
		return filepath.Join(home, "Desktop")
	default:
		return filepath.Join(home, ".local", "share", "applications")
	}
}

// CreateShortcut writes a native OS shortcut (.app / .lnk / .desktop) that
// invokes AnyProfile to open the specified Android package.
func CreateShortcut(spec ShortcutSpec) (string, error) {
	if !ValidPackageName(spec.Package) {
		return "", fmt.Errorf("invalid package name: %q", spec.Package)
	}
	if spec.AnyProfBin == "" {
		return "", errors.New("AnyProfile executable path is required")
	}
	if spec.TargetDir == "" {
		spec.TargetDir = defaultShortcutDir()
	}

	iconFile := ""
	if raw, ext := decodeDataURL(spec.IconDataURL); len(raw) > 0 && spec.IconsDir != "" {
		_ = os.MkdirAll(spec.IconsDir, 0o700)
		iconFile = filepath.Join(spec.IconsDir, spec.Package+ext)
		_ = os.WriteFile(iconFile, raw, 0o644)
	}

	switch runtime.GOOS {
	case "darwin":
		return createDarwinBundle(spec, iconFile)
	case "windows":
		return createWindowsLnk(spec, iconFile)
	default:
		return createLinuxDesktop(spec, iconFile)
	}
}

func decodeDataURL(dataURL string) ([]byte, string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, ""
	}
	comma := strings.IndexByte(dataURL, ',')
	if comma < 0 {
		return nil, ""
	}
	hdr := dataURL[:comma]
	b, err := base64.StdEncoding.DecodeString(dataURL[comma+1:])
	if err != nil {
		return nil, ""
	}
	ext := ".png"
	switch {
	case strings.Contains(hdr, "webp"):
		ext = ".webp"
	case strings.Contains(hdr, "jpeg"):
		ext = ".jpg"
	}
	return b, ext
}

func createDarwinBundle(spec ShortcutSpec, iconFile string) (string, error) {
	name := safeShortcutName(spec.Label)
	bundleDir := filepath.Join(spec.TargetDir, name+".app")
	contentsDir := filepath.Join(bundleDir, "Contents")
	macOSDir := filepath.Join(contentsDir, "MacOS")
	resDir := filepath.Join(contentsDir, "Resources")
	for _, d := range []string{macOSDir, resDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", err
		}
	}

	iconKey := ""
	if iconFile != "" {
		icnsPath := filepath.Join(resDir, "AppIcon.icns")
		if buildICNS(iconFile, icnsPath) == nil {
			iconKey = "AppIcon.icns"
		} else if raw, err := os.ReadFile(iconFile); err == nil {
			_ = os.WriteFile(filepath.Join(resDir, "AppIcon.png"), raw, 0o644)
			iconKey = "AppIcon.png"
		}
	}
	iconPlist := ""
	if iconKey != "" {
		iconPlist = fmt.Sprintf("\t<key>CFBundleIconFile</key>\n\t<string>%s</string>\n", html.EscapeString(iconKey))
	}
	bundleID := "com.anyprofile.apk." + spec.Package
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundleDisplayName</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>launch</string>
%s	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
`, html.EscapeString(name), html.EscapeString(name), html.EscapeString(bundleID), iconPlist)
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	safeBin := "'" + strings.ReplaceAll(spec.AnyProfBin, "'", "'\\''") + "'"
	script := fmt.Sprintf("#!/bin/sh\nexec %s --launch-apk %s\n", safeBin, spec.Package)
	if err := os.WriteFile(filepath.Join(macOSDir, "launch"), []byte(script), 0o755); err != nil {
		return "", err
	}
	_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", bundleDir).Run()
	return bundleDir, nil
}

func buildICNS(srcImg, outICNS string) error {
	tmp, err := os.MkdirTemp("", "anyprofile-icns-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	iconset := filepath.Join(tmp, "AppIcon.iconset")
	if err := os.MkdirAll(iconset, 0o755); err != nil {
		return err
	}
	for _, s := range []struct {
		px   int
		name string
	}{
		{16, "icon_16x16.png"}, {32, "icon_16x16@2x.png"},
		{32, "icon_32x32.png"}, {64, "icon_32x32@2x.png"},
		{128, "icon_128x128.png"}, {256, "icon_128x128@2x.png"},
		{256, "icon_256x256.png"}, {512, "icon_256x256@2x.png"},
	} {
		dst := filepath.Join(iconset, s.name)
		if err := exec.Command("sips", "-s", "format", "png", "-z", fmt.Sprintf("%d", s.px), fmt.Sprintf("%d", s.px), srcImg, "--out", dst).Run(); err != nil {
			return err
		}
	}
	return exec.Command("iconutil", "-c", "icns", iconset, "-o", outICNS).Run()
}

func createLinuxDesktop(spec ShortcutSpec, iconFile string) (string, error) {
	if err := os.MkdirAll(spec.TargetDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(spec.TargetDir, "anyprofile-apk-"+spec.Package+".desktop")
	name := strings.ReplaceAll(safeShortcutName(spec.Label), "\n", " ")
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`", `%`, `%%`).Replace(spec.AnyProfBin) + `"`
	var b strings.Builder
	b.WriteString("[Desktop Entry]\nType=Application\nVersion=1.0\n")
	fmt.Fprintf(&b, "Name=%s\nExec=%s --launch-apk %s\n", name, quoted, spec.Package)
	if iconFile != "" {
		fmt.Fprintf(&b, "Icon=%s\n", iconFile)
	}
	b.WriteString("Terminal=false\nCategories=Utility;\n")
	if err := os.WriteFile(outPath, []byte(b.String()), 0o755); err != nil {
		return "", err
	}
	_ = os.Chmod(outPath, 0o755)
	return outPath, nil
}

func createWindowsLnk(spec ShortcutSpec, iconFile string) (string, error) {
	if err := os.MkdirAll(spec.TargetDir, 0o755); err != nil {
		return "", err
	}
	name := safeShortcutName(spec.Label)
	lnkPath := filepath.Join(spec.TargetDir, name+".lnk")
	icoPath := spec.AnyProfBin
	if iconFile != "" {
		if raw, err := os.ReadFile(iconFile); err == nil {
			if ico, err := buildPNGInICO(raw); err == nil {
				dst := strings.TrimSuffix(iconFile, filepath.Ext(iconFile)) + ".ico"
				if os.WriteFile(dst, ico, 0o644) == nil {
					icoPath = dst
				}
			}
		}
	}
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	ps := fmt.Sprintf(
		"$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%s'); $s.TargetPath = '%s'; $s.Arguments = '--launch-apk %s'; $s.IconLocation = '%s,0'; $s.Description = '%s'; $s.Save()",
		esc(lnkPath), esc(spec.AnyProfBin), spec.Package, esc(icoPath), esc(name),
	)
	if out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create shortcut: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return lnkPath, nil
}

func buildPNGInICO(png []byte) ([]byte, error) {
	if len(png) < 24 || string(png[1:4]) != "PNG" {
		return nil, errors.New("not a PNG")
	}
	w := binary.BigEndian.Uint32(png[16:20])
	h := binary.BigEndian.Uint32(png[20:24])
	if w == 0 || h == 0 || w > 1024 || h > 1024 {
		return nil, errors.New("invalid PNG size")
	}
	var hdr bytes.Buffer
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(0))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(1))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(1))
	wb, hb := byte(w), byte(h)
	if w >= 256 {
		wb = 0
	}
	if h >= 256 {
		hb = 0
	}
	hdr.Write([]byte{wb, hb, 0, 0})
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(1))
	_ = binary.Write(&hdr, binary.LittleEndian, uint16(32))
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(len(png)))
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(22))
	hdr.Write(png)
	return hdr.Bytes(), nil
}
