// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState, type ReactNode } from 'react'
import { Api, main, onEngineProgress, service, type EngineProgress } from '../api'
import { t, type Language } from '../i18n'
import { errorText } from '../lib/format'
import { siteById, type Site } from '../lib/sites'
import { Icon, LogoMark } from '../components/icons'
import { InstallProgress } from '../components/install-progress'
import { SitePicker } from '../components/site-picker'
import { useToast } from '../components/toast'
import { BackupRestore } from '../components/backup-settings'

type Props = {
  sites: Site[]
  fileVaultOn: boolean
  lang: Language
  onLanguage: (l: Language) => void
  /** Setup is over; created is the new profile's id, or null when skipped. */
  onDone: (created: string | null) => void
}

type StepState = 'done' | 'active' | 'later'

function Step({ n, title, state, summary, children }: { n: number; title: string; state: StepState; summary?: string; children: ReactNode }) {
  return (
    <section className={`wstep ${state}`} aria-label={`${t.welcome.step(n)}: ${title}`} aria-current={state === 'active' ? 'step' : undefined}>
      <span className="wnum" aria-hidden="true">{state === 'done' ? <Icon name="check" className="icon sm" /> : n}</span>
      <div className="wbody">
        <h2>{title}</h2>
        {state === 'done' && summary && <p className="wsummary">{summary}</p>}
        {state === 'active' && children}
      </div>
    </section>
  )
}

/** First run: install the engine, add a proxy, create a profile. Resumes from the real state. */
export function WelcomePage({ sites, fileVaultOn, lang, onLanguage, onDone }: Props) {
  const toast = useToast()
  const [engine, setEngine] = useState<main.EngineInfo | null>(null)
  const [proxies, setProxies] = useState<service.ProxyRow[] | null>(null)
  const [noProxy, setNoProxy] = useState(false)
  // Proxies that existed before this visit count at once; new ones wait for Continue.
  const [proxyConfirmed, setProxyConfirmed] = useState(false)
  const [askNoProxy, setAskNoProxy] = useState(false)
  const [progress, setProgress] = useState<EngineProgress | null>(null)
  const [text, setText] = useState('')
  const [failed, setFailed] = useState<string[]>([])
  const [busy, setBusy] = useState('')
  const [name, setName] = useState('')
  const [site, setSite] = useState('other')

  const loadEngine = () => Api.Engines().then((list) => setEngine((list ?? []).find((e) => e.default) ?? null)).catch(() => {})
  const loadProxies = () => Api.ListProxies().then((p) => { setProxies(p ?? []); if ((p ?? []).some((x) => x.exitIp)) setProxyConfirmed(true) })
    .catch(() => setProxies([]))
  useEffect(() => { loadEngine(); loadProxies() }, [])
  useEffect(() => onEngineProgress(setProgress), [])

  const engineDone = !!engine?.installed
  const usable = (proxies ?? []).filter((p) => p.exitIp).length
  const proxyDone = noProxy || (proxyConfirmed && usable > 0)
  const state = (n: number): StepState => {
    const done = [engineDone, proxyDone, false][n - 1]
    if (done) return 'done'
    const first = !engineDone ? 1 : !proxyDone ? 2 : 3
    return n === first ? 'active' : 'later'
  }

  const install = async () => {
    if (!engine) return
    setBusy('install')
    setProgress({ engine: engine.engine, done: 0, total: -1 })
    try {
      await Api.InstallEngine(engine.engine)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setProgress(null)
      setBusy('')
      loadEngine()
    }
  }

  const addProxies = async () => {
    setBusy('proxy')
    setFailed([])
    try {
      const res = await Api.ImportProxies(text, '')
      setFailed([
        ...(res.failed ?? []).map((f) => t.proxies.failedLine(f.line, f.err)),
        ...(res.unreachable ?? []).map((label) => `${label}: ${t.errors.proxy_unreachable}`),
      ])
      setText('') // pasted passwords should not linger on screen
      const list = (await Api.ListProxies()) ?? []
      setProxies(list)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy('')
    }
  }

  const create = async () => {
    if (!name.trim() || busy) return
    setBusy('create')
    try {
      const made = await Api.CreateProfiles(service.BulkCreate.createFrom({
        engine: '', baseName: name.trim(), count: 1, proxy: noProxy || usable === 0 ? 'none' : 'auto',
        sites: [site], startUrl: siteById(sites, site).startUrl, notes: '', tags: [],
      }))
      await Api.FinishOnboarding()
      toast.success(t.welcome.created)
      onDone(made?.[0]?.id ?? null)
    } catch (e) {
      toast.error(errorText(e))
      setBusy('')
    }
  }

  const skip = async () => {
    try { await Api.FinishOnboarding() } catch { /* the setting is retried next launch */ }
    onDone(null)
  }

  return (
    <main className="welcome">
      <header className="welcome-top drag">
        <span className="row"><LogoMark /><b>{t.appName}</b></span>
        <div className="seg nodrag" role="group" aria-label={t.settings.languageTitle}>
          <button type="button" aria-pressed={lang === 'en'} onClick={() => onLanguage('en')}>{t.settings.languageEn}</button>
          <button type="button" aria-pressed={lang === 'vi'} onClick={() => onLanguage('vi')}>{t.settings.languageVi}</button>
        </div>
      </header>
      <div className="welcome-col">
        <h1>{t.welcome.title}</h1>
        <p className="welcome-sub">{t.welcome.sub}</p>

        <Step n={1} title={t.welcome.engineTitle} state={state(1)} summary={engine ? t.welcome.engineDone(engine.version) : ''}>
          <p>{t.welcome.engineText}</p>
          {!fileVaultOn && <div className="banner warn"><Icon name="shield" /><span className="grow">{t.welcome.fileVault}</span></div>}
          {progress ? <InstallProgress progress={progress} /> : (
            <button type="button" className="btn primary wbtn" disabled={!engine || !!busy} onClick={install}>
              <Icon name="download" />{t.welcome.install}
            </button>
          )}
        </Step>

        <Step n={2} title={t.welcome.proxyTitle} state={state(2)}
          summary={noProxy ? t.welcome.noProxyDone : t.welcome.proxyDone(usable)}>
          <p>{t.welcome.proxyText}</p>
          <textarea className="textarea code" rows={3} value={text} placeholder={t.welcome.proxyPlaceholder} spellCheck={false}
            autoComplete="off" aria-label={t.welcome.proxyTitle} onChange={(e) => setText(e.target.value)} />
          {proxies && proxies.length > 0 && (
            <ul className="wproxies">
              {proxies.map((p) => (
                <li key={p.id}>{p.exitIp ? [p.country, p.exitIp, p.timezone].filter(Boolean).join(' · ') : `${p.label} · ${t.welcome.notChecked}`}</li>
              ))}
            </ul>
          )}
          {failed.length > 0 && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{failed.map((f) => <div key={f}>{f}</div>)}</span></div>}
          <div className="row">
            {proxies && proxies.some((p) => p.exitIp) && (
              <button type="button" className="btn primary" onClick={() => setProxyConfirmed(true)}>{t.welcome.continue}</button>
            )}
            <button type="button" className={`btn${proxies?.length ? '' : ' primary'}${busy === 'proxy' ? ' busy' : ''}`} aria-busy={busy === 'proxy'}
              disabled={!text.trim() || !!busy} onClick={addProxies}>
              <Icon name={busy === 'proxy' ? 'refresh' : 'plus'} />{busy === 'proxy' ? t.welcome.checking : t.welcome.checkAdd}
            </button>
            {!askNoProxy && <button type="button" className="btn ghost" onClick={() => setAskNoProxy(true)}>{t.welcome.skipProxy}</button>}
          </div>
          {askNoProxy && (
            <div className="banner warn"><Icon name="alert" /><span className="grow">{t.welcome.noProxyWarn}</span>
              <button type="button" className="btn sm" onClick={() => setNoProxy(true)}>{t.welcome.continueAnyway}</button></div>
          )}
        </Step>

        <Step n={3} title={t.welcome.profileTitle} state={state(3)}>
          <label className="field">
            <span className="label">{t.create.name}</span>
            <input className="input" autoFocus value={name} placeholder={t.welcome.namePlaceholder}
              onChange={(e) => setName(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') create() }} />
          </label>
          <div className="field">
            <span className="label">{t.create.site}</span>
            <SitePicker value={site} onChange={setSite} sites={sites} />
          </div>
          <p className="wnote"><Icon name="key" className="icon sm" />{t.welcome.keychain}</p>
          <button type="button" className="btn primary wbtn" disabled={!name.trim() || !!busy} onClick={create}>
            {busy === 'create' ? t.welcome.creating : t.welcome.create}
          </button>
        </Step>

        {state(1) === 'done' && (
          <details className="wrestore">
            <summary>{t.backup.welcome}</summary>
            <BackupRestore onRestored={(r) => { if (r.profiles > 0) onDone(null) }} />
          </details>
        )}

        <button type="button" className="welcome-skip" onClick={skip}>{t.welcome.skip}</button>
      </div>
    </main>
  )
}
