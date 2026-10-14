// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, store } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useExtensions } from '../hooks/use-extensions'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { useToast } from './toast'

/** The extension library: add from a .zip/.crx or a folder, delete. */
export function ExtensionSettings() {
  const { exts } = useExtensions()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const confirm = useConfirm()
  const toast = useToast()

  const add = async (folder: boolean) => {
    setError('')
    setBusy(true)
    try {
      const e = await Api.AddExtension(folder)
      if (e?.id) toast.success(t.extensions.added(e.name))
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }
  const remove = async (e: store.Extension) => {
    if (!(await confirm({ ...t.confirm.deleteExtension(e.name, e.profiles), danger: true }))) return
    try {
      await Api.DeleteExtension(e.id)
    } catch (err) {
      setError(errorText(err))
    }
  }

  return (
    <div className="field">
      {exts.length === 0 ? <span className="hint muted">{t.extensions.empty}</span> : (
        <ul className="extlist">
          {exts.map((e) => (
            <li key={e.id}>
              <Icon name="puzzle" className="icon sm" />
              <span className="grow"><b>{e.name}</b> <span className="muted">{e.version}</span></span>
              <span className="muted">{t.extensions.usedBy(e.profiles)}</span>
              <button type="button" className="iconbtn sm" aria-label={t.extensions.delete(e.name)} title={t.extensions.delete(e.name)} onClick={() => remove(e)}>
                <Icon name="trash" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <span className="row">
        <button type="button" className="btn sm" disabled={busy} onClick={() => add(false)}><Icon name="plus" className="icon sm" />{t.extensions.addFile}</button>
        <button type="button" className="btn sm" disabled={busy} onClick={() => add(true)}>{t.extensions.addFolder}</button>
      </span>
      <span className="hint">{t.extensions.hint}</span>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}

/** One profile's extensions, switched on and off from the drawer. */
export function ProfileExtensions({ profileId, on, running, onError }: { profileId: string; on: string[]; running: boolean; onError: (msg: string) => void }) {
  const { exts } = useExtensions()
  const toggle = async (id: string, value: boolean) => {
    try {
      await Api.SetExtensionMany([profileId], id, value)
    } catch (e) {
      onError(errorText(e))
    }
  }
  return (
    <section className="dgroup" aria-label={t.extensions.title}>
      <h3>{t.extensions.title}</h3>
      {exts.length === 0 ? <span className="hint muted">{t.extensions.noneInLibrary}</span> : exts.map((e) => (
        <label key={e.id} className="row checkrow">
          <input type="checkbox" checked={on.includes(e.id)} onChange={(ev) => toggle(e.id, ev.target.checked)} />
          <span>{e.name} <span className="muted">{e.version}</span></span>
        </label>
      ))}
      {running && exts.length > 0 && <span className="hint">{t.drawer.nextTime}</span>}
    </section>
  )
}
