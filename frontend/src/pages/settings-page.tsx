// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, engine, onEngineProgress, type EngineProgress } from '../api'
import { t, type Language } from '../i18n'
import { errorText } from '../lib/format'
import type { ThemeChoice } from '../lib/theme'
import { Icon, type IconName } from '../components/icons'
import { PageHead } from '../components/page-head'

const mb = (n: number) => `${Math.round(n / 1048576)} MB`

type Props = {
  status: engine.Status | null
  onEngineChanged: (s: engine.Status) => void
  theme: ThemeChoice
  onTheme: (c: ThemeChoice) => void
  fileVaultOn: boolean
  lang: Language
  onLanguage: (l: Language) => void
}

export function SettingsPage({ status, onEngineChanged, theme, onTheme, fileVaultOn, lang, onLanguage }: Props) {
  const [progress, setProgress] = useState<EngineProgress | null>(null)
  const [error, setError] = useState('')
  useEffect(() => onEngineProgress(setProgress), [])

  const install = async () => {
    setError('')
    setProgress({ done: 0, total: -1 })
    try {
      await Api.InstallEngine()
      onEngineChanged(await Api.EngineStatus())
    } catch (e) {
      setError(errorText(e))
    } finally {
      setProgress(null)
    }
  }
  const pct = progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0
  const themes: [ThemeChoice, string, IconName][] = [['system', t.settings.themeSystem, 'monitor'], ['light', t.settings.themeLight, 'sun'], ['dark', t.settings.themeDark, 'moon']]

  return (
    <>
      <PageHead title={t.settings.title} />
      <div className="pagebody" style={{ maxWidth: 760 }}>
        <section className="card pad">
          <h2>{t.settings.engineTitle}</h2>
          <p className="desc">{t.settings.engineDesc}</p>
          {status && (
            <dl className="facts">
              <dt>{t.settings.version}</dt><dd>Fury {status.version}</dd>
              <dt>{t.settings.chromium}</dt><dd>{status.chromium}</dd>
              <dt>{t.settings.status}</dt>
              <dd>{status.installed
                ? <span className="chip ok"><span className="dot" />{t.settings.installed}</span>
                : <span className="chip">{t.settings.notInstalled}</span>}</dd>
            </dl>
          )}
          {status?.problem && <div className="banner danger"><Icon name="alert" /><span className="grow">{status.problem}</span></div>}
          {progress ? (
            <div className="field">
              <span className="hint">{t.settings.installing} {progress.total > 0 ? `${mb(progress.done)} / ${mb(progress.total)}` : ''}</span>
              <div className="progress" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}><div style={{ width: `${pct}%` }} /></div>
            </div>
          ) : (
            <div className="row">
              <button className={`btn${status?.installed ? '' : ' primary'}`} onClick={install}>
                <Icon name="download" />{status?.installed ? t.settings.reinstall : t.settings.install}
              </button>
            </div>
          )}
          {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
        </section>

        <section className="card pad">
          <h2>{t.settings.themeTitle}</h2>
          <div className="seg" role="group" aria-label={t.settings.themeTitle}>
            {themes.map(([c, label, icon]) => (
              <button key={c} type="button" aria-pressed={theme === c} onClick={() => onTheme(c)}><Icon name={icon} className="icon sm" />{label}</button>
            ))}
          </div>
        </section>

        <section className="card pad">
          <h2>{t.settings.languageTitle}</h2>
          <div className="seg" role="group" aria-label={t.settings.languageTitle}>
            <button type="button" aria-pressed={lang === 'en'} onClick={() => onLanguage('en')}>{t.settings.languageEn}</button>
            <button type="button" aria-pressed={lang === 'vi'} onClick={() => onLanguage('vi')}>{t.settings.languageVi}</button>
          </div>
        </section>

        <section className="card pad">
          <h2>{t.settings.securityTitle}</h2>
          <div className={`banner ${fileVaultOn ? 'ok' : 'warn'}`}><Icon name="shield" />
            <span className="grow">{fileVaultOn ? t.settings.fileVaultOn : t.settings.fileVaultOff}</span></div>
          <p className="desc" style={{ margin: 0 }}>{t.settings.secretsNote}</p>
        </section>
      </div>
    </>
  )
}
