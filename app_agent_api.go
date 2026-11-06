// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/D0bby999/anyprofile/internal/api"
	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/launcher"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

// The profile-management tools an agent gets when Settings lets it change
// profiles. They sit on the same service the UI calls, so a change reaches the
// table, the log and the running browsers at once, with no extra surface.

func (b apiBackend) Status(ctx context.Context) (api.AppStatus, error) {
	live, err := b.svc.LiveProfiles()
	if err != nil {
		return api.AppStatus{}, err
	}
	trashed, err := b.svc.ListProfiles(store.ProfileFilter{Trashed: true})
	if err != nil {
		return api.AppStatus{}, err
	}
	st := api.AppStatus{Profiles: len(live), Trashed: len(trashed), MaxConcurrent: b.svc.MaxConcurrentLaunches()}
	for _, r := range live {
		if r.State == launcher.Running || r.State == launcher.Starting {
			st.Open++
		}
	}
	if v, _ := b.svc.Store.GetSetting("update_auto"); v != "0" {
		st.UpdateAuto = true
	}
	if b.svc.Mobile != nil {
		if ds, err := b.svc.ListMobileDevices(store.ProfileFilter{}); err == nil {
			st.Devices = len(ds)
		}
	}
	ids := make([]string, 0, len(b.svc.Engines))
	for id := range b.svc.Engines {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		status := b.svc.EngineState(ctx, b.svc.Engines[id])
		st.Engines = append(st.Engines, api.AppEngine{ID: id, Version: status.Version.Version, Installed: status.Installed})
	}
	if fs, _, err := b.svc.ListFolders(); err == nil {
		for _, f := range fs {
			st.Folders = append(st.Folders, api.AppFolder{ID: f.ID, Name: f.Name})
		}
	}
	if es, err := b.svc.ListExtensions(); err == nil {
		for _, e := range es {
			st.Extensions = append(st.Extensions, api.AppExtension{ID: e.ID, Name: e.Name, Profiles: e.Profiles})
		}
	}
	return st, nil
}

func (b apiBackend) Proxies() ([]api.ProxyInfo, error) {
	rows, err := b.svc.ListProxies()
	if err != nil {
		return nil, err
	}
	out := make([]api.ProxyInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, proxyInfo(r))
	}
	return out, nil
}

func proxyInfo(r service.ProxyRow) api.ProxyInfo {
	return api.ProxyInfo{
		ID: r.ID, Label: r.Label, Scheme: r.Scheme, Host: r.Host, Port: r.Port,
		Country: r.Country, ExitIP: r.ExitIP, CheckedAt: r.CheckedAt, FailedAt: r.FailedAt,
		Assignable: r.Assignable, UsedBy: len(r.UsedBy),
	}
}

func (b apiBackend) CheckProxy(ctx context.Context, id string) (api.ProxyInfo, error) {
	if _, err := b.svc.CheckProxyByID(ctx, id); err != nil {
		return api.ProxyInfo{}, err
	}
	rows, err := b.svc.ListProxies()
	if err != nil {
		return api.ProxyInfo{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return proxyInfo(r), nil
		}
	}
	return api.ProxyInfo{}, apperr.Newf("proxy_not_found", "no proxy %q; see proxy_list", id).Never()
}

func (b apiBackend) CreateProfiles(ctx context.Context, in api.AppCreate) ([]api.AppProfile, error) {
	ok, err := b.folderExists(in.FolderID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.Newf("folder_not_found", "no folder %q; app_status lists them", in.FolderID).Never()
	}
	proxy := in.Proxy
	if proxy == "" {
		proxy = service.AutoProxy
	}
	ps, err := b.svc.CreateProfiles(ctx, service.BulkCreate{
		Engine: in.Engine, BaseName: in.BaseName, Count: in.Count, Proxy: proxy,
		Sites: in.Sites, StartURL: in.StartURL, Notes: in.Notes, Tags: in.Tags,
	})
	if err != nil {
		return nil, err
	}
	if in.FolderID != "" {
		ids := make([]string, len(ps))
		for i, p := range ps {
			ids[i] = p.ID
		}
		if _, err := b.svc.MoveProfilesToFolder(ids, in.FolderID); err != nil {
			return nil, err
		}
		for i := range ps {
			ps[i].FolderID = in.FolderID
		}
	}
	out := make([]api.AppProfile, 0, len(ps))
	for _, p := range ps {
		out = append(out, b.appProfile(p))
	}
	return out, nil
}

func (b apiBackend) PatchProfile(ctx context.Context, ref string, p api.AppPatch) (api.AppProfile, error) {
	cur, err := b.resolveProfile(ref, false)
	if err != nil {
		return api.AppProfile{}, err
	}
	if p.FolderID != nil {
		ok, err := b.folderExists(*p.FolderID)
		if err != nil {
			return api.AppProfile{}, err
		}
		if !ok {
			return api.AppProfile{}, apperr.Newf("folder_not_found", "no folder %q; app_status lists them", *p.FolderID).Never()
		}
	}
	got, err := b.svc.PatchProfile(ctx, cur.ID, service.ProfilePatch{
		Name: p.Name, Sites: p.Sites, Tags: p.Tags, FolderID: p.FolderID,
		Notes: p.Notes, StartURL: p.StartURL, ProxyID: p.ProxyID,
	})
	if err != nil {
		return api.AppProfile{}, err
	}
	return b.appProfile(got), nil
}

func (b apiBackend) Trash(ctx context.Context, ref string) error {
	p, err := b.resolveProfile(ref, false)
	if err != nil {
		return err
	}
	return b.svc.Trash(p.ID)
}

func (b apiBackend) Restore(ctx context.Context, ref string) (api.AppProfile, error) {
	p, err := b.resolveProfile(ref, true)
	if err != nil {
		return api.AppProfile{}, err
	}
	if err := b.svc.Restore(p.ID); err != nil {
		return api.AppProfile{}, err
	}
	got, err := b.svc.Store.GetProfile(p.ID)
	if err != nil {
		return api.AppProfile{}, err
	}
	return b.appProfile(got), nil
}

func (b apiBackend) Purge(ctx context.Context, ref, confirm string) error {
	p, err := b.resolveProfile(ref, true)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != p.Name {
		return apperr.Newf("confirm_required", "confirm must be the profile's exact name, %q", p.Name).Never()
	}
	return b.svc.Purge(p.ID)
}

func (b apiBackend) AssignProxy(ctx context.Context, ref, proxyID string) (api.AppProfile, error) {
	p, err := b.resolveProfile(ref, false)
	if err != nil {
		return api.AppProfile{}, err
	}
	got, err := b.svc.AssignProxy(ctx, p.ID, proxyID)
	if err != nil {
		return api.AppProfile{}, err
	}
	return b.appProfile(got), nil
}

func (b apiBackend) SetExtension(ctx context.Context, profileIDs []string, extensionID string, on bool) (int, error) {
	ids := make([]string, 0, len(profileIDs))
	for _, ref := range profileIDs {
		p, err := b.resolveProfile(ref, false)
		if err != nil {
			return 0, err
		}
		ids = append(ids, p.ID)
	}
	if err := b.svc.SetExtensionMany(ids, extensionID, on); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// folderExists says whether id names a folder; "" is the unfiled case.
func (b apiBackend) folderExists(id string) (bool, error) {
	if id == "" {
		return true, nil
	}
	fs, _, err := b.svc.ListFolders()
	if err != nil {
		return false, err
	}
	for _, f := range fs {
		if f.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// appProfile is the shape the tools report: what the table shows about a
// profile, never its identity seed or its saved accounts.
func (b apiBackend) appProfile(p store.Profile) api.AppProfile {
	return api.AppProfile{
		ID: p.ID, Serial: p.Serial, Name: p.Name, Engine: p.Engine, ProxyID: p.ProxyID,
		FolderID: p.FolderID, Sites: p.Sites, Tags: p.Tags, StartURL: p.StartURL,
		Notes: p.Notes, State: string(b.svc.ProfileState(p.ID).State),
	}
}

// resolveProfile finds a profile by id, exact name or #serial; trashed says
// which list to look in.
func (b apiBackend) resolveProfile(ref string, trashed bool) (store.Profile, error) {
	rows, err := b.svc.ListProfiles(store.ProfileFilter{Trashed: trashed})
	if err != nil {
		return store.Profile{}, err
	}
	ref = strings.TrimSpace(ref)
	serial, serr := strconv.Atoi(strings.TrimPrefix(ref, "#"))
	var hits []store.Profile
	for _, r := range rows {
		if r.ID == ref || (serr == nil && r.Serial == serial) {
			return r.Profile, nil
		}
		if strings.EqualFold(r.Name, ref) {
			hits = append(hits, r.Profile)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		where := "list_profiles"
		if trashed {
			where = "the trash"
		}
		return store.Profile{}, apperr.Newf("api_not_found", "no profile %q in %s", ref, where)
	default:
		names := make([]string, 0, len(hits))
		for _, p := range hits {
			names = append(names, "#"+strconv.Itoa(p.Serial)+" "+p.Name)
		}
		return store.Profile{}, apperr.Newf("agent_ambiguous", "%q matches %d profiles: %s", ref, len(hits), strings.Join(names, ", ")).Never()
	}
}
