// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, copyText, service, store } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { siteById, type Site } from '../lib/sites'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'
import { useToast } from './toast'

const fail = (e: unknown) => errorText(e)

type AccountProps = {
  profileId: string
  site: string
  account?: store.Account
  onError: (msg: string) => void
}

/** One platform's account: login, password and 2FA. The password and the 2FA
 *  secret go straight to the Keychain and never come back to the UI. */
export function AccountPanel({ profileId, site, account, onError }: AccountProps) {
  const saved = account?.login ?? ''
  const hasPassword = account?.hasPassword ?? false
  const hasTotp = account?.hasTotp ?? false
  const [login, setLogin] = useState(saved)
  const [pw, setPw] = useState('')
  const [secret, setSecret] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [showSecret, setShowSecret] = useState(false)
  const [totp, setTotp] = useState<{ code: string; secondsLeft: number } | null>(null)
  const toast = useToast()
  useEffect(() => setLogin(saved), [saved])

  const refreshTotp = () => {
    if (hasTotp) Api.TOTPCode(profileId, site).then(setTotp).catch((e) => onError(fail(e)))
  }
  useEffect(refreshTotp, [profileId, site, hasTotp])
  useEffect(() => {
    if (!totp || totp.secondsLeft <= 0) return
    const timer = setTimeout(() => setTotp({ ...totp, secondsLeft: totp.secondsLeft - 1 }), 1000)
    if (totp.secondsLeft === 1) refreshTotp()
    return () => clearTimeout(timer)
  }, [totp])

  const run = async (f: () => Promise<unknown>, after?: () => void) => {
    try {
      await f()
      after?.()
    } catch (e) {
      onError(fail(e))
    }
  }
  const commitLogin = () => { if (login.trim() !== saved) run(() => Api.SetAccountLogin(profileId, site, login.trim())) }
  const savePassword = () => { if (pw) run(() => Api.SetAccountPassword(profileId, site, pw), () => { setPw(''); setShowPw(false) }) }
  const saveSecret = () => { if (secret) run(() => Api.SetTOTP(profileId, site, secret), () => { setSecret(''); setShowSecret(false) }) }
  const copy = (text: string, msg: string) => run(() => copyText(text), () => toast.success(msg))
  const copyPassword = () => run(() => Api.CopyAccountPassword(profileId, site), () => toast.success(t.account.copiedPassword))

  return (
    <div className="account">
      <label className="field">
        <span className="label">{t.account.login}</span>
        <span className="row">
          <input className="input" autoComplete="off" value={login} onChange={(e) => setLogin(e.target.value)} onBlur={commitLogin} />
          {login && <button type="button" className="iconbtn" onClick={() => copy(login, t.menu.copiedLogin)} aria-label={t.account.copy} title={t.account.copy}><Icon name="copy" /></button>}
        </span>
      </label>

      <div className="field">
        <span className="label">{t.account.password}</span>
        {hasPassword && !showPw ? (
          <span className="row">
            <span className="chip ok"><span className="dot" />{t.account.saved}</span>
            <button type="button" className="btn sm" onClick={copyPassword} aria-label={t.account.copy}>{t.account.copy}</button>
            <button type="button" className="btn sm ghost" onClick={() => setShowPw(true)}>{t.account.replace}</button>
            <button type="button" className="btn sm ghost" onClick={() => run(() => Api.ClearAccountPassword(profileId, site))}>{t.account.remove}</button>
          </span>
        ) : (
          <span className="row">
            <input className="input" type="password" autoComplete="new-password" value={pw} onChange={(e) => setPw(e.target.value)} />
            <button type="button" className="btn sm primary" disabled={!pw} onClick={savePassword}>{t.account.savePassword}</button>
          </span>
        )}
      </div>

      <div className="field">
        <span className="label">{t.account.totp}</span>
        {hasTotp && !showSecret ? (
          totp ? (
            <span className="row">
              <span className="mono" style={{ letterSpacing: '2px', fontSize: 'var(--t-md)' }}>{totp.code.slice(0, 3)} {totp.code.slice(3)}</span>
              <span className="hint">{t.account.secondsLeft(totp.secondsLeft)}</span>
              <button type="button" className="btn sm" onClick={() => copy(totp.code, '')} aria-label={t.account.copyCode}>{t.account.copyCode}</button>
              <button type="button" className="btn sm ghost" onClick={() => run(() => Api.ClearTOTP(profileId, site))}>{t.account.remove}</button>
            </span>
          ) : (
            <span className="hint muted">{t.common.loading}</span>
          )
        ) : (
          <span className="row">
            <input className="input" value={secret} autoComplete="off" placeholder={t.account.totpPlaceholder} onChange={(e) => setSecret(e.target.value)} />
            <button type="button" className="btn sm primary" disabled={!secret} onClick={saveSecret}>{t.common.save}</button>
          </span>
        )}
      </div>
    </div>
  )
}

/** Every account of a profile: one per platform, then any whose platform was
 *  removed from the profile, which stays until the user forgets it. */
export function AccountsPanel({ row, sites, onError }: { row: service.ProfileRow; sites: Site[]; onError: (msg: string) => void }) {
  const confirm = useConfirm()
  const accounts = row.accounts ?? []
  const find = (id: string) => accounts.find((a) => a.site === id)
  const removed = accounts.filter((a) => !row.sites.includes(a.site))
  const forget = async (a: store.Account) => {
    const name = siteById(sites, a.site).name
    if (!(await confirm({ ...t.confirm.forgetAccount(name), danger: true }))) return
    try {
      await Api.ForgetAccount(row.id, a.site)
    } catch (e) {
      onError(fail(e))
    }
  }
  const head = (id: string) => {
    const s = siteById(sites, id)
    return <h3><SiteGlyph site={s} />{s.name}</h3>
  }
  return (
    <div className="accounts" aria-label={t.account.title}>
      {row.sites.map((id) => (
        <section key={id} className="dgroup">
          {head(id)}
          <AccountPanel profileId={row.id} site={id} account={find(id)} onError={onError} />
        </section>
      ))}
      {removed.length > 0 && <p className="hint">{t.account.removedNote}</p>}
      {removed.map((a) => (
        <section key={a.site} className="dgroup">
          <span className="row">
            {head(a.site)}
            <span className="spacer" />
            <button type="button" className="btn sm ghost" onClick={() => forget(a)}>{t.account.forget}</button>
          </span>
          <AccountPanel profileId={row.id} site={a.site} account={a} onError={onError} />
        </section>
      ))}
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.account.note}</span>
    </div>
  )
}
