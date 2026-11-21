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
	for _, in := range mobileUpdateInstallers(s.Mobile.Driver(), nil) {
		if !in.InstallStatus().CanUpdate {
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
			return s.updateMobileEngines(ctx, nil, func(string, int64, int64) {})
		})
	}
	return nil
}

func mobileUpdateInstallers(drv mobile.Driver, fallback mobile.Installer) []mobile.Installer {
	if multiple, ok := drv.(interface{ UpdateInstallers() []mobile.Installer }); ok {
		if installers := multiple.UpdateInstallers(); len(installers) > 0 {
			return installers
		}
	}
	if fallback != nil {
		return []mobile.Installer{fallback}
	}
	return nil
}

func (s *Service) updateMobileEngines(ctx context.Context, fallback mobile.Installer, progress func(string, int64, int64)) error {
	var installErr error
	attempted := false
	for _, in := range mobileUpdateInstallers(s.Mobile.Driver(), fallback) {
		st := in.InstallStatus()
		if st.CanUpdate {
			attempted = true
			if err := s.installMobile(ctx, in, progress); err != nil {
				installErr = errors.Join(installErr, fmt.Errorf("install %s %s: %w", st.Runtime.Engine, st.Runtime.Latest, err))
			}
		}
	}
	if fallback != nil && !attempted {
		return s.installMobile(ctx, fallback, progress)
	}
	return installErr
}
