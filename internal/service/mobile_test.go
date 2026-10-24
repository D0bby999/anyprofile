// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/netproxy"
	"github.com/D0bby999/anyprofile/internal/store"
)

// fakeMobile is a driver that runs nothing and records what it was asked.
type fakeMobile struct {
	mu        sync.Mutex
	avail     error
	createErr error
	running   map[string]chan struct{}
	opts      mobile.StartOptions
	deleted   []string
	backedUp  string
	restored  string
}

func (f *fakeMobile) Name() string     { return "vm" }
func (f *fakeMobile) Available() error { return f.avail }
func (f *fakeMobile) Create(ctx context.Context, d mobile.Device) (string, string, error) {
	return "dir-" + d.ID, "lineage-23.2", f.createErr
}

func (f *fakeMobile) Start(ctx context.Context, d mobile.Device, o mobile.StartOptions) (mobile.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opts = o
	done := make(chan struct{})
	f.running[d.ID] = done
	return mobile.Run{Launch: mobile.Launch{PID: 1}, Done: done}, nil
}

func (f *fakeMobile) Stop(ctx context.Context, d mobile.Device) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	close(f.running[d.ID])
	delete(f.running, d.ID)
	return nil
}

func (f *fakeMobile) Delete(ctx context.Context, d mobile.Device) error {
	f.deleted = append(f.deleted, d.Ref)
	return nil
}

func (f *fakeMobile) Backup(ctx context.Context, d mobile.Device, dir string) error {
	f.backedUp = dir
	return os.WriteFile(filepath.Join(dir, "vdb.qcow2"), []byte("disk"), 0o600)
}

func (f *fakeMobile) Restore(ctx context.Context, d mobile.Device, dir string) error {
	f.restored = dir
	return nil
}

func (f *fakeMobile) Adopt(ctx context.Context, d mobile.Device, l mobile.Launch, up *netproxy.Upstream) (mobile.Run, error) {
	return mobile.Run{}, errors.New("gone")
}

// newMobileService is newService with a fake driver and, when paid, a Pro key in state.
func newMobileService(t *testing.T, proState string) (*Service, *fakeMobile) {
	t.Helper()
	s, sec := newService(t)
	f := &fakeMobile{running: map[string]chan struct{}{}}
	s.Mobile = mobile.NewManager(f, s.Store, s.Log, nil)
	if proState != "" {
		if err := sec.Set(proVault, proAccount, []byte("APRO-1")); err != nil {
			t.Fatal(err)
		}
		if err := s.saveProState(proSaved{State: proState}); err != nil {
			t.Fatal(err)
		}
	}
	return s, f
}

func code(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestMobileProGate(t *testing.T) {
	ctx := context.Background()
	for _, st := range []string{"", "pending", "expired", "revoked"} {
		s, _ := newMobileService(t, st)
		if _, err := s.CreateMobileDevice(ctx, MobileInput{Name: "a"}); code(err) != "pro_required" {
			t.Errorf("state %q: create = %v", st, err)
		}
		if s.MobileAvailability().Pro {
			t.Errorf("state %q counts as Pro", st)
		}
	}
	for _, st := range []string{"active", "past_due"} {
		s, _ := newMobileService(t, st)
		if _, err := s.CreateMobileDevice(ctx, MobileInput{Name: "a"}); err != nil {
			t.Errorf("state %q: create = %v", st, err)
		}
	}
}

// A lapsed subscription keeps its devices: edit, back up, delete; no opening.
func TestMobileLapsedKeepsData(t *testing.T) {
	ctx := context.Background()
	s, f := newMobileService(t, "active")
	d, err := s.CreateMobileDevice(ctx, MobileInput{Name: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.saveProState(proSaved{State: "expired"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartMobileDevice(ctx, d.ID); code(err) != "pro_required" {
		t.Fatalf("start = %v", err)
	}
	if err := s.UpdateMobileDevice(ctx, d.ID, MobileInput{Name: "renamed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.TrashMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "dir-"+d.ID {
		t.Fatalf("driver deleted %v", f.deleted)
	}
	if _, err := os.Stat(s.Dirs.MobileBackup(d.ID)); !os.IsNotExist(err) {
		t.Fatal("backup left behind")
	}
}

func TestMobileCreateAndStart(t *testing.T) {
	ctx := context.Background()
	s, f := newMobileService(t, "active")
	if _, err := s.ImportProxies(ctx, "gw.example:10002:u:secret2", ""); err != nil {
		t.Fatal(err)
	}
	px, _ := s.ListProxies()
	d, err := s.CreateMobileDevice(ctx, MobileInput{Name: " phone ", ProxyID: px[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "phone" || d.RAMMB != defaultMobileRAMMB || d.CPUs != defaultMobileCPUs ||
		d.DriverRef != "dir-"+d.ID || d.ImageVersion != "lineage-23.2" || d.Driver != "vm" {
		t.Fatalf("created %+v", d)
	}
	if _, err := s.StartMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if f.opts.Timezone != "Asia/Ho_Chi_Minh" || f.opts.Locale != "vi-VN" {
		t.Fatalf("network %+v", f.opts)
	}
	if f.opts.Upstream == nil || f.opts.Upstream.Port != 10002 || f.opts.Upstream.Password != "secret2" {
		t.Fatalf("upstream %+v", f.opts.Upstream)
	}
	rows, _ := s.ListMobileDevices(store.ProfileFilter{})
	if len(rows) != 1 || rows[0].State != mobile.Running || rows[0].LastOpenAt == "" {
		t.Fatalf("rows %+v", rows)
	}
	// A running device keeps its proxy and blocks deleting it.
	if err := s.UpdateMobileDevice(ctx, d.ID, MobileInput{Name: "phone"}); code(err) != "mobile_running" {
		t.Fatalf("proxy change while running = %v", err)
	}
	if err := s.DeleteProxy(px[0].ID); code(err) != "proxy_in_use" {
		t.Fatalf("delete proxy = %v", err)
	}
	if err := s.BackupMobileDevice(ctx, d.ID); code(err) != "mobile_running" {
		t.Fatalf("backup while running = %v", err)
	}
	if err := s.StopMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProxy(px[0].ID); err != nil {
		t.Fatalf("delete proxy after close = %v", err)
	}
	got, _ := s.Store.GetMobileDevice(d.ID)
	if got.ProxyID != "" {
		t.Fatal("deleted proxy still set on the device")
	}
}

func TestMobileUsage(t *testing.T) {
	ctx := context.Background()
	s, _ := newMobileService(t, "active")
	if _, err := s.ImportProxies(ctx, "gw.example:10001:u:p", ""); err != nil {
		t.Fatal(err)
	}
	px, _ := s.ListProxies()
	if _, err := s.CreateMobileDevice(ctx, MobileInput{Name: "phone", ProxyID: px[0].ID}); err != nil {
		t.Fatal(err)
	}
	use, err := s.Store.ProxyUsage()
	if err != nil || len(use[px[0].ID]) != 1 || use[px[0].ID][0] != "phone" {
		t.Fatalf("usage %v %v", use, err)
	}
}

func TestMobileRefusals(t *testing.T) {
	ctx := context.Background()
	s, f := newMobileService(t, "active")
	if _, err := s.ImportProxies(ctx, "gw.example:823:u:rot", ""); err != nil {
		t.Fatal(err)
	}
	px, _ := s.ListProxies()
	for _, tc := range []struct {
		in   MobileInput
		want string
	}{
		{MobileInput{Name: "  "}, "mobile_name_required"},
		{MobileInput{Name: "a", RAMMB: 1024}, "mobile_bad_ram"},
		{MobileInput{Name: "a", CPUs: 9}, "mobile_bad_cpus"},
		{MobileInput{Name: "a", ProxyID: px[0].ID}, "proxy_rotating"},
	} {
		if _, err := s.CreateMobileDevice(ctx, tc.in); code(err) != tc.want {
			t.Errorf("%+v: %v, want %s", tc.in, err, tc.want)
		}
	}
	f.avail = apperr.New("ldplayer_missing", "install LDPlayer 9")
	if _, err := s.CreateMobileDevice(ctx, MobileInput{Name: "a"}); code(err) != "ldplayer_missing" {
		t.Fatalf("driver not ready: %v", err)
	}
	if a := s.MobileAvailability(); a.Ready || a.Reason != "ldplayer_missing" || !a.Pro {
		t.Fatalf("availability %+v", a)
	}
	f.avail = nil
	f.createErr = errors.New("first boot failed")
	if _, err := s.CreateMobileDevice(ctx, MobileInput{Name: "a"}); err == nil {
		t.Fatal("no error")
	}
	if rows, _ := s.ListMobileDevices(store.ProfileFilter{}); len(rows) != 0 {
		t.Fatalf("a device the driver could not make was kept: %+v", rows)
	}
}

func TestMobileOtherOS(t *testing.T) {
	ctx := context.Background()
	s, _ := newMobileService(t, "active")
	d := store.MobileDevice{ID: "x", Name: "made elsewhere", Driver: "vm", Platform: "plan9", RAMMB: 3072, CPUs: 4}
	if err := s.Store.CreateMobileDevice(&d); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartMobileDevice(ctx, "x"); code(err) != "mobile_other_os" {
		t.Fatalf("start = %v", err)
	}
}

func TestMobileSlots(t *testing.T) {
	for gib, want := range map[int64]int{8: 1, 16: 2, 24: 3, 32: 4, 64: 4} {
		if got := mobileSlots(gib << 30); got != want {
			t.Errorf("%d GiB: %d, want %d", gib, got, want)
		}
	}
}

func TestMobileUnsupported(t *testing.T) {
	s, _ := newService(t)
	if a := s.MobileAvailability(); a.Supported || a.Reason != "mobile_unsupported" {
		t.Fatalf("availability %+v", a)
	}
	if rows, err := s.ListMobileDevices(store.ProfileFilter{}); err != nil || len(rows) != 0 {
		t.Fatalf("%v %v", rows, err)
	}
}

func TestInstallMobileApps(t *testing.T) {
	ctx := context.Background()
	s, _ := newMobileService(t, "active")
	d, err := s.CreateMobileDevice(ctx, MobileInput{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallMobileApps(ctx, d.ID, []string{"/x/app.apk", "/x/app.xapk"}); code(err) != "mobile_apk" {
		t.Fatalf("not an apk: %v", err)
	}
	// The fake driver has no adb; real ones refuse a closed device with mobile_not_running.
	if n, err := s.InstallMobileApps(ctx, d.ID, []string{"/x/app.APK"}); err == nil || n != 0 {
		t.Fatalf("installed on a device it cannot reach: %d %v", n, err)
	}
}

// The proxy of a running device that stops answering is marked failed after
// two tries in a row, and cleared once it answers again.
func TestWatchMobileProxies(t *testing.T) {
	ctx := context.Background()
	s, _ := newMobileService(t, "active")
	if _, err := s.ImportProxies(ctx, "gw.example:10002:u:secret2", ""); err != nil {
		t.Fatal(err)
	}
	px, _ := s.ListProxies()
	d, err := s.CreateMobileDevice(ctx, MobileInput{Name: "phone", ProxyID: px[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	var down bool
	pings := 0
	s.PingProxy = func(_ context.Context, up netproxy.Upstream) error {
		pings++
		if up.Port != 10002 || up.Password != "secret2" {
			t.Errorf("pinged %+v", up)
		}
		if down {
			return errors.New("connection refused")
		}
		return nil
	}
	misses := map[string]int{}
	failed := func() bool { p, _ := s.Store.GetProxy(px[0].ID); return p.FailedAt != "" }

	s.watchMobileProxies(ctx, misses)
	if pings != 0 {
		t.Fatal("a stopped device's proxy was tried")
	}
	if _, err := s.StartMobileDevice(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	down = true
	s.watchMobileProxies(ctx, misses)
	if failed() {
		t.Fatal("one missed try marked the proxy failed")
	}
	s.watchMobileProxies(ctx, misses)
	if !failed() {
		t.Fatal("two missed tries did not mark the proxy failed")
	}
	down = false
	s.watchMobileProxies(ctx, misses)
	if failed() {
		t.Fatal("an answering proxy stayed failed")
	}
	if pings != 3 {
		t.Fatalf("pings = %d, want 3", pings)
	}
}
