// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useRef, useState } from 'react'
import { Api, onDataChanged, service } from '../api'
import { language, t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { PageHead } from '../components/page-head'
import { ProxyEditDialog } from '../components/proxy-edit-dialog'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'

function ProxyState({ p }: { p: service.ProxyRow }) {
  if (!p.checkedAt) return <span className="chip">{t.proxies.unchecked}</span>
  if (p.rotating) return <span className="chip warn"><span className="dot" />{t.proxies.rotating}</span>
  return <span className="chip ok"><span className="dot" />{t.proxies.ok}</span>
}

export function ProxiesPage({ onChanged }: { onChanged: () => void }) {
  const [rows, setRows] = useState<service.ProxyRow[]>([])
  const [text, setText] = useState('')
  const [busy, setBusy] = useState<string>('') // '' | 'import' | 'all' | proxy id
  // Lines of the last import that could not be added; they stay next to the paste box.
  const [failedLines, setFailedLines] = useState<string[]>([])
  const toast = useToast()
  const confirm = useConfirm()
  const [editing, setEditing] = useState<service.ProxyRow | null>(null)
  const [expiresOn, setExpiresOn] = useState('')
  const paste = useRef<HTMLTextAreaElement>(null)

  const load = useCallback(() => { Api.ListProxies().then((r) => setRows(r ?? [])).catch((e) => toast.error(errorText(e))) }, [toast])
  useEffect(load, [load])
  useEffect(() => onDataChanged(['proxies', 'profiles'], load), [load])

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
    const res = await Api.ImportProxies(text, expiresOn)
    if (res.added > 0) toast.success(t.proxies.added(res.added))
    setFailedLines((res.failed ?? []).map((f) => t.proxies.failedLine(f.line, f.err)))
    setText('') // pasted passwords should not linger on screen
    setExpiresOn('')
  })

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.total(rows.length)}>
        <button className={`btn nodrag${busy === 'all' ? ' busy' : ''}`} aria-busy={busy === 'all'} disabled={!!busy || rows.length === 0}
          onClick={() => run('all', () => Api.CheckAllProxies())}>
          <Icon name="refresh" />{t.proxies.checkAll}
        </button>
      </PageHead>
      <div className="pagebody">
        <section className="card pad">
          <h2>{t.proxies.addTitle}</h2>
          <p className="desc">{t.proxies.addDesc}</p>
          <textarea id="proxy-paste" ref={paste} className="textarea code" rows={4} value={text} placeholder={t.proxies.pastePlaceholder}
            spellCheck={false} autoComplete="off" onChange={(e) => setText(e.target.value)} aria-label={t.proxies.addTitle} />          <div className="row">
            <button className={`btn primary${busy === 'import' ? ' busy' : ''}`} aria-busy={busy === 'import'} disabled={!!busy || !text.trim()} onClick={importList}>
              <Icon name={busy === 'import' ? 'refresh' : 'plus'} />{busy === 'import' ? t.proxies.importing : t.proxies.import}
            </button>
            <label className="row" style={{ fontSize: 'var(--t-sm)' }}>
              {t.proxies.expiresOn}
              <input type="date" className="input" style={{ width: 150 }} value={expiresOn} onChange={(e) => setExpiresOn(e.target.value)} />
            </label>
            <span className="row muted" style={{ fontSize: 'var(--t-xs)' }}><Icon name="key" className="icon sm" />{t.proxies.passwordNote}</span>
          </div>
          {failedLines.length > 0 && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{failedLines.map((e) => <div key={e}>{e}</div>)}</span></div>}
        </section>

        <section className="card fill" style={{ minHeight: 220 }}>
          <div className="tablewrap">
            {rows.length === 0 ? (
              <EmptyState icon="proxy" title={t.proxies.emptyTitle}>
                <button className="btn" onClick={() => paste.current?.focus()}>{t.proxies.emptyAction}</button>
              </EmptyState>
            ) : (
              <table className="table">
                <thead>
                  <tr><th>{t.proxies.colLabel}</th><th>{t.proxies.colExit}</th>
                    <th>{t.proxies.colStatus}</th><th>{t.proxies.expires}</th><th>{t.proxies.colUsedBy}</th><th className="col-actions" /></tr>
                </thead>
                <tbody>
                  {rows.map((p) => (
                    <tr key={p.id}>
                      <td className="cellname"><b className="selectable">{p.label}</b><span className="mono">{p.scheme}://{p.host}:{p.port}</span>{p.note && <small className="faint ellipsis">{p.note}</small>}</td>
                      <td>{p.exitIp ? (
                        <span className="cellproxy">
                          <span className="cc">{p.country || '?'}</span>
                          <span><span className="mono selectable">{p.exitIp}</span>{p.timezone && <span className="tzline">{p.timezone}</span>}</span>
                        </span>
                      ) : <span className="faint">{t.proxies.unchecked}</span>}</td>
                      <td className="stack nowrap"><ProxyState p={p} />{p.checkedAt && <small className="faint">{t.proxies.checkedAgo(relativeTime(p.checkedAt))}</small>}</td>
                      <td className="stack nowrap">
                        {p.expiresOn && <span className="muted">{new Date(p.expiresOn + 'T00:00:00').toLocaleDateString(language)}</span>}
                        {p.expiry === 'soon' && <span className="chip warn"><span className="dot" />{t.proxies.expiringSoon}</span>}
                        {p.expiry === 'expired' && <span className="chip danger"><span className="dot" />{t.proxies.expired}</span>}
                      </td>
                      <td>{p.usedBy.length ? p.usedBy.map((n) => <span key={n} className="chip accent" style={{ marginRight: 4 }}>{n}</span>) : <span className="faint">{t.proxies.unused}</span>}</td>
                      <td className="col-actions"><span className="row">
                        <button className={`iconbtn sm${busy === p.id ? ' busy' : ''}`} aria-busy={busy === p.id} disabled={!!busy} onClick={() => run(p.id, () => Api.CheckProxy(p.id))} aria-label={t.proxies.check} title={t.proxies.check}><Icon name="refresh" /></button>
                        <button className="iconbtn sm" disabled={!!busy} onClick={() => setEditing(p)} aria-label={t.proxies.rename} title={t.proxies.rename}><Icon name="pencil" /></button>
                        <button className="iconbtn sm" disabled={!!busy} onClick={async () => { if (await confirm({ ...t.confirm.deleteProxy, danger: true })) run(p.id, () => Api.DeleteProxy(p.id)) }} aria-label={t.common.delete} title={t.common.delete}><Icon name="trash" /></button>
                      </span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </section>
      </div>
      {editing && <ProxyEditDialog row={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); load(); onChanged() }} />}
    </>
  )
}
