// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/mobile"
)

// AutoUpdateAnyBrowser stages newer builds next to old ones. The launcher
// switches compatible profiles on their next open, not while they are running.
func (s *Service) AutoUpdateAnyBrowser(ctx context.Context) error {
	eng, ok := s.Engines["anybrowser"].(interface {
		engine.Engine
		InstallIfMissing(context.Context, string) (bool, error)
	})
	if !ok || !s.UpdateStatus().Auto {
		return nil
	}
	seen := map[string]bool{}
	rels := eng.Releases()
	for i, r := range rels {
		if r.Catalogue == "" || seen[r.Catalogue] {
			continue
		}
		seen[r.Catalogue] = true
		if eng.Status(ctx, r.Version).Installed {
			continue
		}
		for _, old := range rels[i+1:] {
			if old.Catalogue != r.Catalogue || !eng.Status(ctx, old.Version).Installed {
				continue
			}
			installed, err := eng.InstallIfMissing(ctx, r.Version)
			if err != nil {
				return fmt.Errorf("install AnyBrowser %s: %w", r.Version, err)
			}
			if installed {
				s.notify("engines")
			}
			break
		}
	}
	return nil
}

// AutoUpdateMobile updates existing Mobile installs when no device needs the
// shared files. First-time Mobile installs still ask before downloading Android.
func (s *Service) AutoUpdateMobile(ctx context.Context) error {
	if s.Mobile == nil || !s.UpdateStatus().Auto || s.mobileAllowed() != nil || s.Mobile.Running() != 0 {
		return nil
	}
	drv, ok := s.Mobile.Driver().(interface{ UpdateInstallers() []mobile.Installer })
	if !ok {
		return nil
	}
	pending := func(st mobile.InstallStatus) bool {
		r := st.Runtime
		return r.Latest != "" && r.Installed != "dev" &&
			(r.Installed == "" || engine.NewerVersion(r.Latest, r.Installed) || st.Image.Installed == "")
	}
	for _, in := range drv.UpdateInstallers() {
		if !pending(in.InstallStatus()) {
			continue
		}
		// Pro download links are short-lived. Refresh only when there's work.
		if _, err := s.ProCheck(ctx, true); err != nil {
			return err
		}
		return s.Mobile.InstallWhenIdle(func() error {
			if !s.UpdateStatus().Auto || s.mobileAllowed() != nil {
				return nil
			}
			var installErr error
			for _, in := range drv.UpdateInstallers() {
				st := in.InstallStatus()
				if pending(st) {
					if err := s.installMobile(ctx, in, func(string, int64, int64) {}); err != nil {
						installErr = errors.Join(installErr, fmt.Errorf("install %s %s: %w", st.Runtime.Engine, st.Runtime.Latest, err))
					}
				}
			}
			return installErr
		})
	}
	return nil
}
