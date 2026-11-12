// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Api, onDataChanged, service } from '../api'
import { t } from '../i18n'
import { errorText, formatDate, proxyLineError, relativeTime } from '../lib/format'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { PageHead } from '../components/page-head'
import { ProxyAddDialog } from '../components/proxy-add-dialog'
import { ProxyDrawer } from '../components/proxy-drawer'
import { BuyProxiesButton } from '../components/partner-grid'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'

function ProxyState({ p }: { p: service.ProxyRow }) {
  const chip = (cls: string, text: string) =>
    <span className={`chip ${cls}`} title={text}><span className="dot" /><span className="chiptext">{text}</span></span>
  if (p.failedAt) return chip('danger', t.proxies.failed)
  if (!p.checkedAt) return chip('', t.proxies.unchecked)
  if (p.rotating) return chip('warn', t.proxies.rotating)
  return chip('ok', t.proxies.ok)
}

export function ProxiesPage({ onChanged }: { onChanged: () => void }) {
  const [rows, setRows] = useState<service.ProxyRow[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'ok' | 'failed' | 'expiring'>('all')
  const [text, setText] = useState('')
  const [busy, setBusy] = useState<string>('') // '' | 'import' | 'all' | proxy id
  // Lines of the last import that could not be added; they stay next to the paste box.
  const [failedLines, setFailedLines] = useState<string[]>([])
  const toast = useToast()
  const confirm = useConfirm()
  const [adding, setAdding] = useState(false)
  const [expiresOn, setExpiresOn] = useState('')
  const [defaultScheme, setDefaultScheme] = useState<'auto' | 'http' | 'https' | 'socks5'>('auto')

  const load = useCallback(() => { Api.ListProxies().then((r) => setRows(r ?? [])).catch((e) => toast.error(errorText(e))) }, [toast])
  useEffect(load, [load])
  useEffect(() => onDataChanged(['proxies', 'profiles', 'mobile'], load), [load])

  const run = async (what: string, fn: () => Promise<unknown>) => {
    setBusy(what)
    setFailedLines([])
    try {
      await fn()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy('')
      load()
      onChanged()
    }
  }

  const importList = () => run('import', async () => {
    const payload = defaultScheme === 'auto' ? text : text.split('\n').map((raw) => {
      const line = raw.trim()
      if (!line || line.startsWith('#') || line.includes('://') || /^(https?|socks5h?):/i.test(line) || /:(https?|socks5h?)$/i.test(line)) {
        return raw
      }
      return `${defaultScheme}://${line}`
    }).join('\n')
    const res = await Api.ImportProxies(payload, expiresOn)
    if (res.added > 0) toast.success(t.proxies.added(res.added))
    const rejected = new Set((res as { rejected?: string[] }).rejected ?? [])
    setFailedLines([
      ...(res.failed ?? []).map(proxyLineError),
      ...(res.unreachable ?? []).map((label) => `${label}: ${rejected.has(label) ? t.errors.proxy_rejected : t.errors.proxy_unreachable}`),
    ])
    setText('') // pasted passwords should not linger on screen
    setExpiresOn('')
  })

  const visibleRows = useMemo(() => {
    let list = rows
    if (statusFilter === 'ok') list = list.filter((p) => p.checkedAt && !p.failedAt)
    else if (statusFilter === 'failed') list = list.filter((p) => !!p.failedAt)
    else if (statusFilter === 'expiring') list = list.filter((p) => p.expiry === 'soon' || p.expiry === 'expired')

    const q = search.trim().toLowerCase()
    if (!q) return list
    return list.filter((p) =>
      p.label.toLowerCase().includes(q) ||
      p.host.toLowerCase().includes(q) ||
      p.exitIp.toLowerCase().includes(q) ||
      p.country.toLowerCase().includes(q) ||
      (p.note && p.note.toLowerCase().includes(q)) ||
      (p.username && p.username.toLowerCase().includes(q)) ||
      p.usedBy.some((u) => u.toLowerCase().includes(q))
    )
  }, [rows, search, statusFilter])

  const active = rows.find((r) => r.id === activeId) ?? null

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.total(rows.length)}>
        <button className={`btn nodrag${busy === 'all' ? ' busy' : ''}`} aria-busy={busy === 'all'} disabled={!!busy || rows.length === 0}
          onClick={() => run('all', async () => {
            const failed = await Api.CheckAllProxies()
            if (failed > 0) toast.error(t.proxies.failedCount(failed))
            else toast.success(t.proxies.allOk)
          })}>
          <Icon name="refresh" />{t.proxies.checkAll}
        </button>
        <BuyProxiesButton className="btn buy nodrag" />
        <button type="button" className="btn primary nodrag" disabled={!!busy} onClick={() => setAdding(true)}>
          <Icon name="plus" />{t.proxies.newBtn}
        </button>
      </PageHead>
      <div className="pagebody">
        <section className="card pad">
          <h2>{t.proxies.addTitle}</h2>
          <p className="desc">{t.proxies.addDesc}</p>
          <textarea id="proxy-paste" className="textarea code" rows={4} value={text} placeholder={t.proxies.pastePlaceholder}
            spellCheck={false} autoComplete="off" onChange={(e) => setText(e.target.value)} aria-label={t.proxies.addTitle} />
          <div className="row">
            <button className={`btn primary${busy === 'import' ? ' busy' : ''}`} aria-busy={busy === 'import'} disabled={!!busy || !text.trim()} onClick={importList}>
              <Icon name={busy === 'import' ? 'refresh' : 'plus'} />{busy === 'import' ? t.proxies.importing : t.proxies.import}
            </button>
            <label className="row" style={{ fontSize: 'var(--t-sm)' }}>
              {t.proxies.scheme}
              <select className="select" style={{ width: 130 }} value={defaultScheme} onChange={(e) => setDefaultScheme(e.target.value as any)}>
                <option value="auto">{t.proxies.schemeAuto}</option>
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
                <option value="socks5">SOCKS5</option>
              </select>
            </label>
            <label className="row" style={{ fontSize: 'var(--t-sm)' }}>
              {t.proxies.expiresOn}
              <input type="date" className="input" style={{ width: 150 }} value={expiresOn} onChange={(e) => setExpiresOn(e.target.value)} />
            </label>
            <span className="row muted" style={{ fontSize: 'var(--t-xs)' }}><Icon name="key" className="icon sm" />{t.proxies.passwordNote}</span>
          </div>
          {failedLines.length > 0 && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{failedLines.map((e) => <div key={e}>{e}</div>)}</span></div>}
        </section>

        <div className={`profiles-split${active ? ' open' : ''}`}>
          <section className="card fill" style={{ minHeight: 220 }}>
            {rows.length > 0 && (
              <div className="toolbar">
                <div className="searchfield">
                  <Icon name="search" />
                  <input
                    id="proxy-search"
                    className="input"
                    placeholder={t.proxies.search}
                    aria-label={t.proxies.search}
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </div>
                {search && (
                  <button type="button" className="btn sm ghost" onClick={() => setSearch('')}>
                    <Icon name="x" className="icon xs" />
                  </button>
                )}
                <div className="seg" role="group" aria-label={t.proxies.colStatus}>
                  <button type="button" className={`seg-btn${statusFilter === 'all' ? ' on' : ''}`} onClick={() => setStatusFilter('all')}>
                    {t.proxies.filterAll}
                  </button>
                  <button type="button" className={`seg-btn${statusFilter === 'ok' ? ' on' : ''}`} onClick={() => setStatusFilter('ok')}>
                    {t.proxies.filterOk}
                  </button>
                  <button type="button" className={`seg-btn${statusFilter === 'failed' ? ' on' : ''}`} onClick={() => setStatusFilter('failed')}>
                    {t.proxies.filterFailed}
                  </button>
                  <button type="button" className={`seg-btn${statusFilter === 'expiring' ? ' on' : ''}`} onClick={() => setStatusFilter('expiring')}>
                    {t.proxies.filterExpiring}
                  </button>
                </div>
                <span className="spacer" />
                <span className="faint" style={{ fontSize: 'var(--t-xs)' }}>
                  {t.proxies.total(visibleRows.length)}
                </span>
              </div>
            )}

            <div className="tablewrap">
              {rows.length === 0 ? (
                // The paste box sits right above, so the empty state points to single add or buying.
                <EmptyState icon="proxy" title={t.proxies.emptyTitle}>
                  <div className="row">
                    <BuyProxiesButton />
                    <button type="button" className="btn primary" disabled={!!busy} onClick={() => setAdding(true)}>
                      <Icon name="plus" />{t.proxies.newBtn}
                    </button>
                  </div>
                </EmptyState>
              ) : visibleRows.length === 0 ? (
                <EmptyState icon="search" title={t.profiles.noMatchTitle} />
              ) : (
                /* Fixed column widths: a long host, label or profile name is cut
                   with an ellipsis instead of running into the next column. */
                <table className="table proxies">
                  <thead>
                    <tr><th>{t.proxies.colLabel}</th><th>{t.proxies.colExit}</th>
                      <th>{t.proxies.colStatus}</th><th>{t.proxies.expires}</th><th>{t.proxies.colUsedBy}</th><th className="col-actions" /></tr>
                  </thead>
                  <tbody>
                    {visibleRows.map((p) => (
                      <tr
                        key={p.id}
                        className={p.id === activeId ? 'active' : ''}
                        onClick={() => setActiveId(activeId === p.id ? null : p.id)}
                      >
                        <td className="cellname">
                          <b className="selectable" title={p.label}>{p.label}</b>
                          <span className="mono" title={`${p.scheme}://${p.host}:${p.port}`}>{p.scheme}://{p.host}:{p.port}</span>
                          {p.note && <small className="faint ellipsis" title={p.note}>{p.note}</small>}
                        </td>
                        <td>{p.exitIp ? (
                          <span className="cellproxy">
                            <span className="cc" title={p.country || '?'}>{p.country || '?'}</span>
                            <span className="exittext">
                              <span className="mono selectable" title={p.exitIp}>{p.exitIp}</span>{p.timezone && <span className="tzline" title={p.timezone}>{p.timezone}</span>}
                              {p.timezone6 && p.timezone6 !== p.timezone && (
                                <span className="tzline warn" title={t.proxies.ipv6Elsewhere(p.exitIp6, p.timezone6)}>
                                  <Icon name="alert" className="icon xs" />IPv6 · {p.timezone6}
                                </span>
                              )}
                            </span>
                          </span>
                        ) : <span className="faint">{p.checkedAt ? '—' : t.proxies.unchecked}</span>}</td>
                        <td className="stack nowrap"><ProxyState p={p} />{p.checkedAt && <small className="faint">{t.proxies.checkedAgo(relativeTime(p.checkedAt))}</small>}</td>
                        <td className="stack nowrap">
                          {p.expiresOn && <span className="muted">{formatDate(p.expiresOn + 'T00:00:00')}</span>}
                          {p.expiry === 'soon' && <span className="chip warn" title={t.proxies.expiringSoon}><span className="dot" /><span className="chiptext">{t.proxies.expiringSoon}</span></span>}
                          {p.expiry === 'expired' && <span className="chip danger" title={t.proxies.expired}><span className="dot" /><span className="chiptext">{t.proxies.expired}</span></span>}
                        </td>
                        {/* Two names and a count: a long list wraps into the rows below
                            otherwise, and every name is still in the tooltip. */}
                        <td className="col-usedby">{p.usedBy.length ? (
                          <span className="usedby" title={p.usedBy.join(', ')}>
                            <span className="chip accent"><span className="chiptext">{p.usedBy[0]}</span></span>
                            {p.usedBy.length > 1 && <span className="chip more" title={p.usedBy.join(', ')}>+{p.usedBy.length - 1}</span>}
                          </span>
                        ) : <span className="faint">{t.proxies.unused}</span>}</td>
                        <td className="col-actions"><span className="row">
                          <button
                            className={`iconbtn sm${busy === p.id ? ' busy' : ''}`}
                            aria-busy={busy === p.id}
                            disabled={!!busy}
                            onClick={(e) => { e.stopPropagation(); run(p.id, () => Api.CheckProxy(p.id)) }}
                            aria-label={t.proxies.check}
                            title={t.proxies.check}
                          >
                            <Icon name="refresh" />
                          </button>
                          <button
                            className="iconbtn sm"
                            disabled={!!busy}
                            onClick={(e) => { e.stopPropagation(); setActiveId(activeId === p.id ? null : p.id) }}
                            aria-label={t.proxies.rename}
                            title={t.proxies.rename}
                          >
                            <Icon name="pencil" />
                          </button>
                          <button
                            className="iconbtn sm"
                            disabled={!!busy}
                            onClick={async (e) => {
                              e.stopPropagation()
                              if (await confirm({ ...t.confirm.deleteProxy, danger: true })) {
                                run(p.id, () => Api.DeleteProxy(p.id))
                                if (activeId === p.id) setActiveId(null)
                              }
                            }}
                            aria-label={t.common.delete}
                            title={t.common.delete}
                          >
                            <Icon name="trash" />
                          </button>
                        </span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </section>

          {active && (
            <ProxyDrawer
              row={active}
              onClose={() => setActiveId(null)}
              onUpdated={() => { load(); onChanged() }}
              onDeleted={() => { setActiveId(null); load(); onChanged() }}
            />
          )}
        </div>
      </div>
      {adding && (
        <ProxyAddDialog
          onClose={() => setAdding(false)}
          onAdded={(res) => {
            setAdding(false)
            if (res.checkErr) toast.error(`${res.proxy.label}: ${t.errors[res.checkErr] ?? res.checkErr}`)
            else toast.success(t.proxies.added(1))
            load()
            onChanged()
          }}
        />
      )}
    </>
  )
}
