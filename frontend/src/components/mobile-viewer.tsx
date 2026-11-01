// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type Props = { row: service.MobileRow; onClose: () => void }

/** In-app preview of a running device: a JPEG every 400 ms, tap to touch.
 *  It is a fallback for a quick look; the scrcpy window stays the smooth
 *  way to scroll, type and drag. */
export function MobileViewer({ row, onClose }: Props) {
  const [src, setSrc] = useState('')
  const [err, setErr] = useState('')
  const alive = useRef(true)
  const timer = useRef(0)

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
      if (alive.current) timer.current = window.setTimeout(tick, 400)
    }
    tick()
    return () => {
      alive.current = false
      window.clearTimeout(timer.current)
    }
  }, [refresh])

  const tap = (e: MouseEvent<HTMLImageElement>) => {
    const r = e.currentTarget.getBoundingClientRect()
    const w = row.screenW || 720
    const h = row.screenH || 1600
    const x = Math.round(((e.clientX - r.left) / r.width) * w)
    const y = Math.round(((e.clientY - r.top) / r.height) * h)
    Api.TapMobileDevice(row.id, x, y).catch((e2) => setErr(errorText(e2)))
  }

  const key = (k: string) => Api.PressMobileKey(row.id, k).catch((e) => setErr(errorText(e)))

  return (
    <Dialog
      title={row.name}
      icon="mobile"
      onClose={onClose}
      className="mobileviewer"
      footer={
        <>
          <button type="button" className="btn" onClick={() => key('back')}>{t.mobile.back}</button>
          <button type="button" className="btn" onClick={() => key('home')}>{t.mobile.home}</button>
          <button type="button" className="btn primary" onClick={onClose}>{t.common.close}</button>
        </>
      }
    >
      {err !== '' && <div className="banner danger" role="alert">{err}</div>}
      {src !== ''
        ? <img className="mobileviewer-image" src={src} alt={row.name} onClick={tap} />
        : <p className="hint">{t.mobile.previewLoading}</p>}
      <p className="hint">{t.mobile.previewHint}</p>
    </Dialog>
  )
}
