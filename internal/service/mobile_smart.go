// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

const (
	mobileEcoModeKey    = "mobile_eco_mode"
	mobileFocusAudioKey = "mobile_focus_audio"
	mobileHotkeysKey    = "mobile_global_hotkeys"
)

type followerGesture struct {
	delay time.Duration
	dx    float64
	dy    float64
}

// MobileEcoMode reports whether Multi-instance Eco Mode is enabled.
func (s *Service) MobileEcoMode() bool {
	v, err := s.Store.GetSetting(mobileEcoModeKey)
	return err == nil && v == "1"
}

// SetMobileEcoMode toggles Multi-instance Eco Mode and pushes updated FPS caps
// to every currently open phone viewport.
func (s *Service) SetMobileEcoMode(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := s.Store.SetSetting(mobileEcoModeKey, v); err != nil {
		return err
	}
	if s.Mobile != nil {
		if drv, ok := s.Mobile.Driver().(interface{ SetEcoFPS(bool) }); ok {
			drv.SetEcoFPS(on)
		}
	}
	s.notify("mobile")
	s.notify("settings")
	return nil
}

// MobileFocusAudio reports whether background phones auto-mute on focus change.
// Unset defaults to true.
func (s *Service) MobileFocusAudio() bool {
	v, err := s.Store.GetSetting(mobileFocusAudioKey)
	if err != nil {
		return true
	}
	return v != "0"
}

// SetMobileFocusAudio enables or disables MuMu-style Focus Audio auto-muting.
func (s *Service) SetMobileFocusAudio(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := s.Store.SetSetting(mobileFocusAudioKey, v); err != nil {
		return err
	}
	if s.Mobile != nil {
		if drv, ok := s.Mobile.Driver().(interface{ SetFocusAudio(bool) }); ok {
			drv.SetFocusAudio(on)
		}
	}
	s.notify("settings")
	return nil
}

// MobileHotkeysEnabled reports whether global macOS mobile hotkeys are active.
// Unset defaults to true.
func (s *Service) MobileHotkeysEnabled() bool {
	v, err := s.Store.GetSetting(mobileHotkeysKey)
	if err != nil {
		return true
	}
	return v != "0"
}

// SetMobileHotkeysEnabled turns global mobile hotkeys (Cmd+Shift+A/M/B) on or off.
func (s *Service) SetMobileHotkeysEnabled(ctx context.Context, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := s.Store.SetSetting(mobileHotkeysKey, v); err != nil {
		return err
	}
	s.applyMobileHotkeys(ctx, on)
	s.notify("settings")
	return nil
}

// MobileHostStats returns live host RAM, CPU, running count, and how many more
// mobile instances the machine can safely open without heavy swapping.
func (s *Service) MobileHostStats() mobile.HostStats {
	totalB, usedB, availB, _ := mobile.HostMemory()
	eco := s.MobileEcoMode()
	running := 0
	if s.Mobile != nil {
		running = s.Mobile.Running()
	}
	const hostReserveMB int64 = 3072
	perVMMB := int64(3840)
	if eco {
		perVMMB = 2560
	}
	availMB := availB >> 20
	extra := int((availMB - hostReserveMB) / perVMMB)
	if extra < 0 {
		extra = 0
	}
	if extra > 12 {
		extra = 12
	}
	return mobile.HostStats{
		TotalRAMMB:       totalB >> 20,
		UsedRAMMB:        usedB >> 20,
		AvailableRAMMB:   availMB,
		CPUPercent:       mobile.HostCPUPercent(),
		CPUCores:         runtime.NumCPU(),
		RunningDevices:   running,
		RecommendedExtra: extra,
		EcoMode:          eco,
		FocusAudio:       s.MobileFocusAudio(),
		HotkeysEnabled:   s.MobileHotkeysEnabled(),
	}
}

// InitMobileRuntimeHooks applies persisted Eco/FocusAudio/Hotkey settings and
// connects Master-Window Sync input forwarding from native viewports.
func (s *Service) InitMobileRuntimeHooks(ctx context.Context) {
	if s.Mobile == nil {
		return
	}
	drv := s.Mobile.Driver()
	if ecoDrv, ok := drv.(interface{ SetEcoFPS(bool) }); ok {
		ecoDrv.SetEcoFPS(s.MobileEcoMode())
	}
	if audioDrv, ok := drv.(interface{ SetFocusAudio(bool) }); ok {
		audioDrv.SetFocusAudio(s.MobileFocusAudio())
	}
	if syncDrv, ok := drv.(interface {
		SetMasterInputHandler(func(string, mobile.ViewInputEvent))
	}); ok {
		syncDrv.SetMasterInputHandler(func(devID string, ev mobile.ViewInputEvent) {
			s.handleMasterViewInput(ctx, devID, ev)
		})
	}
	s.applyMobileHotkeys(ctx, s.MobileHotkeysEnabled())
}

// ShutdownMobileRuntimeHooks unregisters global hotkeys on app exit.
func (s *Service) ShutdownMobileRuntimeHooks() {
	if s.Mobile == nil {
		return
	}
	if hk, ok := s.Mobile.Driver().(interface{ UnregisterGlobalHotkeys() }); ok {
		hk.UnregisterGlobalHotkeys()
	}
}

func (s *Service) applyMobileHotkeys(ctx context.Context, on bool) {
	if s.Mobile == nil {
		return
	}
	hk, ok := s.Mobile.Driver().(interface {
		RegisterGlobalHotkeys(func(string))
		UnregisterGlobalHotkeys()
	})
	if !ok {
		return
	}
	if !on {
		hk.UnregisterGlobalHotkeys()
		return
	}
	hk.RegisterGlobalHotkeys(func(action string) {
		switch action {
		case "arrange":
			s.smartMu.Lock()
			layout := "row"
			if s.hotkeyLayoutAlt {
				layout = "grid2x2"
			}
			s.hotkeyLayoutAlt = !s.hotkeyLayoutAlt
			s.smartMu.Unlock()
			_, _ = s.ArrangeMobileWindows(ctx, layout)
		case "mute-all":
			if m, ok := s.Mobile.Driver().(interface{ ToggleAllMuted(context.Context) bool }); ok {
				m.ToggleAllMuted(ctx)
			}
		case "boss-hide":
			if h, ok := s.Mobile.Driver().(interface{ ToggleAllWindowsVisible() bool }); ok {
				h.ToggleAllWindowsVisible()
			}
		}
	})
}

func (s *Service) pushMobileBadge(deviceID, country, status, label string) {
	if s.Mobile == nil {
		return
	}
	if b, ok := s.Mobile.Driver().(interface {
		SetBadge(string, mobile.ViewBadge)
	}); ok {
		b.SetBadge(deviceID, mobile.ViewBadge{
			Country: country,
			Status:  status,
			Label:   label,
		})
	}
}

func (s *Service) pushMobileSyncRoles(masterID string, followers []string) {
	if s.Mobile == nil {
		return
	}
	sr, ok := s.Mobile.Driver().(interface {
		SetSyncRole(string, string, int)
	})
	if !ok {
		return
	}
	if masterID != "" {
		sr.SetSyncRole(masterID, "master", len(followers))
	}
	for _, fid := range followers {
		sr.SetSyncRole(fid, "follower", 0)
	}
}

func (s *Service) clearMobileSyncRoles(masterID string, followers []string) {
	if s.Mobile == nil {
		return
	}
	sr, ok := s.Mobile.Driver().(interface {
		SetSyncRole(string, string, int)
	})
	if !ok {
		return
	}
	if masterID != "" {
		sr.SetSyncRole(masterID, "none", 0)
	}
	for _, fid := range followers {
		sr.SetSyncRole(fid, "none", 0)
	}
}

// handleMasterViewInput forwards live touch, scroll, key, and text events from
// the Master anyprofile-view window to all Sync Followers with anti-detect
// jitter (50..250ms delay + small coordinate offset) directly via control.sock.
func (s *Service) handleMasterViewInput(ctx context.Context, masterID string, ev mobile.ViewInputEvent) {
	st := s.syncHub().Status()
	if !st.Active || st.MasterID != masterID || len(st.Followers) == 0 {
		return
	}
	if s.Mobile == nil {
		return
	}
	rawCtrl, hasRaw := s.Mobile.Driver().(interface {
		SendControlCommands(context.Context, string, ...string) error
	})
	if !hasRaw {
		return
	}
	j := st.Jitter

	pickDelay := func() time.Duration {
		if !j.Enabled {
			return 0
		}
		lo, hi := j.DelayMinMS, j.DelayMaxMS
		if lo <= 0 {
			lo = 50
		}
		if hi < lo {
			hi = max(lo+150, 250)
		}
		ms := lo
		if hi > lo {
			ms += rand.IntN(hi - lo + 1)
		}
		return time.Duration(ms) * time.Millisecond
	}
	pickOffset := func() (float64, float64) {
		if !j.Enabled {
			return 0, 0
		}
		px := j.OffsetPX
		if px <= 0 {
			px = 5
		}
		// Normalize ±px against ~1080 reference dimension so jitter is subtle & safe.
		dx := float64(rand.IntN(2*px+1)-px) / 1080.0
		dy := float64(rand.IntN(2*px+1)-px) / 1920.0
		return dx, dy
	}

	clamp32k := func(v float64) int {
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		return int(v * 32767.0)
	}

	for _, fid := range st.Followers {
		var g followerGesture
		switch ev.Kind {
		case "touch":
			s.smartMu.Lock()
			if s.syncGestures == nil {
				s.syncGestures = make(map[string]followerGesture)
			}
			if ev.Phase == "down" {
				dx, dy := pickOffset()
				g = followerGesture{delay: pickDelay(), dx: dx, dy: dy}
				s.syncGestures[fid] = g
			} else if prev, ok := s.syncGestures[fid]; ok {
				g = prev
				if ev.Phase == "up" {
					delete(s.syncGestures, fid)
				}
			} else {
				g = followerGesture{delay: pickDelay()}
			}
			s.smartMu.Unlock()
		default:
			dx, dy := pickOffset()
			g = followerGesture{delay: pickDelay(), dx: dx, dy: dy}
		}

		go func(fid string, ev mobile.ViewInputEvent, g followerGesture) {
			if g.delay > 0 {
				select {
				case <-time.After(g.delay):
				case <-ctx.Done():
					return
				}
			}
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			switch ev.Kind {
			case "touch":
				if ev.Phase == "up" {
					_ = rawCtrl.SendControlCommands(c, fid, "touch 0")
				} else {
					tx := clamp32k(ev.RX + g.dx)
					ty := clamp32k(ev.RY + g.dy)
					_ = rawCtrl.SendControlCommands(c, fid, fmt.Sprintf("touch 0 1:%d:%d", tx, ty))
				}
			case "scroll":
				tx := clamp32k(ev.RX + g.dx)
				ty1 := clamp32k(ev.RY + g.dy)
				ty2 := clamp32k(ev.RY + g.dy + ev.DY)
				_ = rawCtrl.SendControlCommands(c, fid,
					fmt.Sprintf("touch 0 2:%d:%d", tx, ty1),
					fmt.Sprintf("touch 0 2:%d:%d", tx, ty2),
					"touch 0",
				)
			case "key":
				if ev.Code > 0 {
					_ = rawCtrl.SendControlCommands(c, fid,
						fmt.Sprintf("key %d 1", ev.Code),
						fmt.Sprintf("key %d 0", ev.Code),
					)
				}
			case "text":
				if ev.Text == "" {
					return
				}
				if cmds, ok := asciiEvdevStrokes(ev.Text); ok {
					_ = rawCtrl.SendControlCommands(c, fid, cmds...)
					return
				}
				if ctrl, err := s.MobileControl(fid); err == nil {
					single := strings.ReplaceAll(strings.ReplaceAll(ev.Text, "\r\n", " "), "\n", " ")
					esc := strings.ReplaceAll(single, `\`, `\\`)
					esc = strings.ReplaceAll(esc, `'`, `'\''`)
					esc = strings.ReplaceAll(esc, " ", "%s")
					_, _ = ctrl.ADB.Shell(c, "input text '"+esc+"'")
				}
			}
		}(fid, ev, g)
	}
}

func asciiEvdevStrokes(text string) ([]string, bool) {
	if len(text) > 240 {
		return nil, false
	}
	var cmds []string
	for _, r := range text {
		code, shift, ok := evdevCodeForRune(r)
		if !ok {
			return nil, false
		}
		if shift {
			cmds = append(cmds, "key 42 1")
		}
		cmds = append(cmds, fmt.Sprintf("key %d 1", code), fmt.Sprintf("key %d 0", code))
		if shift {
			cmds = append(cmds, "key 42 0")
		}
	}
	return cmds, len(cmds) > 0
}

func evdevCodeForRune(r rune) (code int, shift bool, ok bool) {
	lower := map[rune]int{
		'a': 30, 'b': 48, 'c': 46, 'd': 32, 'e': 18, 'f': 33, 'g': 34, 'h': 35, 'i': 23,
		'j': 36, 'k': 37, 'l': 38, 'm': 50, 'n': 49, 'o': 24, 'p': 25, 'q': 16, 'r': 19,
		's': 31, 't': 20, 'u': 22, 'v': 47, 'w': 17, 'x': 45, 'y': 21, 'z': 44,
	}
	if c, found := lower[r]; found {
		return c, false, true
	}
	if r >= 'A' && r <= 'Z' {
		if c, found := lower[r+'a'-'A']; found {
			return c, true, true
		}
	}
	plain := map[rune]int{
		'1': 2, '2': 3, '3': 4, '4': 5, '5': 6, '6': 7, '7': 8, '8': 9, '9': 10, '0': 11,
		'-': 12, '=': 13, '[': 26, ']': 27, ';': 39, '\'': 40, '`': 41, '\\': 43,
		',': 51, '.': 52, '/': 53, ' ': 57, '\n': 28, '\t': 15,
	}
	if c, found := plain[r]; found {
		return c, false, true
	}
	shifted := map[rune]int{
		'!': 2, '@': 3, '#': 4, '$': 5, '%': 6, '^': 7, '&': 8, '*': 9, '(': 10, ')': 11,
		'_': 12, '+': 13, '{': 26, '}': 27, ':': 39, '"': 40, '~': 41, '|': 43,
		'<': 51, '>': 52, '?': 53,
	}
	if c, found := shifted[r]; found {
		return c, true, true
	}
	return 0, false, false
}
