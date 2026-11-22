// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { CPU_CHOICES, MODELS, PRESETS, RAM_CHOICES, SCREEN_CHOICES, type Preset } from '../lib/mobile-presets'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { freeProxies, ProxySelect, RealIpWarning } from './proxy-select'

type Props = { existingNames: string[]; onClose: () => void; onSaved: (id: string) => void }

function nextName(names: string[]) {
  const used = new Set(names.map((name) => name.trim().toLowerCase()))
  let n = 1
  while (used.has(t.mobile.defaultName(n).toLowerCase())) n++
  return t.mobile.defaultName(n)
}

export function MobileDialog({ existingNames, onClose, onSaved }: Props) {
  const [name, setName] = useState(() => nextName(existingNames))
  const [proxyId, setProxyId] = useState('auto')
  const [proxies, setProxies] = useState<service.ProxyRow[] | null>(null)
  const [proxyError, setProxyError] = useState(false)
  const [preset, setPreset] = useState<Preset>('balanced')
  const [hardware, setHardware] = useState({ ...PRESETS.balanced } as { ram: number; cpus: number; screenW: number; screenH: number; density: number; withGms: boolean })
  const [identityTemplate, setIdentityTemplate] = useState('')
  const [notes, setNotes] = useState('')
  const [tags, setTags] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { ram, cpus, screenW, screenH, density, withGms } = hardware

  useEffect(() => {
    let active = true
    Api.ListProxies().then((p) => {
      if (!active) return
      const available = p ?? []
      setProxies(available)
      if (!freeProxies(available)) setProxyId('')
    }).catch(() => { if (active) setProxyError(true) })
    return () => { active = false }
  }, [])

  const close = () => { if (!busy) onClose() }
  const ready = !!name.trim() && (!!proxies || proxyId === '') && !busy
  const save = async () => {
    if (!ready) return
    setBusy(true)
    setError('')
    try {
      const input = service.MobileInput.createFrom({
        name: name.trim(), proxyId, ramMb: ram, cpus, preset, screenW, screenH, density, withGms,
        identityTemplate: withGms ? identityTemplate : '', notes, tags: tags.split(',').map((x) => x.trim()).filter(Boolean),
      })
      const device = await Api.CreateMobileDevice(input)
      onSaved(device.id)
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={t.mobile.dialogNew} icon="mobile" onClose={close} onSubmit={save}
      footer={<>
        <button type="button" className="btn ghost" onClick={close} disabled={busy}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" aria-busy={busy} disabled={!ready}>
          <Icon name={busy ? 'refresh' : 'plus'} className={busy ? 'icon spin' : 'icon'} />
          {busy ? t.mobile.creatingBtn : t.mobile.create}
        </button>
      </>}>
      <label className="field">
        <span className="label">{t.mobile.name}</span>
        <input className="input" autoFocus onFocus={(e) => e.currentTarget.select()} disabled={busy} value={name}
          placeholder={t.mobile.namePlaceholder} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="mobile-create-summary">
        <span>{t.mobile.deviceSpecs(cpus, ram / 1024)}</span>
        <span>{t.mobile.screen(screenW, screenH, density)}</span>
        {withGms && <span>Google Play</span>}
        <span>{proxyId === 'auto' ? t.defaults.proxyAuto : proxies?.find((p) => p.id === proxyId)?.label ?? t.create.proxyDirect}</span>
      </div>
      {proxyId === '' && <RealIpWarning text={t.mobile.realIpWarning} />}
      {proxyError && proxyId === 'auto' && <p className="hint" role="status">{t.mobile.quickProxyError}</p>}
      <details className="mobile-create-advanced">
        <summary>{t.drawer.advanced}</summary>
        <div className="mobile-create-fields">
          <div className="field">
            <span className="label">{t.mobile.proxy}</span>
            <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies ?? []} allowAuto disabled={busy || (!proxies && !proxyError)} />
          </div>
          <label className="field">
            <span className="label">{t.mobile.preset}</span>
            <select className="select" value={preset} disabled={busy} onChange={(e) => {
              const p = e.target.value as Preset
              setPreset(p)
              setHardware({ ...PRESETS[p] })
            }}>
              <option value="balanced">{t.mobile.presetBalanced}</option>
              <option value="automation">{t.mobile.presetAutomation}</option>
              <option value="interactive">{t.mobile.presetInteractive}</option>
            </select>
          </label>
          <div className="row">
            <label className="field grow">
              <span className="label">{t.mobile.cpus}</span>
              <select className="select" value={cpus} disabled={busy} onChange={(e) => setHardware({ ...hardware, cpus: Number(e.target.value) })}>
                {CPU_CHOICES.map((n) => <option key={n} value={n}>{n}</option>)}
              </select>
            </label>
            <label className="field grow">
              <span className="label">{t.mobile.ram}</span>
              <select className="select" value={ram} disabled={busy} onChange={(e) => setHardware({ ...hardware, ram: Number(e.target.value) })}>
                {RAM_CHOICES.map((mb) => <option key={mb} value={mb}>{mb / 1024} GB</option>)}
              </select>
            </label>
          </div>
          <label className="field">
            <span className="label">{t.mobile.screenPreset}</span>
            <select className="select" value={SCREEN_CHOICES.find((x) => x.w === screenW && x.h === screenH)?.key} disabled={busy}
              onChange={(e) => {
                const screen = SCREEN_CHOICES.find((x) => x.key === e.target.value)
                if (screen) setHardware({ ...hardware, screenW: screen.w, screenH: screen.h, density: screen.dpi })
              }}>
              <option value="iphone2x">{t.mobile.screenIphone2x}</option>
              <option value="fhd">{t.mobile.screenFhd}</option>
              <option value="iphone3x">{t.mobile.screenIphone3x}</option>
              <option value="qhd">{t.mobile.screenQhd}</option>
              <option value="light">{t.mobile.screenLight}</option>
            </select>
          </label>
          <label className="row checkrow">
            <input type="checkbox" checked={withGms} disabled={busy} onChange={(e) => setHardware({ ...hardware, withGms: e.target.checked })} />
            {t.mobile.withGms}
          </label>
          <span className="hint">{t.mobile.withGmsHint}</span>
          {withGms && <label className="field">
            <span className="label">{t.mobile.deviceModel}</span>
            <select className="select" value={identityTemplate} disabled={busy} onChange={(e) => setIdentityTemplate(e.target.value)}>
              <option value="">{t.mobile.modelAuto}</option>
              {MODELS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>}
          <label className="field">
            <span className="label">{t.mobile.notes}</span>
            <textarea className="textarea" rows={2} disabled={busy} value={notes} onChange={(e) => setNotes(e.target.value)} />
          </label>
          <label className="field">
            <span className="label">{t.mobile.tags}</span>
            <input className="input" disabled={busy} value={tags} onChange={(e) => setTags(e.target.value)} />
            <span className="hint">{t.mobile.tagsHint}</span>
          </label>
        </div>
      </details>
      {busy && <div className="mobile-creating-panel" role="status">
        <strong>{t.mobile.creatingTitle}</strong>
        <div className="mobile-creating-progress" role="progressbar" aria-label={t.mobile.creatingTitle}><div /></div>
        <span className="hint">{t.mobile.creatingNote}</span>
      </div>}
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
