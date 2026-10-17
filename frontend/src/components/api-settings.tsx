// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, main, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { useToast } from './toast'

/** The local API: off by default, loopback only, one token kept in Go. */
export function APISettings() {
  const [st, setSt] = useState<main.APIStatus | null>(null)
  const [port, setPort] = useState('')
  const [error, setError] = useState('')
  const confirm = useConfirm()
  const toast = useToast()
  useEffect(() => { Api.APIStatus().then((s) => { setSt(s); setPort(String(s.port)) }).catch((e) => setError(errorText(e))) }, [])

  const save = async (enabled: boolean, p: number) => {
    setError('')
    try {
      const next = await Api.SetAPISettings(service.APISettings.createFrom({ enabled, port: p }))
      setSt(next)
      setPort(String(next.port))
    } catch (e) {
      setError(errorText(e))
    }
  }
  const run = async (f: () => Promise<void>, msg: string) => {
    try {
      await f()
      toast.success(msg)
    } catch (e) {
      setError(errorText(e))
    }
  }
  if (!st) return error ? <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div> : null

  const savePort = () => {
    const n = Number(port)
    if (n !== st.port) save(st.enabled, n)
  }
  const newToken = async () => {
    if (!(await confirm(t.api.newTokenConfirm))) return
    run(() => Api.NewAPIToken(), t.api.tokenCopied)
  }

  return (
    <div className="field">
      <div className="seg" role="group" aria-label={t.api.title}>
        <button type="button" aria-pressed={!st.enabled} onClick={() => save(false, st.port)}>{t.api.off}</button>
        <button type="button" aria-pressed={st.enabled} onClick={() => save(true, st.port)}>{t.api.on}</button>
      </div>
      <label className="field">
        <span className="label">{t.api.port}</span>
        <input className="input" type="number" min={1024} max={65535} value={port} style={{ maxWidth: 140 }}
          onChange={(e) => setPort(e.target.value)} onBlur={savePort} onKeyDown={(e) => { if (e.key === 'Enter') savePort() }} />
      </label>
      {st.enabled && (st.running
        ? <span className="chip ok" style={{ alignSelf: 'flex-start' }}><span className="dot" />{t.api.listening(st.port)}</span>
        : st.error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{errorText(st.error)}</span></div>)}
      <span className="row">
        <button type="button" className="btn sm" disabled={!st.enabled} onClick={() => run(() => Api.CopyAPIToken(), t.api.tokenCopied)}>
          <Icon name="copy" className="icon sm" />{t.api.copyToken}
        </button>
        <button type="button" className="btn sm ghost" disabled={!st.enabled} onClick={newToken}>{t.api.newToken}</button>
      </span>
      <span className="hint">{t.api.hint}</span>
      <div className="field">
        <span className="label">{t.api.mcpTitle}</span>
        <code className="mono selectable">http://127.0.0.1:{st.port}/mcp</code>
        <span className="row">
          <button type="button" className="btn sm" disabled={!st.enabled} onClick={() => run(() => Api.CopyMCPCommand(), t.api.mcpCopied)}>
            <Icon name="copy" className="icon sm" />{t.api.mcpCopy}
          </button>
          <button type="button" className="btn sm ghost" onClick={() => Api.OpenAgentFolder('uploads').catch((e) => setError(errorText(e)))}>{t.api.openUploads}</button>
          <button type="button" className="btn sm ghost" onClick={() => Api.OpenAgentFolder('recipes').catch((e) => setError(errorText(e)))}>{t.api.openRecipes}</button>
        </span>
        <span className="hint">{t.api.foldersHint}</span>
        <span className="hint">{t.api.mcpHint}</span>
      </div>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}
