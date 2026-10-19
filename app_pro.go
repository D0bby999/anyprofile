// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"os"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// How long Settings → Pro waits for a purchase to be paid, and how often it asks.
const (
	proWaitFor   = 30 * time.Minute
	proWaitEvery = 5 * time.Second
)

func (a *App) proStatus() service.ProStatus {
	st := a.svc.ProStatus()
	a.proMu.Lock()
	st.Waiting = a.proWait != nil
	a.proMu.Unlock()
	return st
}

// checkPro is the daily check, from the update loop; it says nothing when
// all is well and logs a failure.
func (a *App) checkPro() {
	before := a.svc.ProStatus().Version
	st, err := a.svc.ProCheck(context.Background(), false)
	if err != nil {
		a.svc.Log.Warn("pro check", "err", err)
		return
	}
	if st.Version != before && st.Version != "" {
		runtime.EventsEmit(a.ctx, "pro:updated", st.Version)
	}
}

func (a *App) ProStatus() (service.ProStatus, error) {
	if err := a.ready(); err != nil {
		return service.ProStatus{}, err
	}
	return a.proStatus(), nil
}

func (a *App) ProActivate(key string) (service.ProStatus, error) {
	if err := a.ready(); err != nil {
		return service.ProStatus{}, err
	}
	_, err := a.svc.ProActivate(a.ctx, key)
	return a.proStatus(), err
}

// ProCheck asks the server now, as "Check for updates" in Settings → Pro.
func (a *App) ProCheck() (service.ProStatus, error) {
	if err := a.ready(); err != nil {
		return service.ProStatus{}, err
	}
	_, err := a.svc.ProCheck(a.ctx, true)
	return a.proStatus(), err
}

// purchaseWait is one wait for a payment; a pointer, so a finished wait
// can tell whether a newer one replaced it.
type purchaseWait struct{ cancel context.CancelFunc }

// ProBuy opens the purchase page in the default browser and waits for the
// payment, activating the key it brings. A second Buy replaces the wait.
func (a *App) ProBuy(plan string) error {
	if err := a.ready(); err != nil {
		return err
	}
	u, ref, err := a.svc.ProBuyURL(plan)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), proWaitFor)
	w := &purchaseWait{cancel}
	a.proMu.Lock()
	if a.proWait != nil {
		a.proWait.cancel()
	}
	a.proWait = w
	a.proMu.Unlock()
	runtime.BrowserOpenURL(a.ctx, u)
	runtime.EventsEmit(a.ctx, "data:changed", "pro")
	go a.waitForPurchase(ctx, w, ref)
	return nil
}

func (a *App) waitForPurchase(ctx context.Context, w *purchaseWait, ref string) {
	defer func() {
		w.cancel()
		a.proMu.Lock()
		if a.proWait == w {
			a.proWait = nil
		}
		a.proMu.Unlock()
		runtime.EventsEmit(a.ctx, "data:changed", "pro")
	}()
	tick := time.NewTicker(proWaitEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		done, err := a.svc.ProClaim(ctx, ref)
		if err != nil {
			a.svc.Log.Warn("pro claim", "err", err)
		}
		if done {
			runtime.EventsEmit(a.ctx, "pro:activated", a.svc.ProStatus().Version)
			return
		}
	}
}

// ProCancelBuy stops waiting for a purchase; a payment made later still
// works with the key from the mail.
func (a *App) ProCancelBuy() error {
	a.proMu.Lock()
	if a.proWait != nil {
		a.proWait.cancel()
		a.proWait = nil
	}
	a.proMu.Unlock()
	runtime.EventsEmit(a.ctx, "data:changed", "pro")
	return nil
}

// ProDeactivate frees this computer's place and removes the key and the pack.
func (a *App) ProDeactivate() error {
	return a.must(func() error {
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()
		return a.svc.ProDeactivate(ctx)
	})
}

// ProOpenPortal opens Stripe's page for the subscription in the default browser.
func (a *App) ProOpenPortal() error {
	return a.must(func() error {
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()
		u, err := a.svc.ProPortalURL(ctx)
		if err != nil {
			return err
		}
		runtime.BrowserOpenURL(a.ctx, u)
		return nil
	})
}

// ProOpenScripts shows the Pro pack's scripts folder.
func (a *App) ProOpenScripts() error {
	return a.must(func() error {
		dir := a.svc.ProScripts()
		if dir == "" {
			return apperr.New("pro_no_pack", "the Pro pack is not installed yet")
		}
		if _, err := os.Stat(dir); err != nil {
			return err
		}
		return revealFolder(dir)
	})
}
