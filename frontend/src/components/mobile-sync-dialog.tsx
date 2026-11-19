// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type Props = {
  running: service.MobileRow[]
  onClose: () => void
  onChanged: (msg: string) => void
}

/** MuMu-style Multi-Instance Synchronizer with anti-detect jitter. */
export function MobileSyncDialog({ running, onClose, onChanged }: Props) {
  const [masterId, setMasterId] = useState(running[0]?.id ?? '')
  const [followers, setFollowers] = useState<Set<string>>(() => new Set(running.slice(1).map((r) => r.id)))
  const [jitterOn, setJitterOn] = useState(true)
  const [delayMin, setDelayMin] = useState(50)
  const [delayMax, setDelayMax] = useState(220)
  const [offsetPx, setOffsetPx] = useState(5)
  const [active, setActive] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    Api.MobileSyncStatus().then((st) => {
      if (st.active) {
        setActive(true)
        setMasterId(st.masterId)
        setFollowers(new Set(st.followers ?? []))
        setJitterOn(st.jitter.enabled)
        if (st.jitter.delayMinMs) setDelayMin(st.jitter.delayMinMs)
        if (st.jitter.delayMaxMs) setDelayMax(st.jitter.delayMaxMs)
        if (st.jitter.offsetPx) setOffsetPx(st.jitter.offsetPx)
      }
    }).catch(() => {})
  }, [])

  const toggleFollower = (id: string) => {
    setFollowers((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const start = async () => {
    setErr('')
    const fids = [...followers].filter((id) => id !== masterId)
    try {
      await Api.StartMobileSync(masterId, fids, {
        enabled: jitterOn, delayMinMs: delayMin, delayMaxMs: delayMax, offsetPx,
      })
      onChanged(t.mobile.syncStarted(fids.length))
      onClose()
    } catch (e) {
      setErr(errorText(e))
    }
  }

  const stop = async () => {
    await Api.StopMobileSync()
    onChanged(t.mobile.syncStopped)
    onClose()
  }

  return (
    <Dialog
      title={t.mobile.sync}
      icon="mobile"
      onClose={onClose}
      footer={
        <>
          {active && <button type="button" className="btn danger" onClick={stop}>{t.mobile.syncStop}</button>}
          <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
          <button type="button" className="btn primary" disabled={!masterId || followers.size === 0} onClick={start}>
            {t.mobile.syncStart}
          </button>
        </>
      }
    >
      <label className="field">
        <span className="label">{t.mobile.syncMaster}</span>
        <select className="select" value={masterId} onChange={(e) => setMasterId(e.target.value)}>
          {running.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
        </select>
      </label>
      <div className="field">
        <span className="label">{t.mobile.syncFollowers}</span>
        <div className="stack" style={{ gap: 6 }}>
          {running.filter((r) => r.id !== masterId).map((r) => (
            <label key={r.id} className="row checkrow">
              <input type="checkbox" checked={followers.has(r.id)} onChange={() => toggleFollower(r.id)} />
              <span>{r.name}</span>
            </label>
          ))}
        </div>
      </div>
      <label className="row checkrow">
        <input type="checkbox" checked={jitterOn} onChange={(e) => setJitterOn(e.target.checked)} />
        <span>{t.mobile.syncJitter}</span>
      </label>
      {jitterOn && (
        <div className="row">
          <label className="field grow">
            <span className="label">Min ms</span>
            <input className="input" type="number" value={delayMin} onChange={(e) => setDelayMin(Number(e.target.value))} />
          </label>
          <label className="field grow">
            <span className="label">Max ms</span>
            <input className="input" type="number" value={delayMax} onChange={(e) => setDelayMax(Number(e.target.value))} />
          </label>
          <label className="field grow">
            <span className="label">± px</span>
            <input className="input" type="number" value={offsetPx} onChange={(e) => setOffsetPx(Number(e.target.value))} />
          </label>
        </div>
      )}
      {err && <div className="banner danger" role="alert">{err}</div>}
    </Dialog>
  )
}
