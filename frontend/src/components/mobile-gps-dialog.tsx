// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

const CITIES = [
  { label: 'TP. Hồ Chí Minh', lat: 10.7769, lon: 106.7009 },
  { label: 'Hà Nội', lat: 21.0285, lon: 105.8542 },
  { label: 'Đà Nẵng', lat: 16.0544, lon: 108.2022 },
  { label: 'New York, US', lat: 40.7128, lon: -74.0060 },
  { label: 'Los Angeles, US', lat: 34.0522, lon: -118.2437 },
  { label: 'London, UK', lat: 51.5074, lon: -0.1278 },
  { label: 'Tokyo, JP', lat: 35.6762, lon: 139.6503 },
  { label: 'Seoul, KR', lat: 37.5665, lon: 126.9780 },
  { label: 'Singapore', lat: 1.3521, lon: 103.8198 },
  { label: 'Bangkok, TH', lat: 13.7563, lon: 100.5018 },
  { label: 'Berlin, DE', lat: 52.5200, lon: 13.4050 },
  { label: 'Sydney, AU', lat: -33.8688, lon: 151.2093 },
]

type Props = {
  row: service.MobileRow
  onClose: () => void
  onToast: (msg: string) => void
}

/** Virtual GPS dialog with city presets and one-click proxy geolocation sync. */
export function MobileGpsDialog({ row, onClose, onToast }: Props) {
  const [lat, setLat] = useState('10.7769')
  const [lon, setLon] = useState('106.7009')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const apply = async () => {
    setBusy(true)
    setErr('')
    try {
      await Api.SetMobileLocation(row.id, Number(lat), Number(lon))
      onToast(t.mobile.gpsUpdated)
      onClose()
    } catch (e) {
      setErr(errorText(e))
      setBusy(false)
    }
  }

  const resetProxy = async () => {
    setBusy(true)
    setErr('')
    try {
      const coords = await Api.ResetMobileLocationToProxy(row.id)
      onToast(`${t.mobile.gpsUpdated} (${coords})`)
      onClose()
    } catch (e) {
      setErr(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog
      title={`${t.mobile.gps} — ${row.name}`}
      icon="mobile"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn" disabled={busy} onClick={resetProxy}>{t.mobile.gpsResetProxy}</button>
          <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
          <button type="button" className="btn primary" disabled={busy} onClick={apply}>{t.mobile.gpsApply}</button>
        </>
      }
    >
      <div className="row" style={{ flexWrap: 'wrap', gap: 6 }}>
        {CITIES.map((c) => (
          <button key={c.label} type="button" className="btn sm" onClick={() => { setLat(String(c.lat)); setLon(String(c.lon)) }}>
            {c.label}
          </button>
        ))}
      </div>
      <div className="row" style={{ marginTop: 8 }}>
        <label className="field grow">
          <span className="label">{t.mobile.gpsLat}</span>
          <input className="input" value={lat} onChange={(e) => setLat(e.target.value)} />
        </label>
        <label className="field grow">
          <span className="label">{t.mobile.gpsLon}</span>
          <input className="input" value={lon} onChange={(e) => setLon(e.target.value)} />
        </label>
      </div>
      {Number.isFinite(Number(lat)) && Number.isFinite(Number(lon)) && (
        <iframe
          title="OpenStreetMap GPS preview"
          style={{ width: '100%', height: 180, border: '1px solid var(--border)', borderRadius: 8, marginTop: 8 }}
          src={`https://www.openstreetmap.org/export/embed.html?bbox=${Number(lon) - 0.04},${Number(lat) - 0.03},${Number(lon) + 0.04},${Number(lat) + 0.03}&layer=mapnik&marker=${Number(lat)},${Number(lon)}`}
        />
      )}
      {err && <div className="banner danger" role="alert">{err}</div>}
    </Dialog>
  )
}
