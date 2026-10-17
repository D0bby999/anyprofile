// SPDX-License-Identifier: AGPL-3.0-or-later

package apostate

import (
	"bytes"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGlobalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

// memoryStatusEx is MEMORYSTATUSEX.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func hostMemBytes() uint64 {
	m := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return 0
	}
	return m.TotalPhys
}

// versionManifest is Chromium's version_assembly_manifest.template for a
// release. chrome.exe names the assembly "155.0.8059.31" as a dependency, and
// Windows refuses to start it ("side-by-side configuration is incorrect")
// without this file beside it. Apostate 0.5.1 does not ship it
// (github.com/heretic-tech/apostate/issues/2); the workaround there is this
// file. The release files themselves are not touched.
func versionManifest(chromium string) []byte {
	return []byte(`<?xml version='1.0' encoding='UTF-8' standalone='yes'?>
<assembly xmlns='urn:schemas-microsoft-com:asm.v1' manifestVersion='1.0'>
  <assemblyIdentity
      name='` + chromium + `'
      version='` + chromium + `'
      type='win32'/>
  <file name='chrome_elf.dll'/>
</assembly>
`)
}

// completeApp writes the version assembly manifest the release lacks.
func completeApp(app, chromium string) error {
	want := versionManifest(chromium)
	p := filepath.Join(app, chromium+".manifest")
	if b, err := os.ReadFile(p); err == nil && bytes.Equal(b, want) {
		return nil
	}
	return os.WriteFile(p, want, 0o644)
}
