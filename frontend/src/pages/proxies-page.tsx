// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onDataChanged, service } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { PageHead } from '../components/page-head'

function ProxyState({ p }: { p: service.ProxyRow }) {
  if (!p.checkedAt) return <span className="chip">{t.proxies.unchecked}</span>
  if (p.rotating) return <span className="chip warn"><span className="dot" />{t.proxies.rotating}</span>
  return <span className="chip ok"><span className="dot" />{t.proxies.ok}</span>
}

export function ProxiesPage({ onChanged }: { onChanged: () => void }) {
  const [rows, setRows] = useState<service.ProxyRow[]>([])
  const [text, setText] = useState('')
  const [busy, setBusy] = useState<string>('') // '' | 'import' | 'all' | proxy id
  const [message, setMessage] = useState('')
  const [errors, setErrors] = useState<string[]>([])

  const load = useCallback(() => { Api.ListProxies().then((r) => setRows(r ?? [])).catch((e) => setErrors([errorText(e)])) }, [])
  useEffect(load, [load])
  useEffect(() => onDataChanged(['proxies', 'profiles'], load), [load])

  const run = async (what: string, fn: () => Promise<unknown>) => {
    setBusy(what)
    setErrors([])
    setMessage('')
    try {
      await fn()
    } catch (e) {
      setErrors([errorText(e)])
    } finally {
      setBusy('')
      load()
      onChanged()
    }
  }

  const importList = () => run('import', async () => {
    const res = await Api.ImportProxies(text)
    setMessage(t.proxies.added(res.added))
    setErrors((res.failed ?? []).map((f) => t.proxies.failedLine(f.line, f.err)))
    setText('') // pasted passwords should not linger on screen
  })

  const rename = (p: service.ProxyRow) => {
    const label = window.prompt(t.proxies.renamePrompt, p.label)
    if (label && label.trim()) run(p.id, () => Api.RenameProxy(p.id, label.trim()))
  }

  return (
    <>
      <PageHead title={t.proxies.title} sub={t.proxies.total(rows.length)}>
        <button className={`btn nodrag${busy === 'all' ? ' busy' : ''}`} disabled={!!busy || rows.length === 0}
          onClick={() => run('all', () => Api.CheckAllProxies())}>
          <Icon name="refresh" />{t.proxies.checkAll}
        </button>
      </PageHead>
      <div className="pagebody">
        <section className="card pad">
          <h2>{t.proxies.addTitle}</h2>
          <p className="desc">{t.proxies.addDesc}</p>
          <textarea className="textarea code" rows={4} value={text} placeholder={t.proxies.pastePlaceholder}
            spellCheck={false} autoComplete="off" onChange={(e) => setText(e.target.value)} aria-label={t.proxies.addTitle} />
          <div className="row">
            <button className={`btn primary${busy === 'import' ? ' busy' : ''}`} disabled={!!busy || !text.trim()} onClick={importList}>
              <Icon name={busy === 'import' ? 'refresh' : 'plus'} />{busy === 'import' ? t.proxies.importing : t.proxies.import}
            </button>
            <span className="row muted" style={{ fontSize: 'var(--t-xs)' }}><Icon name="key" className="icon sm" />{t.proxies.passwordNote}</span>
          </div>
          {message && <div className="banner ok"><Icon name="check" /><span className="grow">{message}</span></div>}
          {errors.length > 0 && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{errors.map((e) => <div key={e}>{e}</div>)}</span></div>}
        </section>

        <section className="card fill" style={{ minHeight: 220 }}>
          <div className="tablewrap">
            {rows.length === 0 ? <EmptyState icon="proxy" title={t.proxies.emptyTitle} text={t.proxies.emptyText} /> : (
              <table className="table">
                <thead>
                  <tr><th>{t.proxies.colLabel}</th><th>{t.proxies.colExit}</th><th>{t.proxies.colTimezone}</th>
                    <th>{t.proxies.colState}</th><th>{t.proxies.colUsedBy}</th><th className="col-actions" /></tr>
                </thead>
                <tbody>
                  {rows.map((p) => (
                    <tr key={p.id}>
                      <td className="cellname"><b className="selectable">{p.label}</b><span className="mono">{p.scheme}://{p.host}:{p.port}</span></td>
                      <td>{p.exitIp ? <span className="cellproxy"><span className="cc">{p.country || '?'}</span><span className="mono selectable">{p.exitIp}</span></span> : <span className="faint">{t.proxies.unchecked}</span>}</td>
                      <td className="muted">{p.timezone}</td>
                      <td className="stack nowrap"><ProxyState p={p} />{p.checkedAt && <small className="faint">{t.proxies.checkedAgo(relativeTime(p.checkedAt))}</small>}</td>
                      <td>{p.usedBy.length ? p.usedBy.map((n) => <span key={n} className="chip accent" style={{ marginRight: 4 }}>{n}</span>) : <span className="faint">{t.proxies.unused}</span>}</td>
                      <td className="col-actions"><span className="row">
                        <button className={`iconbtn sm${busy === p.id ? ' busy' : ''}`} disabled={!!busy} onClick={() => run(p.id, () => Api.CheckProxy(p.id))} aria-label={t.proxies.check} title={t.proxies.check}><Icon name="refresh" /></button>
                        <button className="iconbtn sm" disabled={!!busy} onClick={() => rename(p)} aria-label={t.proxies.rename} title={t.proxies.rename}><Icon name="pencil" /></button>
                        <button className="iconbtn sm" disabled={!!busy} onClick={() => window.confirm(t.proxies.confirmDelete) && run(p.id, () => Api.DeleteProxy(p.id))} aria-label={t.common.delete} title={t.common.delete}><Icon name="trash" /></button>
                      </span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </section>
      </div>
    </>
  )
}
