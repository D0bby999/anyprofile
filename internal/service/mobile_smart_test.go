// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
	mobilesync "github.com/D0bby999/anyprofile/internal/mobile/sync"
)

type fakeSmartDriver struct {
	mobile.Driver
	mu     sync.Mutex
	eco    bool
	focus  bool
	cmds   map[string][]string
	roles  map[string]string
	badges map[string]mobile.ViewBadge
}

func (f *fakeSmartDriver) Name() string          { return "avo" }
func (f *fakeSmartDriver) Available() error      { return nil }
func (f *fakeSmartDriver) SetEcoFPS(on bool)     { f.mu.Lock(); f.eco = on; f.mu.Unlock() }
func (f *fakeSmartDriver) SetFocusAudio(on bool) { f.mu.Lock(); f.focus = on; f.mu.Unlock() }
func (f *fakeSmartDriver) SetBadge(id string, b mobile.ViewBadge) {
	f.mu.Lock()
	if f.badges == nil {
		f.badges = map[string]mobile.ViewBadge{}
	}
	f.badges[id] = b
	f.mu.Unlock()
}
func (f *fakeSmartDriver) SetSyncRole(id, role string, _ int) {
	f.mu.Lock()
	if f.roles == nil {
		f.roles = map[string]string{}
	}
	f.roles[id] = role
	f.mu.Unlock()
}
func (f *fakeSmartDriver) SendControlCommands(_ context.Context, id string, cmds ...string) error {
	f.mu.Lock()
	if f.cmds == nil {
		f.cmds = map[string][]string{}
	}
	f.cmds[id] = append(f.cmds[id], cmds...)
	f.mu.Unlock()
	return nil
}

func TestMobileSmartEcoFocusStatsAndMasterSync(t *testing.T) {
	s, _ := newMobileService(t, "active")
	drv := &fakeSmartDriver{}
	s.Mobile = mobile.NewManager(drv, s.Store, s.Log, nil)

	if err := s.SetMobileEcoMode(true); err != nil {
		t.Fatalf("SetMobileEcoMode: %v", err)
	}
	if !s.MobileEcoMode() || !drv.eco {
		t.Fatalf("expected EcoMode persisted and pushed to driver")
	}
	if err := s.SetMobileFocusAudio(false); err != nil {
		t.Fatalf("SetMobileFocusAudio: %v", err)
	}
	if s.MobileFocusAudio() || drv.focus {
		t.Fatalf("expected FocusAudio off")
	}

	stats := s.MobileHostStats()
	if stats.TotalRAMMB <= 0 || stats.CPUCores <= 0 || !stats.EcoMode {
		t.Fatalf("unexpected HostStats: %+v", stats)
	}

	_, err := s.StartMobileSync("m1", []string{"f1", "f2"}, mobilesync.Jitter{Enabled: false})
	if err != nil {
		t.Fatalf("StartMobileSync: %v", err)
	}
	if drv.roles["m1"] != "master" || drv.roles["f1"] != "follower" || drv.roles["f2"] != "follower" {
		t.Fatalf("unexpected sync roles: %+v", drv.roles)
	}

	s.handleMasterViewInput(context.Background(), "m1", mobile.ViewInputEvent{
		Kind: "touch", Phase: "down", RX: 0.5, RY: 0.5,
	})
	s.handleMasterViewInput(context.Background(), "m1", mobile.ViewInputEvent{
		Kind: "touch", Phase: "up", RX: 0.5, RY: 0.5,
	})
	s.handleMasterViewInput(context.Background(), "m1", mobile.ViewInputEvent{
		Kind: "text", Text: "hi",
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		drv.mu.Lock()
		n1, n2 := len(drv.cmds["f1"]), len(drv.cmds["f2"])
		drv.mu.Unlock()
		if n1 >= 6 && n2 >= 6 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	drv.mu.Lock()
	defer drv.mu.Unlock()
	if len(drv.cmds["f1"]) < 6 || len(drv.cmds["f2"]) < 6 {
		t.Fatalf("expected followers to receive touch+text control commands, got f1=%v f2=%v", drv.cmds["f1"], drv.cmds["f2"])
	}
}

func TestAPKHostStatusAndProxyBadge(t *testing.T) {
	s, _ := newMobileService(t, "active")
	drv := &fakeSmartDriver{}
	s.Mobile = mobile.NewManager(drv, s.Store, s.Log, nil)

	st := s.APKHostStatus()
	if st.State != "stopped" {
		t.Fatalf("expected stopped APK host initially, got %+v", st)
	}
	s.pushMobileBadge("apk-host", "US", "ok", "US Proxy")
	drv.mu.Lock()
	b := drv.badges["apk-host"]
	drv.mu.Unlock()
	if b.Country != "US" || b.Status != "ok" {
		t.Fatalf("unexpected badge: %+v", b)
	}
}
