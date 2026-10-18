// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState } from 'react'
import { Api, main, onDataChanged, onMenu, service } from './api'
import { language, setLanguage, t, type Language } from './i18n'
import { applyTheme, readTheme, type ThemeChoice } from './lib/theme'
import { useProfiles } from './hooks/use-profiles'
import { countProfiles } from './lib/health'
import { tagCounts } from './lib/tags'
import { useSites } from './hooks/use-sites'
import { Sidebar, type Page, type View } from './components/sidebar'
import { NewProfileDialog } from './components/new-profile-dialog'
import { Icon, LogoMark } from './components/icons'
import { ProfilesPage } from './pages/profiles-page'
import { ProxiesPage } from './pages/proxies-page'
import { TrashPage } from './pages/trash-page'
import { ToolsPage } from './pages/tools-page'
import { SettingsPage } from './pages/settings-page'
import { WelcomePage } from './pages/welcome-page'
import { CommandPalette, type Command } from './components/command-palette'
import { ShortcutSheet } from './components/shortcut-sheet'
import { useToast } from './components/toast'
import { Dialog } from './components/dialog'
import { registerShortcuts } from './lib/shortcuts'
import { errorText } from './lib/format'

export default function App() {
  const [page, setPage] = useState<Page>('profiles')
  const [view, setView] = useState<View>('all')
  const [creating, setCreating] = useState<number | false>(false) // false, or the count to start with
  const [activeId, setActiveId] = useState<string | null>(null)
  const [onboarded, setOnboarded] = useState<boolean | null>(null)
  const [palette, setPalette] = useState(false)
  const [sheet, setSheet] = useState(false)
  const [whatsNew, setWhatsNew] = useState<service.WhatsNew | null>(null)
  const [checkUpdate, setCheckUpdate] = useState(0) // bumped by the menu bar's Check for Updates
  const toast = useToast()
  const [startupError, setStartupError] = useState('')
  const [fileVaultOn, setFileVaultOn] = useState(true)
  const [engines, setEngines] = useState<main.EngineInfo[] | null>(null)
  const [proxyCount, setProxyCount] = useState(0)
  const [theme, setTheme] = useState<ThemeChoice>(readTheme())
  const [lang, setLang] = useState<Language>(language)
  const [defaults, setDefaults] = useState<service.Defaults>(service.Defaults.createFrom({ site: 'other', proxy: 'auto', onOpen: 'start' }))
  const profiles = useProfiles()
  const { sites } = useSites()

  useEffect(() => applyTheme(theme), [theme])
  useEffect(() => {
    Api.StartupError().then(setStartupError)
    Api.Onboarded().then(setOnboarded).catch(() => setOnboarded(true))
    Api.FileVaultOn().then(setFileVaultOn)
  }, [])
  const refreshEngines = () => {
    Api.Defaults().then(setDefaults).catch(() => {})
    Api.Engines().then(setEngines).catch(() => {})
  }
  useEffect(refreshEngines, [])
  useEffect(() => onDataChanged(['settings'], refreshEngines), [])
  const refreshProxies = () => { Api.ListProxies().then((p) => setProxyCount((p ?? []).length)).catch(() => {}) }
  useEffect(refreshProxies, [])
  useEffect(() => onDataChanged(['proxies'], refreshProxies), [])

  const counts = useMemo(() => countProfiles(profiles.rows), [profiles.rows])

  const focusSearch = () => {
    setPage('profiles')
    setTimeout(() => document.getElementById('profile-search')?.focus(), 0)
  }
  // Once, the first time a new version runs.
  useEffect(() => { Api.TakeWhatsNew().then((w) => { if (w.version) setWhatsNew(w) }).catch(() => {}) }, [])
  useEffect(() => onMenu((what) => {
    setPage('settings')
    if (what === 'check-update') setCheckUpdate((n) => n + 1)
  }), [])
  // App-wide shortcuts; the profiles page adds its own for the table.
  useEffect(() => registerShortcuts([
    { combo: 'mod+k', global: true, run: () => setPalette(true) },
    { combo: 'mod+n', run: () => setCreating(1) },
    { combo: 'mod+f', run: focusSearch },
    { combo: '/', run: focusSearch },
    { combo: '?', run: () => setSheet(true) },
  ]), [])
  const tags = useMemo(() => tagCounts(profiles.rows), [profiles.rows])

  const engineCounts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const r of profiles.rows) c[r.engine] = (c[r.engine] ?? 0) + 1
    return c
  }, [profiles.rows])

  const siteCounts = useMemo(() => {
    const m = new Map<string, number>()
    // A profile on several platforms counts under each of them.
    for (const r of profiles.rows) for (const id of r.sites) m.set(id, (m.get(id) ?? 0) + 1)
    return m
  }, [profiles.rows])

  if (startupError) {
    return (
      <main className="app" style={{ alignItems: 'center', justifyContent: 'center' }}>
        <section className="card pad drag" style={{ width: 520 }}>
          <span className="row"><LogoMark /><b>{t.appName}</b></span>
          <h2>{t.system.startupFailed}</h2>
          <div className="banner danger"><Icon name="alert" /><span className="grow selectable">{startupError}</span></div>
        </section>
      </main>
    )
  }

  const changeLanguage = async (l: Language) => {
    try {
      await Api.SetLanguage(l)
      setLanguage(l)
      setLang(l)
    } catch {
      // the language stays as it was
    }
  }

  if (onboarded === null) return null
  if (!onboarded) {
    return (
      // Not keyed by language: switching it must keep the setup progress.
      <div className="app">
        <WelcomePage sites={sites} fileVaultOn={fileVaultOn} lang={lang} onLanguage={changeLanguage}
          onDone={(id) => { setOnboarded(true); setPage('profiles'); setView('all'); setActiveId(id); profiles.reload(); refreshEngines(); refreshProxies() }} />
      </div>
    )
  }

  const defaultEngine = engines?.find((e) => e.default)
  const engineReady = !!defaultEngine?.installed
  const engineName = defaultEngine ? (t.engines.names[defaultEngine.engine] ?? defaultEngine.engine) : ''
  const navigate = (p: Page, v?: View) => { setPage(p); if (v) setView(v) }
  const newProfile = () => setCreating(1)
  const cycleTheme = () => setTheme(theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system')
  const actions: Command[] = [
    { id: 'new', label: t.palette.newProfile, icon: 'plus', run: newProfile },
    { id: 'several', label: t.palette.createSeveral, icon: 'plus', run: () => setCreating(5) },
    { id: 'proxies', label: t.palette.addProxies, icon: 'proxy', run: () => { setPage('proxies'); setTimeout(() => document.getElementById('proxy-paste')?.focus(), 0) } },
    { id: 'check', label: t.palette.checkProxies, icon: 'refresh', run: () => { toast.info(t.palette.checking); Api.CheckAllProxies().catch((e) => toast.error(errorText(e))) } },
    { id: 'settings', label: t.palette.openSettings, icon: 'settings', run: () => setPage('settings') },
    { id: 'theme', label: t.palette.switchTheme, icon: 'moon', run: cycleTheme },
    { id: 'language', label: t.palette.switchLanguage, icon: 'settings', run: () => changeLanguage(lang === 'en' ? 'vi' : 'en') },
    { id: 'keys', label: t.palette.shortcuts, icon: 'settings', run: () => setSheet(true) },
  ]
  const places: Command[] = [
    { id: 'all', label: t.nav.all, icon: 'profiles', run: () => navigate('profiles', 'all') },
    { id: 'running', label: t.nav.running, icon: 'running', run: () => navigate('profiles', 'running') },
    { id: 'attention', label: t.nav.attention, icon: 'alert', run: () => navigate('profiles', 'attention') },
    { id: 'proxies', label: t.nav.proxies, icon: 'proxy', run: () => setPage('proxies') },
    { id: 'trash', label: t.nav.trash, icon: 'trash', run: () => setPage('trash') },
    { id: 'tools', label: t.nav.tools, icon: 'spark', run: () => setPage('tools') },
    { id: 'settings', label: t.nav.settings, icon: 'settings', run: () => setPage('settings') },
  ]
  const openProfile = (id: string) => { setPage('profiles'); setActiveId(id) }
  const launchProfile = (row: service.ProfileRow) => {
    const go = row.state === 'running' ? Api.FocusProfile(row.id) : Api.StartProfile(row.id)
    Promise.resolve(go).catch((e: unknown) => toast.error(errorText(e)))
  }
  return (
    <div className="app" key={lang}>
      <Sidebar page={page} view={view} counts={counts} tags={tags} sites={sites} siteCounts={siteCounts} proxyCount={proxyCount}
        engineReady={engineReady}
        engineLabel={defaultEngine ? (engineReady ? t.engines.ready(engineName, defaultEngine.version) : t.engines.missing(engineName)) : ''}
        onNavigate={navigate} onNewProfile={newProfile} />
      <main className="main">
        {page === 'profiles' && (
          <ProfilesPage rows={profiles.rows} sites={sites} view={view} engineReady={engines === null || engineReady}
            fileVaultOn={fileVaultOn} error={profiles.error} setError={profiles.setError} reload={profiles.reload}
            onNew={newProfile} onNeedEngine={() => setPage('settings')} onOpenSettings={() => setPage('settings')} onClearView={() => setView('all')} onView={setView}
            activeId={activeId} setActiveId={setActiveId} />
        )}
        {page === 'proxies' && <ProxiesPage onChanged={() => { refreshProxies(); profiles.reload() }} />}
        {page === 'trash' && <TrashPage onChanged={profiles.reload} />}
        {page === 'tools' && <ToolsPage />}
        {page === 'settings' && (
          <SettingsPage engines={engines ?? []} engineCounts={engineCounts} defaults={defaults} onEnginesChanged={refreshEngines} theme={theme} onTheme={setTheme} fileVaultOn={fileVaultOn}
            lang={lang} onLanguage={changeLanguage} openProfiles={profiles.rows.filter((r) => r.state === 'running').length} checkUpdate={checkUpdate} />
        )}
      </main>
      {whatsNew && (
        <Dialog title={t.update.whatsNew(whatsNew.version)} icon="spark" onClose={() => setWhatsNew(null)}
          footer={<button type="button" className="btn primary" autoFocus onClick={() => setWhatsNew(null)}>{t.update.gotIt}</button>}>
          <p className="updnotes selectable">{whatsNew.notes}</p>
        </Dialog>
      )}
      {palette && (
        <CommandPalette profiles={profiles.rows} actions={actions} places={places} onOpenProfile={openProfile}
          onLaunchProfile={launchProfile} onClose={() => setPalette(false)} />
      )}
      {sheet && <ShortcutSheet onClose={() => setSheet(false)} />}
      {creating !== false && (
        <NewProfileDialog initialCount={creating} defaultSite={view.startsWith('site:') ? view.slice(5) : defaults.site} defaults={defaults} sites={sites}
          onClose={() => setCreating(false)}
          onCreated={() => { setCreating(false); setPage('profiles'); profiles.reload(); refreshProxies() }} />
      )}
    </div>
  )
}
