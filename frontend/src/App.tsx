// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useState } from 'react'
import { Api, engine } from './api'
import { language, setLanguage, t, type Language } from './i18n'
import { applyTheme, readTheme, type ThemeChoice } from './lib/theme'
import { useProfiles } from './hooks/use-profiles'
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

  useEffect(() => applyTheme(theme), [theme])
  useEffect(() => {
    Api.StartupError().then(setStartupError)
    Api.FileVaultOn().then(setFileVaultOn)
    Api.EngineStatus().then(setEngineStatus).catch(() => {})
  }, [])
  const refreshProxies = () => { Api.ListProxies().then((p) => setProxyCount((p ?? []).length)).catch(() => {}) }
  useEffect(refreshProxies, [])

  const counts = useMemo(() => {
    const c: Record<View, number> = { all: 0, running: 0, tiktok: 0, youtube: 0, other: 0 }
    for (const r of profiles.rows) {
      c.all++
      if (r.state !== 'stopped') c.running++
      if (r.channelKind in c) c[r.channelKind as View]++
    }
    return c
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
      <Sidebar page={page} view={view} counts={counts} proxyCount={proxyCount}
        engineReady={engineReady} engineLabel={engineReady ? t.engine.ready(engineStatus!.version) : t.engine.missing}
        onNavigate={navigate} onNewProfile={newProfile} />
      <main className="main">
        {page === 'profiles' && (
          <ProfilesPage rows={profiles.rows} view={view} engineReady={engineStatus === null || engineReady}
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
        <NewProfileDialog defaultChannel={view === 'tiktok' || view === 'youtube' ? view : 'tiktok'}
          onClose={() => setCreating(false)}
          onCreated={() => { setCreating(false); setPage('profiles'); profiles.reload(); refreshProxies() }} />
      )}
    </div>
  )
}
