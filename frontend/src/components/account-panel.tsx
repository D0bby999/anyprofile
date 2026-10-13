// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, copyText, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Icon } from './icons'

type Props = {
  row: service.ProfileRow
  login: string
  onLogin: (v: string) => void
  onError: (msg: string) => void
}

const fail = (e: unknown) => errorText(e)

export function AccountPanel({ row, login, onLogin, onError }: Props) {
  const [pw, setPw] = useState('')
  const [secret, setSecret] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [showSecret, setShowSecret] = useState(false)
  const [totp, setTotp] = useState<{ code: string; secondsLeft: number } | null>(null)
  const [notice, setNotice] = useState('')

  const refreshTotp = () => {
    if (row.hasTotp) Api.TOTPCode(row.id).then(setTotp).catch((e) => onError(fail(e)))
  }
  useEffect(refreshTotp, [row.id, row.hasTotp])
  useEffect(() => {
    if (!totp || totp.secondsLeft <= 0) return
    const timer = setTimeout(() => setTotp({ ...totp, secondsLeft: totp.secondsLeft - 1 }), 1000)
    if (totp.secondsLeft === 1) refreshTotp()
    return () => clearTimeout(timer)
  }, [totp])

  const savePassword = async () => {
    if (!pw) return
    try {
      await Api.SetAccountPassword(row.id, pw)
      setPw('')
      setShowPw(false)
    } catch (e) {
      onError(fail(e))
    }
  }

  const saveSecret = async () => {
    if (!secret) return
    try {
      await Api.SetTOTP(row.id, secret)
      setSecret('')
      setShowSecret(false)
    } catch (e) {
      onError(fail(e))
    }
  }

  const copy = async (text: string, msg: string) => {
    try {
      await copyText(text)
      setNotice(msg)
      setTimeout(() => setNotice(''), 2000)
    } catch (e) {
      onError(fail(e))
    }
  }

  const copyPassword = async () => {
    try {
      await Api.CopyAccountPassword(row.id)
      setNotice(t.account.copiedPassword)
      setTimeout(() => setNotice(''), 2000)
    } catch (e) {
      onError(fail(e))
    }
  }

  return (
    <div className="panel">
      <span className="row"><Icon name="key" className="icon sm" /><b style={{ fontSize: 'var(--t-sm)' }}>{t.account.title}</b></span>

      <label className="field">
        <span className="label">{t.account.login}</span>
        <span className="row">
          <input className="input" autoComplete="off" value={login} onChange={(e) => onLogin(e.target.value)} />
          {login && <button type="button" className="iconbtn" onClick={() => copy(login, '')} aria-label={t.account.copy} title={t.account.copy}><Icon name="copy" /></button>}
        </span>
      </label>

      <div className="field">
        <span className="label">{t.account.password}</span>
        {row.hasPassword && !showPw ? (
          <span className="row">
            <span className="chip ok"><span className="dot" />{t.account.saved}</span>
            <button type="button" className="btn sm" onClick={copyPassword} aria-label={t.account.copy}>{t.account.copy}</button>
            <button type="button" className="btn sm ghost" onClick={() => setShowPw(true)}>{t.account.replace}</button>
            <button type="button" className="btn sm ghost" onClick={async () => {
              try { await Api.ClearAccountPassword(row.id) } catch (e) { onError(fail(e)) }
            }}>{t.account.remove}</button>
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
        {row.hasTotp && !showSecret ? (
          totp ? (
            <span className="row">
              <span className="mono" style={{ letterSpacing: '2px', fontSize: 'var(--t-md)' }}>{totp.code.slice(0, 3)} {totp.code.slice(3)}</span>
              <span className="hint">{t.account.secondsLeft(totp.secondsLeft)}</span>
              <button type="button" className="btn sm" onClick={() => copy(totp.code, '')} aria-label={t.account.copyCode}>{t.account.copyCode}</button>
              <button type="button" className="btn sm ghost" onClick={async () => {
                try { await Api.ClearTOTP(row.id) } catch (e) { onError(fail(e)) }
              }}>{t.account.remove}</button>
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

      {notice && <div className="banner ok"><Icon name="check" /><span className="grow">{notice}</span></div>}
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.account.note}</span>
    </div>
  )
}
