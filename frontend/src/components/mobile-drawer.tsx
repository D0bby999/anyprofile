// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { useConfirm } from './confirm'
import { useToast } from './toast'
import { Icon } from './icons'
import { InlineName } from './drawer/inline-name'
import { NotesGroup } from './drawer/notes-group'
import { TagsGroup } from './drawer/tags-group'
import { ProxySelect, RealIpWarning } from './proxy-select'
import { MOBILE_APPS_GUIDE } from './api-settings'

type Tab = 'overview' | 'hardware' | 'activity'
const TABS: Tab[] = ['overview', 'hardware', 'activity']

const RAM_CHOICES = [2048, 3072, 4096, 6144]
const CPU_CHOICES = [2, 4, 6, 8]
const PRESETS = {
  automation: { ram: 2048, cpus: 2, screenW: 540, screenH: 960, density: 240 },
  balanced: { ram: 3072, cpus: 4, screenW: 720, screenH: 1600, density: 280 },
  interactive: { ram: 4096, cpus: 4, screenW: 720, screenH: 1600, density: 280 },
} as const
type Preset = keyof typeof PRESETS

const busyStates = ['provisioning', 'starting', 'stopping', 'working']

export function StateChip({ r }: { r: service.MobileRow }) {
  const label = t.mobile.state[r.state] ?? r.state
  if (r.state === 'running') return <span className="chip ok"><span className="dot" />{label}</span>
  if (busyStates.includes(r.state)) return <span className="chip warn"><span className="dot" />{label}</span>
  return <span className="chip">{label}</span>
}

type Props = {
  row: service.MobileRow
  proxies: service.ProxyRow[]
  knownTags: string[]
  onClose: () => void
  onStart: (id: string) => void
  onStop: (id: string) => void
  onShow: (id: string) => void
  onView: (row: service.MobileRow) => void
  onScreenshot: (id: string) => void
  onRecord: (id: string) => void
  onTop: (id: string) => void
  onBackup: (id: string) => void
  onRestore: (id: string) => void
  onInstallAurora: (id: string) => void
  onInstallApps: (id: string) => void
  onTrash: (row: service.MobileRow) => void
  recording?: boolean
  busy?: boolean
  onChanged?: () => void
}

/** Everything about one Android mobile device, edited in place next to the table. */
export function MobileDrawer({
  row, proxies, knownTags, onClose, onStart, onStop, onShow, onView, onScreenshot,
  onRecord, onTop, onBackup, onRestore, onInstallAurora, onInstallApps, onTrash,
  recording = false, busy = false, onChanged,
}: Props) {
  const toast = useToast()
  const confirm = useConfirm()
  const [tab, setTab] = useState<Tab>('overview')
  const [advanced, setAdvanced] = useState(false)
  const tabRefs = useRef<Record<Tab, HTMLButtonElement | null>>({ overview: null, hardware: null, activity: null })

  const running = row.state === 'running'
  const isBusy = busy || busyStates.includes(row.state)

  const update = async (patch: Partial<service.MobileInput>) => {
    try {
      const input = service.MobileInput.createFrom({
        name: row.name,
        proxyId: row.proxyId,
        ramMb: row.ramMb,
        cpus: row.cpus,
        preset: row.preset,
        screenW: row.screenW,
        screenH: row.screenH,
        density: row.density,
        notes: row.notes,
        tags: row.tags,
        pinned: row.pinned,
        ...patch,
      })
      await Api.UpdateMobileDevice(row.id, input)
      onChanged?.()
      return true
    } catch (e) {
      toast.error(errorText(e))
      return false
    }
  }

  const choosePreset = (name: Preset) => {
    const p = PRESETS[name]
    update({
      preset: name,
      ramMb: p.ram,
      cpus: p.cpus,
      screenW: p.screenW,
      screenH: p.screenH,
      density: p.density,
    })
  }

  const onTabKey = (e: KeyboardEvent) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const next = TABS[(TABS.indexOf(tab) + (e.key === 'ArrowRight' ? 1 : TABS.length - 1)) % TABS.length]
    setTab(next)
    tabRefs.current[next]?.focus()
  }

  const presetLabel = (p?: string) => {
    if (!p) return t.mobile.presetBalanced
    const key = ('preset' + p.charAt(0).toUpperCase() + p.slice(1)) as keyof typeof t.mobile
    return (t.mobile[key] as string) ?? p
  }

  return (
    <aside className="drawer card" aria-label={t.mobile.drawerLabel(row.name)}>
      <header className="drawer-head">
        <div className="drawer-title">
          <span className="mobile-device-avatar" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: 28, height: 28, borderRadius: 'var(--r-sm)', background: 'var(--accent-soft)', color: 'var(--accent)' }}>
            <Icon name="mobile" className="icon sm" />
          </span>
          <InlineName value={row.name} onSave={(name) => update({ name })} />
          <span className="serial">#{row.serial}</span>
          <span className="spacer" />
          <button type="button" className={`iconbtn sm${row.pinned ? ' on' : ''}`} aria-pressed={row.pinned}
            onClick={() => update({ pinned: !row.pinned })}
            aria-label={row.pinned ? t.menu.unpin : t.menu.pin} title={row.pinned ? t.menu.unpin : t.menu.pin}>
            <Icon name="pin" />
          </button>
          <button type="button" className="iconbtn sm" onClick={onClose} aria-label={t.drawer.close} title={t.drawer.close}>
            <Icon name="x" />
          </button>
        </div>

        <div className="drawer-sub">
          <span>{row.driver === 'vm' ? 'Anymobile VM' : 'LDPlayer'} · {presetLabel(row.preset)} · {row.ramMb / 1024} GB RAM · {row.cpus} CPU</span>
        </div>

        <div className="drawer-actions">
          <StateChip r={row} />
          {running && (
            <>
              <button type="button" className="iconbtn sm" disabled={isBusy} onClick={() => onShow(row.id)} title={t.mobile.show} aria-label={t.mobile.show}>
                <Icon name="monitor" />
              </button>
              <button type="button" className="iconbtn sm" disabled={isBusy} onClick={() => onView(row)} title={t.mobile.viewInApp} aria-label={t.mobile.viewInApp}>
                <Icon name="eye" />
              </button>
              <button type="button" className="iconbtn sm" disabled={isBusy} onClick={() => onScreenshot(row.id)} title={t.mobile.screenshot} aria-label={t.mobile.screenshot}>
                <Icon name="camera" />
              </button>
              <button type="button" className={`iconbtn sm${recording ? ' on' : ''}`} disabled={isBusy} onClick={() => onRecord(row.id)}
                title={recording ? t.mobile.stopRecording : t.mobile.recordScreen} aria-label={recording ? t.mobile.stopRecording : t.mobile.recordScreen}>
                <Icon name="record" />
              </button>
              <button type="button" className="iconbtn sm" disabled={isBusy} onClick={() => onTop(row.id)} title={t.mobile.alwaysOnTop} aria-label={t.mobile.alwaysOnTop}>
                <Icon name="layers" />
              </button>
            </>
          )}
          <span className="spacer" />
          {running || row.state === 'stopping' ? (
            <button type="button" className={`btn sm stop actbtn${isBusy ? ' busy' : ''}`} aria-busy={isBusy} disabled={isBusy} onClick={() => onStop(row.id)}>
              <Icon name={isBusy ? 'refresh' : 'stop'} className="icon sm" />{t.mobile.close}
            </button>
          ) : (
            <button type="button" className={`btn sm primary actbtn${isBusy ? ' busy' : ''}`} aria-busy={isBusy} disabled={isBusy} onClick={() => onStart(row.id)}>
              <Icon name={isBusy ? 'refresh' : 'play'} className="icon sm" />{t.mobile.open}
            </button>
          )}
        </div>

        <div className="tabs" role="tablist" aria-label={t.mobile.drawerLabel(row.name)} onKeyDown={onTabKey}>
          <button type="button" role="tab" ref={(el) => { tabRefs.current.overview = el }} id="mdtab-overview"
            aria-selected={tab === 'overview'} aria-controls="mdpanel-overview" tabIndex={tab === 'overview' ? 0 : -1} onClick={() => setTab('overview')}>
            {t.drawer.tabs.overview}
          </button>
          <button type="button" role="tab" ref={(el) => { tabRefs.current.hardware = el }} id="mdtab-hardware"
            aria-selected={tab === 'hardware'} aria-controls="mdpanel-hardware" tabIndex={tab === 'hardware' ? 0 : -1} onClick={() => setTab('hardware')}>
            {t.mobile.hardware}
          </button>
          <button type="button" role="tab" ref={(el) => { tabRefs.current.activity = el }} id="mdtab-activity"
            aria-selected={tab === 'activity'} aria-controls="mdpanel-activity" tabIndex={tab === 'activity' ? 0 : -1} onClick={() => setTab('activity')}>
            {t.mobile.activity}
          </button>
        </div>
      </header>

      <div className="drawer-body" role="tabpanel" id={`mdpanel-${tab}`} aria-labelledby={`mdtab-${tab}`}>
        {tab === 'overview' ? (
          <>
            <section className="dgroup" aria-label={t.mobile.proxy}>
              <h3>{t.mobile.proxy}</h3>
              {row.state !== 'stopped' ? (
                <div className="row muted">
                  <Icon name="proxy" className="icon sm" />
                  <span>{proxies.find((p) => p.id === row.proxyId)?.label ?? t.common.direct}</span>
                </div>
              ) : (
                <ProxySelect value={row.proxyId} onChange={(proxyId) => update({ proxyId })} proxies={proxies} currentProfile={row.name} disabled={isBusy} />
              )}
              {row.proxyId === '' && row.state === 'stopped' && <RealIpWarning text={t.mobile.realIpWarning} />}
              {row.state !== 'stopped' && <span className="hint">{t.mobile.closedOnly}</span>}
            </section>

            <TagsGroup tags={row.tags} known={knownTags} onChange={(tags) => update({ tags })} />
            <NotesGroup value={row.notes} onSave={async (notes) => update({ notes })} />

            <section className="dgroup" aria-label={t.mobile.apkHelp}>
              <h3>{t.mobile.apkHelp}</h3>
              <div className="row">
                <button type="button" className="btn sm" disabled={!running || isBusy} onClick={() => onInstallAurora(row.id)} title={t.mobile.installAurora}>
                  <Icon name="cart" className="icon sm" />{t.mobile.installAurora}
                </button>
                <button type="button" className="btn sm" disabled={!running || isBusy} onClick={() => onInstallApps(row.id)} title={t.mobile.installApps}>
                  <Icon name="download" className="icon sm" />{t.mobile.installApps}
                </button>
              </div>
              <span className="hint">
                {t.mobile.apkHint}{' '}
                <a href={MOBILE_APPS_GUIDE} target="_blank" rel="noreferrer" style={{ textDecoration: 'underline' }}>{t.mobile.apkGuide}</a>
              </span>
            </section>

            <section className="dgroup">
              <button type="button" className="disclosure" aria-expanded={advanced} onClick={() => setAdvanced(!advanced)}>
                <Icon name="chevron" className="icon sm" />{t.drawer.advanced}
              </button>
              {advanced && (
                <>
                  <dl className="facts">
                    <dt>Device ID</dt>
                    <dd className="mono">{row.id}</dd>
                    <dt>{t.mobile.driver}</dt>
                    <dd>{row.driver} ({row.platform})</dd>
                    <dt>{t.mobile.adbPort}</dt>
                    <dd className="mono">{row.adbPort ? `127.0.0.1:${row.adbPort}` : '—'}</dd>
                  </dl>
                  <div className="row" style={{ marginTop: 8 }}>
                    <button type="button" className="btn sm" disabled={row.state !== 'stopped' || isBusy} onClick={() => onBackup(row.id)}>
                      <Icon name="download" className="icon sm" />{t.mobile.backup}
                    </button>
                    {row.backedUpAt && (
                      <button type="button" className="btn sm" disabled={row.state !== 'stopped' || isBusy} onClick={() => onRestore(row.id)}>
                        <Icon name="refresh" className="icon sm" />{t.mobile.restoreBackup}
                      </button>
                    )}
                  </div>
                  <button type="button" className="btn sm danger textdanger" style={{ marginTop: 8 }} disabled={running || isBusy} onClick={() => onTrash(row)}>
                    <Icon name="trash" className="icon sm" />{t.drawer.moveToTrash}
                  </button>
                </>
              )}
            </section>
          </>
        ) : tab === 'hardware' ? (
          <>
            <section className="dgroup" aria-label={t.mobile.hardware}>
              <h3>{t.mobile.preset}</h3>
              <label className="field">
                <select className="select" value={row.preset || 'balanced'} disabled={row.state !== 'stopped' || isBusy} onChange={(e) => choosePreset(e.target.value as Preset)}>
                  <option value="automation">{t.mobile.presetAutomation} (2 CPU · 2 GB)</option>
                  <option value="balanced">{t.mobile.presetBalanced} (4 CPU · 3 GB)</option>
                  <option value="interactive">{t.mobile.presetInteractive} (4 CPU · 4 GB)</option>
                </select>
                <span className="hint">{t.mobile.presetHint}</span>
              </label>
            </section>

            <section className="dgroup" aria-label={t.mobile.ram}>
              <h3>{t.mobile.ram} & {t.mobile.cpus}</h3>
              <div className="row">
                <label className="field grow">
                  <span className="label">{t.mobile.ram}</span>
                  <select className="select" value={row.ramMb} disabled={row.state !== 'stopped' || isBusy} onChange={(e) => update({ ramMb: Number(e.target.value) })}>
                    {RAM_CHOICES.map((mb) => <option key={mb} value={mb}>{mb / 1024} GB</option>)}
                  </select>
                </label>
                <label className="field grow">
                  <span className="label">{t.mobile.cpus}</span>
                  <select className="select" value={row.cpus} disabled={row.state !== 'stopped' || isBusy} onChange={(e) => update({ cpus: Number(e.target.value) })}>
                    {CPU_CHOICES.map((n) => <option key={n} value={n}>{n} CPU</option>)}
                  </select>
                </label>
              </div>
            </section>

            <section className="dgroup">
              <h3>{t.mobile.deviceInfo}</h3>
              <dl className="facts">
                <dt>{t.mobile.screen(row.screenW, row.screenH, row.density).split(',')[0]}</dt>
                <dd>{row.screenW} × {row.screenH} px ({row.density} DPI)</dd>
                <dt>{t.mobile.driver}</dt>
                <dd>{row.driver === 'vm' ? 'Anymobile VM (QEMU)' : 'LDPlayer 9'}</dd>
                <dt>{t.mobile.imageVersion}</dt>
                <dd>{row.imageVersion || 'LineageOS Android 16'}</dd>
              </dl>
              {row.state !== 'stopped' && (
                <div className="banner warn" style={{ marginTop: 12 }}>
                  <Icon name="alert" />
                  <span className="grow">{t.mobile.hardwareLockedNotice}</span>
                </div>
              )}
            </section>
          </>
        ) : (
          <>
            <section className="dgroup" aria-label={t.mobile.activity}>
              <h3>{t.mobile.activity}</h3>
              <dl className="facts">
                <dt>{t.mobile.serialNum}</dt>
                <dd className="mono">#{row.serial}</dd>
                <dt>{t.mobile.colState}</dt>
                <dd><StateChip r={row} /></dd>
                <dt>{t.mobile.createdAt}</dt>
                <dd>{row.createdAt ? relativeTime(row.createdAt) : '—'}</dd>
                <dt>{t.mobile.updatedAt}</dt>
                <dd>{row.updatedAt ? relativeTime(row.updatedAt) : '—'}</dd>
                <dt>{t.mobile.colLastOpen}</dt>
                <dd>{row.lastOpenAt ? relativeTime(row.lastOpenAt) : t.mobile.never}</dd>
                <dt>{t.mobile.backedUp('')}</dt>
                <dd>{row.backedUpAt ? relativeTime(row.backedUpAt) : t.mobile.noBackupYet}</dd>
                <dt>{t.mobile.adbPort}</dt>
                <dd className="mono">{row.adbPort ? `127.0.0.1:${row.adbPort}` : '—'}</dd>
              </dl>
            </section>

            <section className="dgroup">
              <h3>{t.mobile.backup}</h3>
              <div className="row">
                <button type="button" className="btn sm" disabled={row.state !== 'stopped' || isBusy} onClick={() => onBackup(row.id)}>
                  <Icon name="download" className="icon sm" />{t.mobile.backupNow}
                </button>
                {row.backedUpAt && (
                  <button type="button" className="btn sm" disabled={row.state !== 'stopped' || isBusy}
                    onClick={async () => {
                      if (await confirm({ ...t.confirm.restoreMobile(relativeTime(row.backedUpAt)), danger: true })) {
                        onRestore(row.id)
                      }
                    }}>
                    <Icon name="refresh" className="icon sm" />{t.mobile.restoreFromBackup}
                  </button>
                )}
              </div>
            </section>

            {row.error && (
              <div className="banner danger" role="alert">
                <Icon name="alert" />
                <span className="grow">{row.error}</span>
              </div>
            )}
          </>
        )}
      </div>
    </aside>
  )
}

