// SPDX-License-Identifier: AGPL-3.0-or-later

package mobile

/*
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <mach/vm_statistics.h>

static int darwinHostMemory(uint64_t *usedBytes, uint64_t *availBytes) {
	mach_port_t host = mach_host_self();
	vm_size_t pageSize = 0;
	if (host_page_size(host, &pageSize) != KERN_SUCCESS || pageSize == 0) {
		return -1;
	}
	vm_statistics64_data_t vm;
	mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
	if (host_statistics64(host, HOST_VM_INFO64, (host_info64_t)&vm, &count) != KERN_SUCCESS) {
		return -1;
	}
	uint64_t ps = (uint64_t)pageSize;
	// Activity Monitor formula: active + wired + compressor = app/kernel pressure;
	// free + inactive + speculative + purgeable are reclaimable for VMs.
	*usedBytes = ((uint64_t)vm.active_count + (uint64_t)vm.wire_count + (uint64_t)vm.compressor_page_count) * ps;
	*availBytes = ((uint64_t)vm.free_count + (uint64_t)vm.inactive_count + (uint64_t)vm.speculative_count) * ps;
	return 0;
}

static int darwinCPUTicks(uint64_t *busy, uint64_t *total) {
	mach_port_t host = mach_host_self();
	host_cpu_load_info_data_t cpu;
	mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
	if (host_statistics(host, HOST_CPU_LOAD_INFO, (host_info_t)&cpu, &count) != KERN_SUCCESS) {
		return -1;
	}
	uint64_t u = (uint64_t)cpu.cpu_ticks[CPU_STATE_USER];
	uint64_t s = (uint64_t)cpu.cpu_ticks[CPU_STATE_SYSTEM];
	uint64_t n = (uint64_t)cpu.cpu_ticks[CPU_STATE_NICE];
	uint64_t i = (uint64_t)cpu.cpu_ticks[CPU_STATE_IDLE];
	*busy = u + s + n;
	*total = u + s + n + i;
	return 0;
}
*/
import "C"

import (
	"math"
	"sync"

	"golang.org/x/sys/unix"
)

// FreeBytes is the space an unprivileged user can still write on dir's volume.
func FreeBytes(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// TotalRAM is the computer's physical memory in bytes.
func TotalRAM() (int64, error) {
	n, err := unix.SysctlUint64("hw.memsize")
	return int64(n), err
}

// HostMemory returns total, currently pressured/used, and reclaimable available RAM.
func HostMemory() (totalBytes, usedBytes, availBytes int64, err error) {
	totalBytes, err = TotalRAM()
	if err != nil {
		return 0, 0, 0, err
	}
	var used, avail C.uint64_t
	if C.darwinHostMemory(&used, &avail) != 0 {
		return totalBytes, 0, totalBytes, nil
	}
	u := int64(used)
	a := int64(avail)
	if u > totalBytes {
		u = totalBytes
	}
	if a > totalBytes {
		a = totalBytes - u
	}
	return totalBytes, u, a, nil
}

var (
	cpuSampleMu  sync.Mutex
	prevCPUBusy  uint64
	prevCPUTotal uint64
	hasCPUSample bool
	lastCPUPct   float64
)

// HostCPUPercent returns recent host CPU utilization (0..100) without blocking.
func HostCPUPercent() float64 {
	var busy, total C.uint64_t
	if C.darwinCPUTicks(&busy, &total) != 0 {
		return 0
	}
	b, t := uint64(busy), uint64(total)
	cpuSampleMu.Lock()
	defer cpuSampleMu.Unlock()
	if !hasCPUSample {
		prevCPUBusy, prevCPUTotal, hasCPUSample = b, t, true
		return 0
	}
	if t > prevCPUTotal {
		db := float64(b - prevCPUBusy)
		dt := float64(t - prevCPUTotal)
		pct := (db / dt) * 100.0
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		lastCPUPct = math.Round(pct*10) / 10
		prevCPUBusy, prevCPUTotal = b, t
	}
	return lastCPUPct
}
