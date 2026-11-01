// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onDataChanged, onProEvent, service } from '../api'
import { t } from '../i18n'
import { errorText, formatDate, formatDateTime } from '../lib/format'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { useToast } from './toast'

/** Settings → Pro. Account is the primary path; buying and key recovery are
 *  separated so the common case never looks like a settings form. */
export function ProSettings() {
  const [st, setSt] = useState<service.ProStatus | null>(null)
  const [account, setAccount] = useState<service.ProAccountStatus | null>(null)
  const [key, setKey] = useState('')
  const [gift, setGift] = useState('')
  const [advanced, setAdvanced] = useState(false)
  type Device = { install_id: string; name: string; platform: string; last_seen: number; current: boolean }
  const [devices, setDevices] = useState<Device[]>([])
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const confirm = useConfirm()
  const toast = useToast()

  const load = () => Api.ProStatus().then(setSt).catch((e) => setError(errorText(e)))
  const loadAccount = () => Api.ProAccountStatus().then(setAccount).catch(() => setAccount(null))
  const loadDevices = () => Api.ProDevices().then((d) => setDevices(d as Device[])).catch(() => setDevices([]))
  useEffect(() => { load(); loadAccount() }, [])
  useEffect(() => { if (st?.active) loadDevices() }, [st?.active])
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
  const redeem = () => run('gift', async () => { setSt(await Api.ProRedeemGift(gift)); setGift(''); toast.success(t.pro.gifted) })
  const check = () => run('check', async () => {
    const before = st.version
    const now = await Api.ProCheck()
    setSt(now)
    if (now.version !== before) toast.success(t.pro.updated(now.version))
    else toast.info(t.pro.upToDate)
  })
  const signIn = () => run('account', async () => {
    const next = await Api.ProAccountLogin()
    setAccount(next)
    if (next.signedIn) toast.success(next.email ? t.pro.signedInAs(next.email) : t.pro.activated(''))
    else setError(t.pro.accountSignInFailed)
  })
  const signOut = () => run('signout', async () => {
    await Api.ProAccountSignOut()
    setAccount({ signedIn: false, email: '', name: '' } as service.ProAccountStatus)
  })
  const linkKey = () => run('link', async () => {
    const next = await Api.ProAccountLink(key)
    setAccount(next); setKey(''); toast.success(t.pro.linked)
  })
  const remove = async () => {
    if (!(await confirm(t.pro.confirmRemove))) return
    run('remove', () => Api.ProDeactivate())
  }

  const accountBlock = (
    <div className="pro-block">
      <div className="pro-block-head">
        <div>
          <h4 className="pro-block-title">{t.pro.accountTitle}</h4>
          <p className="pro-block-sub">{t.pro.accountHint}</p>
        </div>
        {account?.signedIn
          ? <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={signOut}>{t.pro.signOut}</button>
          : <button type="button" className="btn sm primary" disabled={busy !== ''} onClick={signIn}>
              {busy === 'account' ? t.pro.signingIn : t.pro.signIn}
            </button>}
      </div>
      {account?.signedIn && (
        <div className="pro-account-line">
          <span className="pro-account-email"><Icon name="check" className="icon sm" />{account.email}</span>
          {account.license?.status && <span className="chip">{account.license.status}</span>}
          {account.license && <button type="button" className="btn sm" disabled={busy !== ''}
            onClick={() => run('account-activate', async () => { setSt(await Api.ProAccountActivate()); toast.success(t.pro.accountActivated) })}>
            {t.pro.useAccount}
          </button>}
        </div>
      )}
      {account?.signedIn && !account.license && <p className="pro-note">{t.pro.accountNoEntitlement}</p>}
    </div>
  )

  const advancedBlock = (
    <div className="pro-advanced">
      <button type="button" className="btn sm ghost" onClick={() => setAdvanced((v) => !v)} aria-expanded={advanced}>
        <Icon name={advanced ? 'x' : 'key'} className="icon sm" />{advanced ? t.pro.hideAdvanced : t.pro.haveKey}
      </button>
      {advanced && (
        <div className="pro-block pro-advanced-body">
          <p className="pro-block-sub">{t.pro.advancedHint}</p>
          <form className="pro-form" onSubmit={(e) => { e.preventDefault(); if (key.trim()) (account?.signedIn ? linkKey() : activate()) }}>
            <input className="input mono" value={key} onChange={(e) => setKey(e.target.value)} placeholder="APRO-XXXXX-XXXXX-XXXXX-XXXXX"
              aria-label={t.pro.keyLabel} autoComplete="off" spellCheck={false} />
            <button type="submit" className="btn sm" disabled={busy !== '' || !key.trim()}>
              {busy === 'activate' ? t.pro.activating : account?.signedIn ? t.pro.linkKey : t.pro.activate}
            </button>
          </form>
          <form className="pro-form" onSubmit={(e) => { e.preventDefault(); if (gift.trim()) redeem() }}>
            <input className="input mono" value={gift} onChange={(e) => setGift(e.target.value)} placeholder="APRO-GIFT-XXXX-XXXX-XXXX-XXXX"
              aria-label={t.pro.giftLabel} autoComplete="off" spellCheck={false} />
            <button type="submit" className="btn sm" disabled={busy !== '' || !gift.trim()}>
              {busy === 'gift' ? t.pro.redeeming : t.pro.redeem}
            </button>
          </form>
          <p className="pro-note">{t.pro.privacy}</p>
        </div>
      )}
    </div>
  )

  if (!st.active) {
    return (
      <div className="pro-stack">
        {accountBlock}
        <div className="pro-block">
          <div className="pro-block-head">
            <h4 className="pro-block-title">{t.pro.pitch}</h4>
          </div>
          <ul className="ticks pro-features">{t.pro.items.map((x) => <li key={x}>{x}</li>)}</ul>
          {st.waiting ? (
            <div className="banner" role="status">
              <Icon name="refresh" /><span className="grow">{t.pro.waiting}</span>
              <button type="button" className="btn sm ghost" onClick={() => run('cancel', () => Api.ProCancelBuy())}>{t.pro.stopWaiting}</button>
            </div>
          ) : (
            <div className="pro-actions">
              <button type="button" className="btn sm primary" disabled={busy !== ''} onClick={() => run('buy', () => Api.ProBuy('year'))}>{t.pro.buyYear}</button>
              <button type="button" className="btn sm" disabled={busy !== ''} onClick={() => run('buy', () => Api.ProBuy('month'))}>{t.pro.buyMonth}</button>
              <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={() => run('buy-gift', () => Api.ProBuyGift())}>{t.pro.buyGift}</button>
            </div>
          )}
          <p className="pro-note">{t.pro.buyHint}</p>
        </div>
        {advancedBlock}
        {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
      </div>
    )
  }

  const until = st.paidThrough || st.licenseExpiresAt ? formatDate(st.paidThrough || st.licenseExpiresAt) : ''
  const chip = st.state === 'active' ? ['ok', t.pro.stateActive(until)]
    : st.state === 'past_due' ? ['warn', t.pro.statePastDue]
    : st.state === 'expired' ? ['warn', t.pro.stateExpired(until)]
    : st.state === 'revoked' ? ['danger', t.pro.stateRevoked]
    : ['', t.pro.statePending]
  return (
    <div className="pro-stack">
      {accountBlock}
      <div className="pro-block">
        <div className="pro-block-head">
          <div>
            <h4 className="pro-block-title">{t.pro.title}</h4>
            {st.version ? <p className="pro-block-sub">{t.pro.pack(st.version)}</p> : <p className="pro-block-sub">{t.pro.noPack}</p>}
          </div>
          <span className={`chip ${chip[0]}`}><span className="dot" />{chip[1]}</span>
        </div>
        {st.offline && st.graceUntil && <div className="banner" role="status"><Icon name="refresh" /><span className="grow">{t.pro.offlineUntil(formatDate(st.graceUntil))}</span></div>}
        <div className="pro-meta">
          {st.key && <span className="mono">{st.key}</span>}
          {st.kind && st.kind !== 'subscription' && <span className="chip">{st.kind === 'lifetime' ? t.pro.kindLifetime : st.kind === 'gift' ? t.pro.kindGift : t.pro.kindTest}</span>}
          {st.checkedAt && <span className="hint">{t.pro.checkedAt(formatDateTime(st.checkedAt))}</span>}
        </div>
        {st.notes && <p className="pro-note">{st.notes}</p>}
        <div className="pro-actions">
          <button type="button" className="btn sm" disabled={busy !== ''} onClick={check}>{busy === 'check' ? t.pro.checking : t.pro.check}</button>
          <button type="button" className="btn sm ghost" disabled={!st.version} onClick={() => run('scripts', () => Api.ProOpenScripts())}>{t.pro.openScripts}</button>
          <button type="button" className="btn sm ghost" disabled={busy !== '' || (st.source !== '' && st.source !== 'stripe')} onClick={() => run('portal', () => Api.ProOpenPortal())}>{t.pro.manage}</button>
          <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={remove}>{t.pro.remove}</button>
        </div>
        <p className="pro-note">{st.state === 'expired' ? t.pro.expiredHint : t.pro.activeHint}</p>
      </div>
      <div className="pro-block">
        <div className="pro-block-head">
          <div><h4 className="pro-block-title">{t.pro.devices}</h4><p className="pro-block-sub">{t.pro.devicesHint}</p></div>
          <button type="button" className="btn sm ghost" disabled={busy !== ''} onClick={loadDevices}>{t.pro.refreshDevices}</button>
        </div>
        {devices.length === 0 ? <p className="pro-note">{t.pro.noDevices}</p> : (
          <div className="pro-devices">{devices.map((d) => (
            <div className="pro-device" key={d.install_id}>
              <span>{d.name || d.install_id} <span className="muted">{d.current ? t.pro.currentDevice : d.platform}</span></span>
              <button type="button" className="btn sm ghost" disabled={busy !== '' || d.current}
                onClick={() => run('device', async () => { await Api.ProRemoveDevice(d.install_id); loadDevices() })}>{t.pro.removeDevice}</button>
            </div>
          ))}</div>
        )}
      </div>
      {advancedBlock}
      {(error || st.error) && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error || errorText(st.error)}</span></div>}
    </div>
  )
}
