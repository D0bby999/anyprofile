// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useState } from 'react'
import { Api, engine, onDataChanged } from './api'
import { language, setLanguage, t, type Language } from './i18n'
import { applyTheme, readTheme, type ThemeChoice } from './lib/theme'
import { useProfiles } from './hooks/use-profiles'
import { useSites } from './hooks/use-sites'
import { Sidebar, type Page, type View } from './components/sidebar'
import { NewProfileDialog } from './components/new-profile-dialog'
import { Icon, LogoMark } from './components/icons'
import { ProfilesPage } from './pages/profiles-page'
import { ProxiesPage } from './pages/proxies-page'
import { TrashPage } from './pages/trash-page'
import { SettingsPage } from './pages/settings-page'

export default function App() {
  const [page, setPage] = useState<Page>('profiles')
  const [view, setView] = useState<View>('all')
  const [creating, setCreating] = useState(false)
  const [startupError, setStartupError] = useState('')
  const [fileVaultOn, setFileVaultOn] = useState(true)
  const [engineStatus, setEngineStatus] = useState<engine.Status | null>(null)
  const [proxyCount, setProxyCount] = useState(0)
  const [theme, setTheme] = useState<ThemeChoice>(readTheme())
  const [lang, setLang] = useState<Language>(language)
  const profiles = useProfiles()
  const { sites } = useSites()

  useEffect(() => applyTheme(theme), [theme])
  useEffect(() => {
    Api.StartupError().then(setStartupError)
    Api.FileVaultOn().then(setFileVaultOn)
    Api.EngineStatus().then(setEngineStatus).catch(() => {})
  }, [])
  const refreshProxies = () => { Api.ListProxies().then((p) => setProxyCount((p ?? []).length)).catch(() => {}) }
  useEffect(refreshProxies, [])
  useEffect(() => onDataChanged(['proxies'], refreshProxies), [])

  const counts = useMemo(() => {
    const c = { all: 0, running: 0 }
    for (const r of profiles.rows) {
      c.all++
      if (r.state !== 'stopped') c.running++
    }
    return c
  }, [profiles.rows])

  const siteCounts = useMemo(() => {
    const m = new Map<string, number>()
    for (const r of profiles.rows) m.set(r.site, (m.get(r.site) ?? 0) + 1)
    return m
  }, [profiles.rows])

  // Site ids ordered by how often the profiles use them, most first.
  const recent = useMemo(() =>
    [...siteCounts.entries()].sort((a, b) => b[1] - a[1]).map(([id]) => id),
  [siteCounts])

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

  const engineReady = !!engineStatus?.installed
  const navigate = (p: Page, v?: View) => { setPage(p); if (v) setView(v) }
  const newProfile = () => setCreating(true)
  const changeLanguage = async (l: Language) => {
    try {
      await Api.SetLanguage(l)
      setLanguage(l)
      setLang(l)
    } catch {
      // the language stays as it was
    }
  }

  return (
    <div className="app" key={lang}>
      <Sidebar page={page} view={view} counts={counts} sites={sites} siteCounts={siteCounts} proxyCount={proxyCount}
        engineReady={engineReady} engineLabel={engineReady ? t.engine.ready(engineStatus!.version) : t.engine.missing}
        onNavigate={navigate} onNewProfile={newProfile} />
      <main className="main">
        {page === 'profiles' && (
          <ProfilesPage rows={profiles.rows} sites={sites} recent={recent} view={view} engineReady={engineStatus === null || engineReady}
            fileVaultOn={fileVaultOn} error={profiles.error} setError={profiles.setError} reload={profiles.reload}
            onNew={newProfile} onNeedEngine={() => setPage('settings')} onClearView={() => setView('all')} />
        )}
        {page === 'proxies' && <ProxiesPage onChanged={() => { refreshProxies(); profiles.reload() }} />}
        {page === 'trash' && <TrashPage onChanged={profiles.reload} />}
        {page === 'settings' && (
          <SettingsPage status={engineStatus} onEngineChanged={setEngineStatus} theme={theme} onTheme={setTheme} fileVaultOn={fileVaultOn}
            lang={lang} onLanguage={changeLanguage} />
        )}
      </main>
      {creating && (
        <NewProfileDialog defaultSite={view.startsWith('site:') ? view.slice(5) : 'other'} sites={sites} recent={recent}
          onClose={() => setCreating(false)}
          onCreated={() => { setCreating(false); setPage('profiles'); profiles.reload(); refreshProxies() }} />
      )}
    </div>
  )
}
