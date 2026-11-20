// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package avo

import (
	"context"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
	"github.com/D0bby999/anyprofile/internal/mobile/vm"
	"github.com/D0bby999/anyprofile/internal/netproxy"
)

// Router prefers the Metal-accelerated avo driver for new devices when staged
// on macOS while keeping legacy QEMU "vm" devices working seamlessly.
type Router struct {
	avo *Driver
	vm  *vm.Driver
}

// NewRouter wraps both the avo Metal driver and legacy QEMU vm driver.
func NewRouter(avoDrv *Driver, vmDrv *vm.Driver) *Router {
	return &Router{avo: avoDrv, vm: vmDrv}
}

func (r *Router) primaryAvo() bool {
	return r.avo != nil && r.avo.Available() == nil
}

func (r *Router) Name() string {
	if r.primaryAvo() {
		return r.avo.Name()
	}
	return r.vm.Name()
}

func (r *Router) SupportsDriver(name string) bool {
	return name == "avo" || name == "vm"
}

func (r *Router) Available() error {
	if r.primaryAvo() {
		return nil
	}
	return r.vm.Available()
}

func (r *Router) forDevice(dev mobile.Device) mobile.Driver {
	if dev.Driver == "avo" || (dev.Driver == "" && r.primaryAvo() && !exists(filepath.Join(r.avo.devices, dev.ID, "vda.qcow2"))) {
		return r.avo
	}
	return r.vm
}

func (r *Router) isAvoID(id string) bool {
	return r.avo.proc(id) != nil || exists(filepath.Join(r.avo.devices, id, "inst"))
}

func (r *Router) Create(ctx context.Context, dev mobile.Device) (string, string, error) {
	return r.forDevice(dev).Create(ctx, dev)
}

func (r *Router) Start(ctx context.Context, dev mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	return r.forDevice(dev).Start(ctx, dev, o)
}

func (r *Router) Stop(ctx context.Context, dev mobile.Device) error {
	return r.forDevice(dev).Stop(ctx, dev)
}

func (r *Router) Delete(ctx context.Context, dev mobile.Device) error {
	if dev.Driver == "avo" {
		return r.avo.Delete(ctx, dev)
	}
	return r.vm.Delete(ctx, dev)
}

func (r *Router) Backup(ctx context.Context, dev mobile.Device, dir string) error {
	return r.forDevice(dev).Backup(ctx, dev, dir)
}

func (r *Router) Restore(ctx context.Context, dev mobile.Device, dir string) error {
	return r.forDevice(dev).Restore(ctx, dev, dir)
}

func (r *Router) Adopt(ctx context.Context, dev mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	return r.forDevice(dev).Adopt(ctx, dev, l, up)
}

func (r *Router) ADB(dev mobile.Device) (adb.Client, bool) {
	if c, ok := r.avo.ADB(dev); ok {
		return c, true
	}
	return r.vm.ADB(dev)
}

func (r *Router) Pause(id string) error {
	if r.avo.proc(id) != nil {
		return r.avo.Pause(id)
	}
	return r.vm.Pause(id)
}

func (r *Router) Resume(id string) error {
	if r.avo.proc(id) != nil {
		return r.avo.Resume(id)
	}
	return r.vm.Resume(id)
}

func (r *Router) Paused(id string) bool {
	if r.avo.proc(id) != nil {
		return r.avo.Paused(id)
	}
	return r.vm.Paused(id)
}

func (r *Router) Windowed(id string) bool {
	if r.avo.proc(id) != nil {
		return r.avo.Windowed(id)
	}
	return r.vm.Windowed(id)
}

func (r *Router) Show(ctx context.Context, dev mobile.Device) error {
	if r.avo.proc(dev.ID) != nil || dev.Driver == "avo" {
		return r.avo.Show(ctx, dev)
	}
	return r.vm.Show(ctx, dev)
}

func (r *Router) Reset(ctx context.Context, dev mobile.Device, mode string) error {
	if dev.Driver == "avo" {
		return r.avo.Reset(ctx, dev, mode)
	}
	return r.vm.Reset(ctx, dev, mode)
}

func (r *Router) ArrangeWindows(ctx context.Context, layout string, devs []mobile.Device) int {
	var avoDevs, vmDevs []mobile.Device
	for _, dev := range devs {
		if dev.Driver == "avo" || r.avo.proc(dev.ID) != nil {
			avoDevs = append(avoDevs, dev)
		} else {
			vmDevs = append(vmDevs, dev)
		}
	}
	return r.avo.ArrangeWindows(ctx, layout, avoDevs) + r.vm.ArrangeWindows(ctx, layout, vmDevs)
}

func (r *Router) SetFastStart(on bool) {
	r.avo.SetFastStart(on)
	r.vm.SetFastStart(on)
}

func (r *Router) DropRAM(dev mobile.Device) {
	if dev.Driver == "avo" {
		r.avo.DropRAM(dev)
		return
	}
	r.vm.DropRAM(dev)
}

func (r *Router) Metrics(id string) mobile.Metrics {
	if r.isAvoID(id) {
		return r.avo.Metrics(id)
	}
	return r.vm.Metrics(id)
}

func (r *Router) WindowOnTop(dev mobile.Device) bool {
	if dev.Driver == "avo" {
		return r.avo.WindowOnTop(dev)
	}
	return r.vm.WindowOnTop(dev)
}

func (r *Router) SetWindowOnTop(ctx context.Context, dev mobile.Device, on bool) error {
	if dev.Driver == "avo" {
		return r.avo.SetWindowOnTop(ctx, dev, on)
	}
	return r.vm.SetWindowOnTop(ctx, dev, on)
}

func (r *Router) WindowMuted(dev mobile.Device) bool {
	if dev.Driver == "avo" {
		return r.avo.WindowMuted(dev)
	}
	return r.vm.WindowMuted(dev)
}

func (r *Router) SetWindowMuted(ctx context.Context, dev mobile.Device, muted bool) error {
	if dev.Driver == "avo" {
		return r.avo.SetWindowMuted(ctx, dev, muted)
	}
	return r.vm.SetWindowMuted(ctx, dev, muted)
}

func (r *Router) InstallStatus() mobile.InstallStatus {
	if r.primaryAvo() || r.avo.HasOffer() {
		return r.avo.InstallStatus()
	}
	return r.vm.InstallStatus()
}

// UpdateInstallers upgrades existing runtimes and adds Metal alongside existing QEMU.
// Devices keep their assigned driver and disk format.
func (r *Router) UpdateInstallers() []mobile.Installer {
	if r.vm.InstallStatus().Runtime.Installed != "" {
		return []mobile.Installer{r.vm, r.avo}
	}
	if r.avo.InstallStatus().Runtime.Installed != "" {
		return []mobile.Installer{r.avo}
	}
	return nil
}

func (r *Router) Install(ctx context.Context, progress func(engine string, done, total int64)) error {
	if r.avo.HasOffer() {
		return r.avo.Install(ctx, progress)
	}
	if r.primaryAvo() {
		return nil
	}
	return r.vm.Install(ctx, progress)
}

func (r *Router) OfferRuntime(rs []engine.Release) error {
	errAvo := r.avo.OfferRuntime(rs)
	errVM := r.vm.OfferRuntime(rs)
	if errAvo != nil {
		return errAvo
	}
	return errVM
}

var (
	_ mobile.Driver         = (*Router)(nil)
	_ mobile.ADBer          = (*Router)(nil)
	_ mobile.Pauser         = (*Router)(nil)
	_ mobile.Shower         = (*Router)(nil)
	_ mobile.Resetter       = (*Router)(nil)
	_ mobile.Installer      = (*Router)(nil)
	_ mobile.RuntimeOfferer = (*Router)(nil)
)
