// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type MacroScript = { id: string; name: string; createdAt: string; steps?: unknown[] }

type Props = {
  running: service.MobileRow[]
  onClose: () => void
  onToast: (msg: string) => void
}

/** MuMu-style Operation Recorder: record touch/swipe/key sequences and replay with loops & speed. */
export function MobileMacroPanel({ running, onClose, onToast }: Props) {
  const [macros, setMacros] = useState<MacroScript[]>([])
  const [recDev, setRecDev] = useState(running[0]?.id ?? '')
  const [activeRec, setActiveRec] = useState('')
  const [name, setName] = useState('')
  const [loops, setLoops] = useState(1)
  const [speed, setSpeed] = useState(1)
  const [err, setErr] = useState('')

  const reload = async () => {
    try {
      const [list, rec] = await Promise.all([Api.ListMobileMacros(), Api.MobileMacroRecording()])
      setMacros((list ?? []) as MacroScript[])
      setActiveRec(rec ?? '')
    } catch (e) {
      setErr(errorText(e))
    }
  }

  useEffect(() => { reload() }, [])

  const startRec = async () => {
    setErr('')
    try {
      await Api.StartMacroRecord(recDev, name)
      setActiveRec(recDev)
    } catch (e) {
      setErr(errorText(e))
    }
  }

  const stopRec = async () => {
    setErr('')
    try {
      const sc = await Api.StopMacroRecord()
      setActiveRec('')
      setName('')
      onToast(t.mobile.macroSaved(sc.name, (sc.steps ?? []).length))
      reload()
    } catch (e) {
      setErr(errorText(e))
    }
  }

  const play = async (id: string) => {
    setErr('')
    const targets = running.map((r) => r.id)
    try {
      await Api.RunMobileMacro(id, targets, { loops, speed })
      onToast(t.mobile.macroPlay)
    } catch (e) {
      setErr(errorText(e))
    }
  }

  const remove = async (id: string) => {
    await Api.DeleteMobileMacro(id)
    reload()
  }

  return (
    <Dialog
      title={t.mobile.macro}
      icon="mobile"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn" onClick={() => Api.StopMobileMacro()}>{t.mobile.macroStopPlay}</button>
          <button type="button" className="btn primary" onClick={onClose}>{t.common.close}</button>
        </>
      }
    >
      <div className="row">
        <select className="select grow" value={recDev} disabled={!!activeRec} onChange={(e) => setRecDev(e.target.value)}>
          {running.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
        </select>
        <input className="input grow" placeholder={t.mobile.macroName} value={name} disabled={!!activeRec} onChange={(e) => setName(e.target.value)} />
        {activeRec ? (
          <button type="button" className="btn danger" onClick={stopRec}>{t.mobile.macroStopRecord}</button>
        ) : (
          <button type="button" className="btn primary" disabled={!recDev} onClick={startRec}>{t.mobile.macroRecord}</button>
        )}
      </div>
      <div className="row">
        <label className="field grow">
          <span className="label">{t.mobile.macroLoops}</span>
          <input className="input" type="number" min={1} max={100} value={loops} onChange={(e) => setLoops(Number(e.target.value))} />
        </label>
        <label className="field grow">
          <span className="label">{t.mobile.macroSpeed}</span>
          <select className="select" value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
            <option value={0.5}>0.5×</option>
            <option value={1}>1×</option>
            <option value={1.5}>1.5×</option>
            <option value={2}>2×</option>
          </select>
        </label>
      </div>
      <div className="stack" style={{ gap: 6, marginTop: 8 }}>
        {macros.length === 0 ? (
          <p className="hint">{t.mobile.macroEmpty}</p>
        ) : macros.map((m) => (
          <div key={m.id} className="row" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
            <span><strong>{m.name}</strong> <small className="faint">({(m.steps ?? []).length} steps)</small></span>
            <span className="row" style={{ gap: 6 }}>
              <button type="button" className="btn sm primary" disabled={running.length === 0} onClick={() => play(m.id)}>{t.mobile.macroPlay}</button>
              <button type="button" className="btn sm ghost" onClick={() => remove(m.id)}>{t.common.delete}</button>
            </span>
          </div>
        ))}
      </div>
      {err && <div className="banner danger" role="alert">{err}</div>}
    </Dialog>
  )
}
