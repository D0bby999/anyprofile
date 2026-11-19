// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useRef, useState, type PointerEvent, type WheelEvent } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type Props = {
  row: service.MobileRow
  onClose: () => void
  onOpenGps?: () => void
}

/** MuMu-class in-app phone shell: iPhone 19.5:9 bezel + docked right toolbar +
 *  tap, drag-to-swipe and mouse-wheel scrolling. */
export function MobileViewer({ row, onClose, onOpenGps }: Props) {
  const [src, setSrc] = useState('')
  const [err, setErr] = useState('')
  const [status, setStatus] = useState('')
  const alive = useRef(true)
  const timer = useRef(0)
  const downPt = useRef<{ x: number; y: number; ts: number } | null>(null)

  const refresh = useCallback(async () => {
    try {
      const b64 = await Api.MobilePreviewFrame(row.id)
      if (alive.current) {
        setSrc(`data:image/jpeg;base64,${b64}`)
        setErr('')
      }
    } catch (e) {
      if (alive.current) setErr(errorText(e))
    }
  }, [row.id])

  useEffect(() => {
    alive.current = true
    const tick = async () => {
      await refresh()
      if (alive.current) timer.current = window.setTimeout(tick, 250)
    }
    tick()
    return () => {
      alive.current = false
      window.clearTimeout(timer.current)
    }
  }, [refresh])

  const screenPt = (e: PointerEvent<HTMLImageElement>) => {
    const r = e.currentTarget.getBoundingClientRect()
    const w = row.screenW || 780
    const h = row.screenH || 1688
    return {
      x: Math.round(((e.clientX - r.left) / r.width) * w),
      y: Math.round(((e.clientY - r.top) / r.height) * h),
    }
  }

  const onPointerDown = (e: PointerEvent<HTMLImageElement>) => {
    const pt = screenPt(e)
    downPt.current = { ...pt, ts: Date.now() }
  }

  const onPointerUp = (e: PointerEvent<HTMLImageElement>) => {
    const start = downPt.current
    downPt.current = null
    if (!start) return
    const end = screenPt(e)
    const dist = Math.hypot(end.x - start.x, end.y - start.y)
    if (dist > 18) {
      const ms = Math.min(Math.max(Date.now() - start.ts, 120), 600)
      Api.SwipeMobileDevice(row.id, start.x, start.y, end.x, end.y, ms).catch((e2) => setErr(errorText(e2)))
    } else {
      Api.TapMobileDevice(row.id, end.x, end.y).catch((e2) => setErr(errorText(e2)))
    }
  }

  const onWheel = (e: WheelEvent<HTMLImageElement>) => {
    const w = row.screenW || 780
    const h = row.screenH || 1688
    const cx = Math.round(w / 2)
    const dir = e.deltaY > 0 ? 1 : -1
    const y1 = Math.round(h * (dir > 0 ? 0.72 : 0.28))
    const y2 = Math.round(h * (dir > 0 ? 0.28 : 0.72))
    Api.SwipeMobileDevice(row.id, cx, y1, cx, y2, 220).catch((e2) => setErr(errorText(e2)))
  }

  const key = (k: string) => Api.PressMobileKey(row.id, k).catch((e) => setErr(errorText(e)))

  const runTool = async (fn: () => Promise<string | void>) => {
    setErr('')
    try {
      const msg = await fn()
      if (msg) setStatus(msg)
    } catch (e) {
      setErr(errorText(e))
    }
  }

  return (
    <Dialog
      title={row.name}
      icon="mobile"
      onClose={onClose}
      className="mobileviewer"
      footer={
        <>
          <button type="button" className="btn sm" onClick={() => key('back')}>{t.mobile.back}</button>
          <button type="button" className="btn sm" onClick={() => key('home')}>{t.mobile.home}</button>
          <button type="button" className="btn sm" onClick={() => key('recents')}>{t.mobile.recents}</button>
          <button type="button" className="btn primary sm" onClick={onClose}>{t.common.close}</button>
        </>
      }
    >
      {err !== '' && <div className="banner danger" role="alert">{err}</div>}
      {status !== '' && <div className="banner" role="status">{status}</div>}
      <div className="mobileviewer-stage">
        <div className="mobileviewer-bezel">
          <div className="mobileviewer-notch" aria-hidden="true" />
          {src !== ''
            ? <img
                className="mobileviewer-image"
                src={src}
                alt={row.name}
                draggable={false}
                onPointerDown={onPointerDown}
                onPointerUp={onPointerUp}
                onWheel={onWheel}
              />
            : <p className="hint">{t.mobile.previewLoading}</p>}
        </div>
        <aside className="mobileviewer-sidebar" aria-label="Phone controls">
          <button type="button" className="btn sm" title={t.mobile.volUp} onClick={() => key('volume_up')}>{t.mobile.volUp}</button>
          <button type="button" className="btn sm" title={t.mobile.volDown} onClick={() => key('volume_down')}>{t.mobile.volDown}</button>
          <button type="button" className="btn sm" title={t.mobile.rotate} onClick={() => runTool(() => Api.RotateMobileDevice(row.id))}>{t.mobile.rotate}</button>
          <button type="button" className="btn sm" title={t.mobile.screenshot} onClick={() => runTool(async () => t.mobile.screenshotSaved(await Api.ScreenshotMobileDevice(row.id)))}>{t.mobile.screenshot}</button>
          <button type="button" className="btn sm" title={t.mobile.pushMedia} onClick={() => runTool(async () => { const n = await Api.PushMobileMedia(row.id); if (n > 0) return t.mobile.mediaPushed(n) })}>{t.mobile.pushMedia}</button>
          <button type="button" className="btn sm" title={t.mobile.sharedFolder} onClick={() => runTool(async () => { await Api.OpenMobileSharedFolder(row.id) })}>{t.mobile.sharedFolder}</button>
          {onOpenGps && <button type="button" className="btn sm" title={t.mobile.gps} onClick={onOpenGps}>{t.mobile.gps}</button>}
        </aside>
      </div>
      <p className="hint">{t.mobile.previewHint}</p>
    </Dialog>
  )
}
