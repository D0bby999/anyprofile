// SPDX-License-Identifier: AGPL-3.0-or-later
import { main, service } from '../api'
import { t, type Language } from '../i18n'
import type { ThemeChoice } from '../lib/theme'
import { useSites } from '../hooks/use-sites'
import { Icon, type IconName } from '../components/icons'
import { SiteSettings } from '../components/site-settings'
import { StorageSettings } from '../components/storage-settings'
import { BackupSettings } from '../components/backup-settings'
import { DefaultsSettings, OnOpenSettings } from '../components/defaults-settings'
import { UpdateSettings } from '../components/update-settings'
import { SupportSettings } from '../components/support-settings'
import { DonateSettings } from '../components/donate-settings'
import { shownDonateLinks } from '../lib/donate'
import { APISettings } from '../components/api-settings'
import { ExtensionSettings } from '../components/extension-settings'
import { PageHead } from '../components/page-head'
import { EngineSettings } from '../components/engine-settings'


type Props = {
  engines: main.EngineInfo[]
  engineCounts: Record<string, number>
  /** Browsers open now; an update waits until they are closed. */
  openProfiles: number
  defaults: service.Defaults
  onEnginesChanged: () => void
  theme: ThemeChoice
  onTheme: (c: ThemeChoice) => void
  fileVaultOn: boolean
  lang: Language
  onLanguage: (l: Language) => void
  /** Bumped by the menu bar's Check for Updates. */
  checkUpdate?: number
}

export function SettingsPage({ engines, engineCounts, defaults, onEnginesChanged, theme, onTheme, fileVaultOn, lang, onLanguage, openProfiles, checkUpdate }: Props) {
  const { sites, reload: reloadSites } = useSites()
  const themes: [ThemeChoice, string, IconName][] = [['system', t.settings.themeSystem, 'monitor'], ['light', t.settings.themeLight, 'sun'], ['dark', t.settings.themeDark, 'moon']]

  return (
    <>
      <PageHead title={t.settings.title} narrow />
      <div className="pagebody narrow">
        <section className="card pad">
          <h2>{t.engines.title}</h2>
          <EngineSettings engines={engines} counts={engineCounts} defaults={defaults} onChanged={onEnginesChanged} />
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
          <h2>{t.onOpen.title}</h2>
          <OnOpenSettings />
        </section>

        <section className="card pad">
          <h2>{t.defaults.title}</h2>
          <DefaultsSettings />
        </section>

        <section className="card pad">
          <h2>{t.sites.customTitle}</h2>
          <p className="desc">{t.sites.customDesc}</p>
          <SiteSettings sites={sites} reload={reloadSites} />
        </section>

        <section className="card pad">
          <h2>{t.extensions.title}</h2>
          <ExtensionSettings />
        </section>

        <section className="card pad" id="settings-update">
          <h2>{t.update.title}</h2>
          <UpdateSettings openProfiles={openProfiles} checkToken={checkUpdate} />
        </section>

        <section className="card pad">
          <h2>{t.api.title}</h2>
          <APISettings />
        </section>

        <section className="card pad" id="settings-backup">
          <h2>{t.backup.title}</h2>
          <BackupSettings />
        </section>

        <section className="card pad">
          <h2>{t.storage.title}</h2>
          <StorageSettings />
        </section>

        <section className="card pad">
          <h2>{t.settings.securityTitle}</h2>
          <div className={`banner ${fileVaultOn ? 'ok' : 'warn'}`}><Icon name="shield" />
            <span className="grow">{fileVaultOn ? t.settings.fileVaultOn : t.settings.fileVaultOff}</span></div>
          <p className="desc" style={{ margin: 0 }}>{t.settings.secretsNote}</p>
        </section>

        {shownDonateLinks().length > 0 && (
          <section className="card pad">
            <h2>{t.donate.title}</h2>
            <p className="desc">{t.donate.desc}</p>
            <DonateSettings />
          </section>
        )}

        <section className="card pad">
          <h2>{t.support.title}</h2>
          <SupportSettings />
        </section>
      </div>
    </>
  )
}
