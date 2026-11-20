// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/pro"
)

type browserUpdateShelf struct{ shelfEngine }

func (b *browserUpdateShelf) InstallIfMissing(_ context.Context, version string) (bool, error) {
	b.installed[version] = true
	return true, nil
}

func TestAutoUpdateAnyBrowser(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	b := &browserUpdateShelf{shelfEngine{installed: map[string]bool{}}}
	s.Engines["anybrowser"] = b
	var changed []string
	s.OnChange = func(kind string) { changed = append(changed, kind) }
	if err := s.AutoUpdateAnyBrowser(ctx); err != nil || len(b.installed) != 0 {
		t.Fatalf("first install: %v, installed %v", err, b.installed)
	}
	b.installed["0.3.5"] = true
	if err := s.SetUpdateAuto(false); err != nil {
		t.Fatal(err)
	}
	if err := s.AutoUpdateAnyBrowser(ctx); err != nil || len(b.installed) != 1 {
		t.Fatalf("opted out: %v, installed %v", err, b.installed)
	}
	if err := s.SetUpdateAuto(true); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.AutoUpdateAnyBrowser(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if !b.installed["0.4.5"] || !b.installed["0.3.5"] || len(b.installed) != 2 || strings.Join(changed, ",") != "engines" {
		t.Fatalf("want only the compatible upgrade, alongside the old build; got %v, events %v", b.installed, changed)
	}
}

type mobileUpdateShelf struct {
	*fakeInstaller
	installed, latest string
}

func (m *mobileUpdateShelf) UpdateInstallers() []mobile.Installer {
	if m.installed == "" {
		return nil
	}
	return []mobile.Installer{m}
}

func (m *mobileUpdateShelf) OfferRuntime(rs []engine.Release) error {
	m.latest = ""
	if len(rs) > 0 {
		m.latest = rs[0].Version
	}
	return nil
}

func (m *mobileUpdateShelf) InstallStatus() mobile.InstallStatus {
	return mobile.InstallStatus{Runtime: mobile.Component{Engine: "anymobile", Installed: m.installed, Latest: m.latest}, Image: mobile.Component{Installed: "old-image"}}
}

func (m *mobileUpdateShelf) Install(ctx context.Context, progress func(string, int64, int64)) error {
	m.installed = m.latest
	return m.fakeInstaller.Install(ctx, progress)
}

func TestAutoUpdateMobile(t *testing.T) {
	s, _ := newService(t)
	f := newFakePro(t, s)
	r := pro.Runtime{Engine: "anymobile", Version: "1.0.2", Platform: engine.HostPlatform(), QEMU: "11.1.2",
		SHA256: strings.Repeat("a", 64), Files: map[string]string{"bin/qemu": strings.Repeat("b", 64)}, Bin: "bin/qemu",
		URL: "https://pro.test/anymobile-1.0.2-darwin-arm64.tar.xz"}
	r.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, pro.RuntimeMessage(r.Engine, r.Platform, r.Version, r.SHA256)))
	f.runtimes = []pro.Runtime{r}
	drv := &mobileUpdateShelf{fakeInstaller: &fakeInstaller{fakeMobile: &fakeMobile{}}}
	s.Mobile = mobile.NewManager(drv, s.Store, s.Log, nil)
	ctx := context.Background()
	if _, err := s.ProActivate(ctx, f.key); err != nil {
		t.Fatal(err)
	}
	asks := f.asks
	if err := s.AutoUpdateMobile(ctx); err != nil || drv.installs != 0 || f.asks != asks {
		t.Fatalf("first install: %v, installs %d, asks %d", err, drv.installs, f.asks-asks)
	}
	drv.installed = "1.0.1"
	if err := s.Mobile.Busy("device", mobile.Provisioning, func() error { return s.AutoUpdateMobile(ctx) }); err != nil || drv.installs != 0 || f.asks != asks {
		t.Fatalf("busy: %v, installs %d, asks %d", err, drv.installs, f.asks-asks)
	}
	for range 2 {
		if err := s.AutoUpdateMobile(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if drv.installed != "1.0.2" || drv.installs != 1 || f.asks != asks+1 {
		t.Fatalf("auto update: version %s, installs %d, asks %d", drv.installed, drv.installs, f.asks-asks)
	}
	// A fresh server answer without an offer must not reuse the earlier link.
	drv.installed = "1.0.1"
	f.mu.Lock()
	f.runtimes = nil
	f.mu.Unlock()
	if err := s.AutoUpdateMobile(ctx); err != nil || drv.installs != 1 || drv.latest != "" {
		t.Fatalf("withdrawn offer: %v, installs %d, latest %q", err, drv.installs, drv.latest)
	}
}
