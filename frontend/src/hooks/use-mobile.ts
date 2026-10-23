// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onDataChanged, onMobileChanged, service, store } from '../api'

/** Live (not trashed) mobile devices and what stands between the user and
 *  using them, kept current by the device and data events. */
export function useMobile() {
  const [rows, setRows] = useState<service.MobileRow[]>([])
  const [avail, setAvail] = useState<service.MobileAvailability | null>(null)
  const [error, setError] = useState('')

  const reload = useCallback(() => {
    Api.ListMobileDevices(store.ProfileFilter.createFrom({ query: '', tag: '', trashed: false }))
      .then((r) => { setRows(r ?? []); setError('') })
      .catch((e) => setError(String(e)))
    Api.MobileAvailability().then(setAvail).catch(() => {})
  }, [])

  useEffect(reload, [reload])
  useEffect(() => onDataChanged(['mobile', 'proxies', 'pro'], reload), [reload])
  useEffect(() => onMobileChanged((info) => {
    setRows((cur) => cur.map((r) => (r.id !== info.deviceId ? r : service.MobileRow.createFrom({
      ...r, state: info.state, error: info.error ?? '',
    }))))
  }), [])

  return { rows, avail, error, setError, reload }
}
