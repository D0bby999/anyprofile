// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Api, copyText, service } from '../api'
import { t } from '../i18n'
import { errorText, formatDate, relativeTime } from '../lib/format'
import { Icon } from './icons'
import { InlineName } from './drawer/inline-name'
import { NotesGroup } from './drawer/notes-group'
import { useToast } from './toast'
import { useConfirm } from './confirm'

export type ProxyTab = 'overview' | 'network' | 'usedBy'
const TABS: ProxyTab[] = ['overview', 'network', 'usedBy']

function ProxyStateChip({ p }: { p: service.ProxyRow }) {
  const chip = (cls: string, text: string) => (
    <span className={`chip ${cls}`} title={text}>
      <span className="dot" />
      <span className="chiptext">{text}</span>
    </span>
  )
  if (p.failedAt) return chip('danger', t.proxies.failed)
  if (!p.checkedAt) return chip('', t.proxies.unchecked)
  if (p.rotating) return chip('warn', t.proxies.rotating)
  return chip('ok', t.proxies.ok)
}

type Props = {
  row: service.ProxyRow
  onClose: () => void
  onUpdated: () => void
  onDeleted: () => void
}

export function ProxyDrawer({ row, onClose, onUpdated, onDeleted }: Props) {
  const [tab, setTab] = useState<ProxyTab>('overview')
  const [checking, setChecking] = useState(false)
  const [copied, setCopied] = useState(false)
  const [expiresOn, setExpiresOn] = useState(row.expiresOn || '')
  const toast = useToast()
  const confirm = useConfirm()
  const tabRefs = useRef<Record<ProxyTab, HTMLButtonElement | null>>({
    overview: null,
    network: null,
    usedBy: null,
  })

  useEffect(() => {
    setExpiresOn(row.expiresOn || '')
  }, [row.expiresOn])

  const saveMeta = async (patch: Partial<service.ProxyMeta>): Promise<boolean> => {
    try {
      await Api.UpdateProxy(
        row.id,
        service.ProxyMeta.createFrom({
          label: patch.label !== undefined ? patch.label : row.label,
          note: patch.note !== undefined ? patch.note : row.note,
          expiresOn: patch.expiresOn !== undefined ? patch.expiresOn : row.expiresOn,
        }),
      )
      onUpdated()
      return true
    } catch (e) {
      toast.error(errorText(e))
      return false
    }
  }

  const check = async () => {
    setChecking(true)
    try {
      await Api.CheckProxy(row.id)
      toast.success(t.proxies.ok)
      onUpdated()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setChecking(false)
    }
  }

  const copyAddress = async () => {
    const addr = `${row.scheme}://${row.host}:${row.port}`
    await copyText(addr)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast.success(t.proxies.copied)
  }

  const remove = async () => {
    if (await confirm({ ...t.confirm.deleteProxy, danger: true })) {
      try {
        await Api.DeleteProxy(row.id)
        onDeleted()
      } catch (e) {
        toast.error(errorText(e))
      }
    }
  }

  const onTabKey = (e: KeyboardEvent) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const next = TABS[(TABS.indexOf(tab) + (e.key === 'ArrowRight' ? 1 : TABS.length - 1)) % TABS.length]
    setTab(next)
    tabRefs.current[next]?.focus()
  }

  return (
    <aside className="drawer card" aria-label={t.proxies.drawerLabel(row.label)}>
      <header className="drawer-head">
        <div className="drawer-title">
          <span
            className="proxy-avatar"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 28,
              height: 28,
              borderRadius: 'var(--r-sm)',
              background: 'var(--accent-soft)',
              color: 'var(--accent)',
              fontWeight: 650,
              fontSize: 'var(--t-xs)',
            }}
          >
            {row.country || <Icon name="proxy" className="icon sm" />}
          </span>
          <InlineName value={row.label} onSave={(name) => saveMeta({ label: name })} />
          <ProxyStateChip p={row} />
          <span className="spacer" />
          <button
            type="button"
            className="iconbtn sm"
            onClick={onClose}
            aria-label={t.drawer.close}
            title={t.drawer.close}
          >
            <Icon name="x" />
          </button>
        </div>

        <div className="drawer-sub">
          <span className="mono">{row.scheme}://{row.host}:{row.port}</span>
          {row.country && <span> · {row.country}</span>}
          {row.timezone && <span> · {row.timezone}</span>}
        </div>

        <div className="drawer-actions">
          <button
            type="button"
            className={`btn sm${checking ? ' busy' : ''}`}
            aria-busy={checking}
            disabled={checking}
            onClick={check}
          >
            <Icon name="refresh" className="icon sm" />
            {t.proxies.check}
          </button>
          <button
            type="button"
            className="btn sm ghost"
            onClick={copyAddress}
            title={t.proxies.copy}
          >
            <Icon name={copied ? 'check' : 'copy'} className="icon sm" />
            {copied ? t.proxies.copied : t.proxies.copy}
          </button>
          <span className="spacer" />
          <button
            type="button"
            className="iconbtn sm danger"
            onClick={remove}
            aria-label={t.common.delete}
            title={t.common.delete}
          >
            <Icon name="trash" />
          </button>
        </div>

        <div className="tabs" role="tablist" onKeyDown={onTabKey} aria-label={t.proxies.drawerLabel(row.label)}>
          <button
            ref={(el) => { tabRefs.current.overview = el }}
            role="tab"
            id="ptab-overview"
            aria-selected={tab === 'overview'}
            aria-controls="ppanel-overview"
            tabIndex={tab === 'overview' ? 0 : -1}
            onClick={() => setTab('overview')}
            type="button"
          >
            {t.proxies.tabOverview}
          </button>
          <button
            ref={(el) => { tabRefs.current.network = el }}
            role="tab"
            id="ptab-network"
            aria-selected={tab === 'network'}
            aria-controls="ppanel-network"
            tabIndex={tab === 'network' ? 0 : -1}
            onClick={() => setTab('network')}
            type="button"
          >
            {t.proxies.tabNetwork}
          </button>
          <button
            ref={(el) => { tabRefs.current.usedBy = el }}
            role="tab"
            id="ptab-usedby"
            aria-selected={tab === 'usedBy'}
            aria-controls="ppanel-usedby"
            tabIndex={tab === 'usedBy' ? 0 : -1}
            onClick={() => setTab('usedBy')}
            type="button"
          >
            {t.proxies.tabUsedBy}
            {row.usedBy?.length > 0 && <span className="counter">{row.usedBy.length}</span>}
          </button>
        </div>
      </header>

      {tab === 'overview' && (
        <div className="drawer-body" role="tabpanel" id="ppanel-overview" aria-labelledby="ptab-overview">
          <section className="dgroup" aria-label={t.proxies.connection}>
            <h3>{t.proxies.connection}</h3>
            <dl className="facts">
              <dt>{t.proxies.scheme}</dt>
              <dd className="mono">{row.scheme.toUpperCase()}</dd>
              <dt>{t.proxies.host}</dt>
              <dd className="mono selectable">{row.host}</dd>
              <dt>{t.proxies.port}</dt>
              <dd className="mono">{row.port}</dd>
              <dt>{t.proxies.username}</dt>
              <dd className="mono">{row.username || '—'}</dd>
            </dl>
            <div className="row muted" style={{ fontSize: 'var(--t-xs)', marginTop: 8 }}>
              <Icon name="key" className="icon sm" />
              <span>{t.proxies.passwordNote}</span>
            </div>
          </section>

          <section className="dgroup" aria-label={t.proxies.expires}>
            <h3>{t.proxies.expires}</h3>
            <div className="row" style={{ alignItems: 'center', gap: 10 }}>
              <input
                type="date"
                className="input"
                style={{ width: 160 }}
                value={expiresOn}
                onChange={(e) => {
                  setExpiresOn(e.target.value)
                  saveMeta({ expiresOn: e.target.value })
                }}
              />
              {expiresOn && (
                <button
                  type="button"
                  className="btn sm ghost"
                  onClick={() => {
                    setExpiresOn('')
                    saveMeta({ expiresOn: '' })
                  }}
                >
                  {t.common.clear}
                </button>
              )}
              {row.expiry === 'soon' && (
                <span className="chip warn">
                  <span className="dot" />
                  <span className="chiptext">{t.proxies.expiringSoon}</span>
                </span>
              )}
              {row.expiry === 'expired' && (
                <span className="chip danger">
                  <span className="dot" />
                  <span className="chiptext">{t.proxies.expired}</span>
                </span>
              )}
            </div>
          </section>

          <NotesGroup value={row.note} onSave={async (note) => saveMeta({ note })} />
        </div>
      )}

      {tab === 'network' && (
        <div className="drawer-body" role="tabpanel" id="ppanel-network" aria-labelledby="ptab-network">
          <section className="dgroup" aria-label={t.proxies.colStatus}>
            <h3>{t.proxies.colStatus}</h3>
            <dl className="facts">
              <dt>{t.proxies.colStatus}</dt>
              <dd><ProxyStateChip p={row} /></dd>
              <dt>{t.proxies.lastChecked}</dt>
              <dd>{row.checkedAt ? `${relativeTime(row.checkedAt)} (${formatDate(row.checkedAt)})` : t.proxies.unchecked}</dd>
            </dl>
            {row.failedAt && (
              <div className="banner danger" style={{ marginTop: 10 }}>
                <Icon name="alert" />
                <span>{t.proxies.failed}</span>
              </div>
            )}
          </section>

          <section className="dgroup" aria-label={t.proxies.tabNetwork}>
            <h3>{t.proxies.tabNetwork}</h3>
            <dl className="facts">
              <dt>{t.proxies.exitIp}</dt>
              <dd className="mono selectable">{row.exitIp || '—'}</dd>
              <dt>{t.proxies.country}</dt>
              <dd>{row.country || '—'}</dd>
              <dt>{t.proxies.timezone}</dt>
              <dd>{row.timezone || '—'}</dd>
              <dt>{t.proxies.coords}</dt>
              <dd className="mono">
                {row.lat !== undefined && row.lon !== undefined ? `${row.lat}, ${row.lon}` : '—'}
              </dd>
              <dt>{t.proxies.proxyType}</dt>
              <dd>{row.rotating ? t.proxies.rotatingIp : t.proxies.stickyIp}</dd>
              {row.exitIp6 && (
                <>
                  <dt>IPv6</dt>
                  <dd className="mono selectable">{row.exitIp6}</dd>
                  <dt>IPv6 Timezone</dt>
                  <dd>{row.timezone6 || '—'}</dd>
                </>
              )}
            </dl>
            {row.timezone6 && row.timezone6 !== row.timezone && (
              <div className="banner warn" style={{ marginTop: 10 }}>
                <Icon name="alert" />
                <span>{t.proxies.ipv6Elsewhere(row.exitIp6, row.timezone6)}</span>
              </div>
            )}
          </section>
        </div>
      )}

      {tab === 'usedBy' && (
        <div className="drawer-body" role="tabpanel" id="ppanel-usedby" aria-labelledby="ptab-usedby">
          <section className="dgroup" aria-label={t.proxies.tabUsedBy}>
            <h3>{t.proxies.usedByCount(row.usedBy?.length ?? 0)}</h3>
            {!row.usedBy || row.usedBy.length === 0 ? (
              <p className="muted" style={{ margin: '8px 0 0' }}>{t.proxies.noProfilesUsing}</p>
            ) : (
              <ul style={{ listStyle: 'none', padding: 0, margin: '12px 0 0', display: 'flex', flexDirection: 'column', gap: 6 }}>
                {row.usedBy.map((name) => (
                  <li
                    key={name}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      padding: '8px 12px',
                      background: 'var(--surface-sunken)',
                      borderRadius: 'var(--r-md)',
                      border: '1px solid var(--line)',
                    }}
                  >
                    <Icon name="profiles" className="icon sm muted" />
                    <span style={{ fontWeight: 550, fontSize: 'var(--t-sm)' }}>{name}</span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      )}
    </aside>
  )
}
