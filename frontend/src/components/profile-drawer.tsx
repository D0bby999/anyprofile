// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { machineLabel } from '../lib/persona'
import { siteById, type Site } from '../lib/sites'
import { AccountPanel } from './account-panel'
import { useConfirm } from './confirm'
import { HealthPill } from './health-pill'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'
import { useToast } from './toast'
import { IdentityGroup } from './drawer/identity-group'
import { InlineName } from './drawer/inline-name'
import { NetworkGroup } from './drawer/network-group'
import { NotesGroup } from './drawer/notes-group'
import { PlatformGroup } from './drawer/platform-group'
import { TagsGroup } from './drawer/tags-group'
import { SessionsTab } from './drawer/sessions-tab'

type Tab = 'overview' | 'account' | 'sessions'
const TABS: Tab[] = ['overview', 'account', 'sessions']

type Props = {
  row: service.ProfileRow
  sites: Site[]
  recent: string[]
  knownTags: string[]
  onClose: () => void
  onStart: (id: string) => void
  onStop: (id: string) => void
}

/** Everything about one profile, edited in place next to the table. */
export function ProfileDrawer({ row, sites, recent, knownTags, onClose, onStart, onStop }: Props) {
  const toast = useToast()
  const confirm = useConfirm()
  const [tab, setTab] = useState<Tab>('overview')
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [login, setLogin] = useState(row.login)
  const [advanced, setAdvanced] = useState(false)
  const tabRefs = useRef<Record<Tab, HTMLButtonElement | null>>({ overview: null, account: null, sessions: null })
  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])).catch(() => {}) }, [row.proxyId])
  useEffect(() => setLogin(row.login), [row.login])

  const update = async (patch: Partial<store.Profile>) => {
    try {
      await Api.UpdateProfile(store.Profile.createFrom({ ...row, ...patch }))
      return true
    } catch (e) {
      toast.error(errorText(e))
      return false
    }
  }

  const running = row.state !== 'stopped'
  const busy = row.state === 'starting' || row.state === 'stopping'
  const site = siteById(sites, row.site)
  const engine = `${t.engines.names[row.engine] ?? row.engine} ${row.engineVersion}`

  const onTabKey = (e: KeyboardEvent) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const next = TABS[(TABS.indexOf(tab) + (e.key === 'ArrowRight' ? 1 : TABS.length - 1)) % TABS.length]
    setTab(next)
    tabRefs.current[next]?.focus()
  }

  const matchIp = async () => {
    if (!(await confirm(t.confirm.matchIp))) return
    try {
      await Api.MatchExit(row.id)
      toast.success(t.sessions.matched)
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  const trash = async () => {
    if (!(await confirm({ ...t.confirm.trash(row.name), danger: true }))) return
    try {
      await Api.TrashProfile(row.id)
      onClose()
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  return (
    <aside className="drawer card" aria-label={t.drawer.label(row.name)}>
      <header className="drawer-head">
        <div className="drawer-title">
          <SiteGlyph site={site} />
          <InlineName value={row.name} onSave={(name) => update({ name })} />
          <span className="serial">#{row.serial}</span>
          <span className="spacer" />
          <button type="button" className={`iconbtn sm${row.pinned ? ' on' : ''}`} aria-pressed={row.pinned}
            onClick={() => Api.SetPinnedMany([row.id], !row.pinned).catch((e) => toast.error(errorText(e)))}
            aria-label={row.pinned ? t.menu.unpin : t.menu.pin} title={row.pinned ? t.menu.unpin : t.menu.pin}>
            <Icon name="pin" />
          </button>
          <button type="button" className="iconbtn sm" onClick={onClose} aria-label={t.drawer.close} title={t.drawer.close}>
            <Icon name="x" />
          </button>
        </div>
        <div className="drawer-sub">
          <span>{machineLabel(row)} · {engine}</span>
        </div>
        <div className="drawer-actions">
          <HealthPill row={row} />
          <span className="spacer" />
          {running || row.state === 'stopping' ? (
            <button type="button" className="btn sm stop actbtn" disabled={busy} onClick={() => onStop(row.id)}>
              <Icon name="stop" className="icon sm" />{busy ? t.profiles.stopping : t.profiles.stop}
            </button>
          ) : (
            <button type="button" className={`btn sm primary actbtn${busy ? ' busy' : ''}`} aria-busy={busy} disabled={busy} onClick={() => onStart(row.id)}>
              <Icon name={busy ? 'refresh' : 'play'} className="icon sm" />{busy ? t.profiles.opening : t.profiles.open}
            </button>
          )}
        </div>
        <div className="tabs" role="tablist" aria-label={t.drawer.label(row.name)} onKeyDown={onTabKey}>
          {TABS.map((x) => (
            <button key={x} type="button" role="tab" ref={(el) => { tabRefs.current[x] = el }} id={`dtab-${x}`}
              aria-selected={tab === x} aria-controls={`dpanel-${x}`} tabIndex={tab === x ? 0 : -1} onClick={() => setTab(x)}>
              {t.drawer.tabs[x]}
            </button>
          ))}
        </div>
      </header>

      <div className="drawer-body" role="tabpanel" id={`dpanel-${tab}`} aria-labelledby={`dtab-${tab}`}>
        {tab === 'overview' ? (
          <>
            <NetworkGroup row={row} proxies={proxies} running={running} onApply={(proxyId) => update({ proxyId })} onMatchIp={matchIp} />
            <PlatformGroup site={row.site} startUrl={row.startUrl} sites={sites} recent={recent} running={running}
              onSite={(id) => update({ site: id })} onStartUrl={(startUrl) => update({ startUrl })} />
            <TagsGroup tags={row.tags} known={knownTags} onChange={(tags) => update({ tags })} />
            <NotesGroup value={row.notes} onSave={(notes) => update({ notes })} />
            <IdentityGroup row={row} />
            <section className="dgroup">
              <button type="button" className="disclosure" aria-expanded={advanced} onClick={() => setAdvanced(!advanced)}>
                <Icon name="chevron" className="icon sm" />{t.drawer.advanced}
              </button>
              {advanced && (
                <>
                  <label className="row checkrow">
                    <input type="checkbox" checked={row.cdpNextLaunch} onChange={(e) => update({ cdpNextLaunch: e.target.checked })} />
                    {t.editor.cdp}
                  </label>
                  {row.cdpNextLaunch && <div className="banner danger"><Icon name="alert" /><span className="grow">{t.editor.cdpWarning}</span></div>}
                  <button type="button" className="btn sm danger textdanger" disabled={running} title={running ? t.drawer.closeFirst : undefined} onClick={trash}>
                    <Icon name="trash" className="icon sm" />{t.drawer.moveToTrash}
                  </button>
                </>
              )}
            </section>
          </>
        ) : tab === 'sessions' ? (
          <SessionsTab profileId={row.id} />
        ) : (
          <AccountPanel row={row} login={login} onLogin={setLogin} onError={toast.error}
            onLoginCommit={() => { if (login.trim() !== row.login) update({ login: login.trim() }) }} />
        )}
      </div>
    </aside>
  )
}
