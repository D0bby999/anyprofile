// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// FreeBytes is the space the current user can still write on dir's volume.
func FreeBytes(dir string) (int64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, nil, nil); err != nil {
		return 0, err
	}
	return int64(free), nil
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

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

// TotalRAM is the computer's physical memory in bytes.
func TotalRAM() (int64, error) {
	ms := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r == 0 {
		return 0, err
	}
	return int64(ms.TotalPhys), nil
}

// HostMemory returns total, used, and available physical RAM in bytes.
func HostMemory() (totalBytes, usedBytes, availBytes int64, err error) {
	ms := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r == 0 {
		return 0, 0, 0, err
	}
	total := int64(ms.TotalPhys)
	avail := int64(ms.AvailPhys)
	used := total - avail
	if used < 0 {
		used = 0
	}
	return total, used, avail, nil
}

// HostCPUPercent returns 0 on Windows (shortcut: Windows CPU telemetry is deferred until Windows MuMu12 QA).
func HostCPUPercent() float64 { return 0 }
