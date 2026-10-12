// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import type { Site } from '../lib/sites'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'

type Props = { sites: Site[]; reload: () => void }

type Form = { id: string; name: string; domains: string; startUrl: string; color: string }

const emptyForm: Form = { id: '', name: '', domains: '', startUrl: '', color: '#6B7280' }

/** Manage the user's own platforms: list, add, edit and delete. */
export function SiteSettings({ sites, reload }: Props) {
  const custom = sites.filter((s) => s.custom)
  const [form, setForm] = useState<Form | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const startAdd = () => { setError(''); setForm(emptyForm) }
  const startEdit = (s: Site) => {
    setError('')
    setForm({ id: s.id, name: s.name, domains: (s.domains ?? []).join(', '), startUrl: s.startUrl, color: s.color })
  }

  const save = async () => {
    if (!form || busy) return
    setBusy(true)
    setError('')
    const in_ = service.SiteInput.createFrom({
      name: form.name,
      domains: form.domains.split(',').map((d) => d.trim()).filter(Boolean),
      startUrl: form.startUrl.trim(),
      color: form.color,
    })
    try {
      if (form.id) await Api.UpdateSite(form.id, in_)
      else await Api.CreateSite(in_)
      setForm(null)
      reload()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  const remove = async (s: Site) => {
    if (!window.confirm(t.sites.confirmDelete(s.name))) return
    setError('')
    try {
      await Api.DeleteSite(s.id)
      reload()
    } catch (e) {
      setError(errorText(e))
    }
  }

  return (
    <div className="field">
      {custom.length === 0 && !form && <p className="desc" style={{ margin: 0 }}>{t.sites.empty}</p>}
      {custom.length > 0 && (
        <table className="table">
          <tbody>
            {custom.map((s) => (
              <tr key={s.id}>
                <td className="cellname"><span className="platform"><SiteGlyph site={s} /><b>{s.name}</b></span>
                  <span className="sub"><span className="muted">{(s.domains ?? []).join(', ')}</span></span></td>
                <td className="col-actions"><span className="row">
                  <button className="iconbtn sm" onClick={() => startEdit(s)} aria-label={t.profiles.edit} title={t.profiles.edit}><Icon name="pencil" /></button>
                  <button className="iconbtn sm" onClick={() => remove(s)} aria-label={t.common.delete} title={t.common.delete}><Icon name="trash" /></button>
                </span></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {form ? (
        <div className="panel">
          <div className="grid2">
            <label className="field">
              <span className="label">{t.sites.name}</span>
              <input className="input" autoFocus value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </label>
            <label className="field">
              <span className="label">{t.sites.color}</span>
              <input type="color" value={form.color} onChange={(e) => setForm({ ...form, color: e.target.value })} aria-label={t.sites.color} />
            </label>
          </div>
          <label className="field">
            <span className="label">{t.sites.domains}</span>
            <input className="input" value={form.domains} placeholder={t.sites.domainsHint} onChange={(e) => setForm({ ...form, domains: e.target.value })} />
          </label>
          <label className="field">
            <span className="label">{t.sites.startUrl}</span>
            <input className="input" value={form.startUrl} onChange={(e) => setForm({ ...form, startUrl: e.target.value })} />
          </label>
          {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
          <div className="row">
            <button className="btn primary" disabled={busy || !form.name.trim()} onClick={save}>{t.common.save}</button>
            <button className="btn ghost" disabled={busy} onClick={() => setForm(null)}>{t.common.cancel}</button>
          </div>
        </div>
      ) : (
        <div className="row">
          <button className="btn" onClick={startAdd}><Icon name="plus" />{t.sites.add}</button>
        </div>
      )}
    </div>
  )
}
