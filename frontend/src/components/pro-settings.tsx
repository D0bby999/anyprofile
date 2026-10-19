// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onDataChanged, onProEvent, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { useToast } from './toast'

/** Settings → Pro: buy, enter a key, and the state of the Pro pack. Nothing
 *  here reaches the server until the user presses a button or has a key, so
 *  prices show on the payment page, not here. */
export function ProSettings() {
  const [st, setSt] = useState<service.ProStatus | null>(null)
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const confirm = useConfirm()
  const toast = useToast()

  const load = () => Api.ProStatus().then(setSt).catch((e) => setError(errorText(e)))
  useEffect(() => { load() }, [])
  useEffect(() => onDataChanged(['pro'], load), [])
  useEffect(() => onProEvent((what, version) => {
    if (what === 'activated') toast.success(t.pro.activated(version))
    else toast.success(t.pro.updated(version))
    load()
  }), [])
  if (!st) return null

  async function run(what: string, fn: () => Promise<unknown>) {
    setBusy(what); setError('')
    try { await fn() } catch (e) { setError(errorText(e)) } finally { setBusy(''); load() }
  }
  const activate = () => run('activate', async () => { setSt(await Api.ProActivate(key)); setKey(''); toast.success(t.pro.activated('')) })
  const check = () => run('check', async () => {
    const before = st.version
    const now = await Api.ProCheck()
    setSt(now)
    if (now.version !== before) toast.success(t.pro.updated(now.version))
    else toast.info(t.pro.upToDate)
  })
  const remove = async () => {
    if (!(await confirm(t.pro.confirmRemove))) return
    run('remove', () => Api.ProDeactivate())
  }

  if (!st.active) {
    return (
      <div className="field">
        <span>{t.pro.pitch}</span>
        <ul className="pro-list">{t.pro.items.map((x) => <li key={x}>{x}</li>)}</ul>
        {st.waiting ? (
          <div className="banner" role="status">
            <Icon name="refresh" />
            <span className="grow">{t.pro.waiting}</span>
            <button type="button" className="btn sm ghost" onClick={() => run('cancel', () => Api.ProCancelBuy())}>{t.pro.stopWaiting}</button>
          </div>
        ) : (
          <span className="row">
            <button type="button" className="btn sm primary" disabled={busy !== ''} onClick={() => run('buy', () => Api.ProBuy('year'))}>
              {t.pro.buyYear}
            </button>
            <button type="button" className="btn sm" disabled={busy !== ''} onClick={() => run('buy', () => Api.ProBuy('month'))}>
              {t.pro.buyMonth}
            </button>
          </span>
        )}
        <span className="hint">{t.pro.buyHint}</span>
        <form className="row" onSubmit={(e) => { e.preventDefault(); if (key.trim()) activate() }}>
          <input className="input mono" value={key} onChange={(e) => setKey(e.target.value)} placeholder="APRO-XXXXX-XXXXX-XXXXX-XXXXX"
            aria-label={t.pro.keyLabel} autoComplete="off" spellCheck={false} style={{ maxWidth: 340 }} />
          <button type="submit" className="btn sm" disabled={busy !== '' || !key.trim()}>
            {busy === 'activate' ? t.pro.activating : t.pro.activate}
          </button>
        </form>
        <span className="hint">{t.pro.privacy}</span>
        {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
      </div>
    )
  }

  const until = st.paidThrough ? new Date(st.paidThrough).toLocaleDateString() : ''
  const chip = st.state === 'active' ? ['ok', t.pro.stateActive(until)]
    : st.state === 'past_due' ? ['warn', t.pro.statePastDue]
    : st.state === 'expired' ? ['warn', t.pro.stateExpired(until)]
    : st.state === 'revoked' ? ['danger', t.pro.stateRevoked]
    : ['', t.pro.statePending]
  return (
    <div className="field">
      <span className="row" style={{ flexWrap: 'wrap' }}>
        <span className={`chip ${chip[0]}`}><span className="dot" />{chip[1]}</span>
        <span className="mono" style={{ fontSize: 'var(--t-sm)' }}>{st.key}</span>
      </span>
      <span>{st.version ? t.pro.pack(st.version) : t.pro.noPack}</span>
      {st.notes && <span className="updnotes">{st.notes}</span>}
      {st.checkedAt && <span className="hint">{t.pro.checkedAt(new Date(st.checkedAt).toLocaleString())}</span>}
      <span className="row" style={{ flexWrap: 'wrap' }}>
        <button type="button" className="btn sm" disabled={busy !== ''} onClick={check}>{busy === 'check' ? t.pro.checking : t.pro.check}</button>
        <button type="button" className="btn sm ghost" disabled={!st.version} onClick={() => run('scripts', () => Api.ProOpenScripts())}>{t.pro.openScripts}</button>
        <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={() => run('portal', () => Api.ProOpenPortal())}>{t.pro.manage}</button>
        <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={remove}>{t.pro.remove}</button>
      </span>
      <span className="hint">{st.state === 'expired' ? t.pro.expiredHint : t.pro.activeHint}</span>
      {(error || st.error) && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error || errorText(st.error)}</span></div>}
    </div>
  )
}
