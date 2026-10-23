// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import "syscall"

// SameProcess is true when pid is alive and is the process that started at
// startTime (from ProcessStartTime): a recycled pid is not. Mobile devices
// use it to adopt a VM, as Adopt does a browser.
func SameProcess(pid int, startTime string) bool { return sameProcess(pid, startTime) }

// ProcessStartTime is the kernel's start time of pid, as a string to store.
func ProcessStartTime(pid int) (string, error) { return pidStartTime(pid) }

// DetachedAttr starts a child in its own process group, so it outlives a
// signal sent to the app's group and can be adopted after a restart.
func DetachedAttr() *syscall.SysProcAttr { return procAttr() }
