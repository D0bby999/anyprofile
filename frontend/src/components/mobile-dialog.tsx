// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { ProxySelect, RealIpWarning } from './proxy-select'

type Props = {
  /** The device to edit; null creates one. */
  row: service.MobileRow | null
  onClose: () => void
  onSaved: (name: string) => void
}

const RAM_CHOICES = [2048, 3072, 4096, 6144, 8192]
const CPU_CHOICES = [2, 4, 6, 8]
const PRESETS = {
  automation: { ram: 2048, cpus: 2, screenW: 540, screenH: 1170, density: 220, withGms: false },
  balanced: { ram: 4096, cpus: 4, screenW: 780, screenH: 1688, density: 320, withGms: true },
  interactive: { ram: 4096, cpus: 6, screenW: 780, screenH: 1688, density: 320, withGms: true },
} as const
const SCREEN_CHOICES = [
  { key: 'iphone2x', w: 780, h: 1688, dpi: 320 },
  { key: 'iphone3x', w: 1170, h: 2532, dpi: 460 },
  { key: 'light', w: 540, h: 1170, dpi: 220 },
] as const
const MODEL_CHOICES = [
  { value: '', label: () => t.mobile.modelAuto },
  { value: 'pixel8', label: () => 'Google Pixel 8' },
  { value: 'pixel9', label: () => 'Google Pixel 9' },
  { value: 's24ultra', label: () => 'Samsung Galaxy S24 Ultra' },
  { value: 's24', label: () => 'Samsung Galaxy S24' },
  { value: 's23ultra', label: () => 'Samsung Galaxy S23 Ultra' },
  { value: 'a54', label: () => 'Samsung Galaxy A54 5G' },
  { value: 'xiaomi14pro', label: () => 'Xiaomi 14 Pro' },
  { value: 'xiaomi13ultra', label: () => 'Xiaomi 13 Ultra' },
  { value: 'oneplus12', label: () => 'OnePlus 12' },
  { value: 'oppofindx6pro', label: () => 'OPPO Find X6 Pro' },
] as const
type Preset = keyof typeof PRESETS

const splitTags = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)

/** Shows step-by-step progress and elapsed time while initializing a new Android device. */
function MobileCreatingProgress() {
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    const timer = setInterval(() => setElapsed((s) => s + 1), 1000)
    return () => clearInterval(timer)
  }, [])

  const currentStep = elapsed < 5 ? 0 : elapsed < 14 ? 1 : elapsed < 26 ? 2 : 3
  const steps = [
    t.mobile.creatingStep1,
    t.mobile.creatingStep2,
    t.mobile.creatingStep3,
    t.mobile.creatingStep4,
  ]

  const mins = Math.floor(elapsed / 60)
  const secs = elapsed % 60
  const timeStr = `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`

  return (
    <div className="mobile-creating-panel" role="status" aria-live="polite">
      <div className="mobile-creating-head">
        <div className="mobile-creating-title">
          <Icon name="refresh" className="icon spin" />
          <span>{t.mobile.creatingTitle}</span>
        </div>
        <span className="mobile-creating-timer" title="Elapsed time">{timeStr}</span>
      </div>

      <div className="mobile-creating-progress" role="progressbar" aria-label={t.mobile.creatingTitle}>
        <div />
      </div>

      <div className="mobile-creating-steps">
        {steps.map((label, idx) => {
          const status = idx < currentStep ? 'done' : idx === currentStep ? 'active' : 'pending'
          return (
            <div key={idx} className={`mobile-step ${status}`}>
              <span className="mobile-step-bullet">
                {idx < currentStep ? (
                  <Icon name="check" className="icon" />
                ) : idx === currentStep ? (
                  <span className="pulse-dot" />
                ) : (
                  <span className="pending-dot" />
                )}
              </span>
              <span className="mobile-step-text">{label}</span>
            </div>
          )
        })}
      </div>

      <p className="mobile-creating-hint">{t.mobile.creatingNote}</p>
    </div>
  )
}

/** Creates a mobile device or edits one. The proxy, memory and cores are
 *  locked while the device runs. */
export function MobileDialog({ row, onClose, onSaved }: Props) {
  const [name, setName] = useState(row?.name ?? '')
  const [proxyId, setProxyId] = useState(row ? row.proxyId : 'auto')
  const [preset, setPreset] = useState<Preset>((row?.preset as Preset) || 'balanced')
  const [ram, setRam] = useState(row?.ramMb ?? PRESETS.balanced.ram)
  const [cpus, setCpus] = useState(row?.cpus ?? PRESETS.balanced.cpus)
  const [screenW, setScreenW] = useState(row?.screenW ?? PRESETS.balanced.screenW)
  const [screenH, setScreenH] = useState(row?.screenH ?? PRESETS.balanced.screenH)
  const [density, setDensity] = useState(row?.density ?? PRESETS.balanced.density)
  const [withGms, setWithGms] = useState(row ? !!row.withGms : PRESETS.balanced.withGms)
  const [identityTemplate, setIdentityTemplate] = useState('')
  const [notes, setNotes] = useState(row?.notes ?? '')
  const [tags, setTags] = useState((row?.tags ?? []).join(', '))
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const locked = !!row && row.state !== 'stopped'

  useEffect(() => {
    Api.ListProxies().then((p) => {
      setProxies(p ?? [])
      // Without an assignable proxy, "auto" cannot pick one: fall back to choosing.
      if (!row && !(p ?? []).some((x) => x.assignable)) setProxyId('')
    }).catch(() => {})
  }, [row])

  const choosePreset = (name: Preset) => {
    setPreset(name)
    const p = PRESETS[name]
    setRam(p.ram)
    setCpus(p.cpus)
    setScreenW(p.screenW)
    setScreenH(p.screenH)
    setDensity(p.density)
    if (!row) setWithGms(p.withGms)
  }

  const chooseScreen = (key: string) => {
    const s = SCREEN_CHOICES.find((x) => x.key === key)
    if (!s) return
    setScreenW(s.w)
    setScreenH(s.h)
    setDensity(s.dpi)
  }

  const screenKey = SCREEN_CHOICES.find((x) => x.w === screenW && x.h === screenH)?.key ?? 'iphone2x'

  const save = async () => {
    if (!name.trim() || busy) return
    setBusy(true)
    setError('')
    const input = service.MobileInput.createFrom({
      name: name.trim(), proxyId, ramMb: ram, cpus, preset, screenW, screenH, density,
      withGms, identityTemplate,
      notes, tags: splitTags(tags), pinned: row?.pinned ?? false,
    })
    try {
      if (row) await Api.UpdateMobileDevice(row.id, input)
      else await Api.CreateMobileDevice(input)
      onSaved(name.trim())
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={row ? t.mobile.dialogEdit : t.mobile.dialogNew} icon="mobile" onClose={onClose} onSubmit={save}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose} disabled={busy && !row}>{t.common.cancel}</button>
        <button type="submit" className={`btn primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={!name.trim() || busy}>
          {busy && !row ? (
            <>
              <Icon name="refresh" className="icon spin" />
              <span>{t.mobile.creatingBtn}</span>
            </>
          ) : (
            row ? t.common.save : t.mobile.create
          )}
        </button>
      </>}>
      <label className="field">
        <span className="label">{t.mobile.name}</span>
        <input className="input" autoFocus disabled={busy} value={name} placeholder={t.mobile.namePlaceholder} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="field">
        <span className="label">{t.mobile.proxy}</span>
        {locked ? <span className="muted">{proxies.find((p) => p.id === proxyId)?.label ?? t.common.direct}</span> : (
          <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} allowAuto={!row} currentProfile={row?.name} disabled={busy} />
        )}
        {proxyId === '' && !locked && <RealIpWarning text={t.mobile.realIpWarning} />}
      </div>
      <label className="field">
        <span className="label">{t.mobile.preset}</span>
        <select className="select" value={preset} disabled={locked || busy} onChange={(e) => choosePreset(e.target.value as Preset)}>
          <option value="automation">{t.mobile.presetAutomation}</option>
          <option value="balanced">{t.mobile.presetBalanced}</option>
          <option value="interactive">{t.mobile.presetInteractive}</option>
        </select>
        <span className="hint">{t.mobile.presetHint}</span>
      </label>
      <div className="row">
        <label className="field grow">
          <span className="label">{t.mobile.ram}</span>
          <select className="select" value={ram} disabled={locked || busy} onChange={(e) => setRam(Number(e.target.value))}>
            {RAM_CHOICES.map((mb) => <option key={mb} value={mb}>{mb / 1024} GB</option>)}
          </select>
        </label>
        <label className="field grow">
          <span className="label">{t.mobile.cpus}</span>
          <select className="select" value={cpus} disabled={locked || busy} onChange={(e) => setCpus(Number(e.target.value))}>
            {CPU_CHOICES.map((n) => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>
      </div>
      <label className="field">
        <span className="label">{t.mobile.screenPreset}</span>
        <select className="select" value={screenKey} disabled={locked || busy} onChange={(e) => chooseScreen(e.target.value)}>
          <option value="iphone2x">{t.mobile.screenIphone2x}</option>
          <option value="iphone3x">{t.mobile.screenIphone3x}</option>
          <option value="light">{t.mobile.screenLight}</option>
        </select>
        <span className="hint">{t.mobile.screen(screenW, screenH, density)}</span>
      </label>
      {!row && (
        <>
          <label className="row checkrow">
            <input type="checkbox" checked={withGms} disabled={busy} onChange={(e) => setWithGms(e.target.checked)} />
            <span>{t.mobile.withGms}</span>
          </label>
          <span className="hint">{t.mobile.withGmsHint}</span>
          {withGms && (
            <label className="field">
              <span className="label">{t.mobile.deviceModel}</span>
              <select className="select" value={identityTemplate} disabled={busy} onChange={(e) => setIdentityTemplate(e.target.value)}>
                {MODEL_CHOICES.map((m) => <option key={m.value} value={m.value}>{m.label()}</option>)}
              </select>
            </label>
          )}
        </>
      )}
      <div className="mobile-store-hint" role="note">
        <Icon name="cart" />
        <span>{t.mobile.dialogStoreHint}</span>
      </div>
      {locked && <span className="hint">{t.mobile.closedOnly}</span>}
      <label className="field">
        <span className="label">{t.mobile.notes}</span>
        <textarea className="textarea" rows={2} disabled={busy} value={notes} onChange={(e) => setNotes(e.target.value)} />
      </label>
      <label className="field">
        <span className="label">{t.mobile.tags}</span>
        <input className="input" disabled={busy} value={tags} onChange={(e) => setTags(e.target.value)} />
        <span className="hint">{t.mobile.tagsHint}</span>
      </label>
      {busy && !row && <MobileCreatingProgress />}
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
