// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onLaunchChanged, service, store } from '../api'

/** Live (not trashed) profiles, kept current by launch events. One source for
 *  the sidebar counts and the table. */
export function useProfiles() {
  const [rows, setRows] = useState<service.ProfileRow[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  const reload = useCallback(() => {
    Api.ListProfiles(store.ProfileFilter.createFrom({ query: '', tag: '', trashed: false }))
      .then((r) => { setRows(r ?? []); setError('') })
      .catch((e) => setError(String(e)))
      .finally(() => setLoaded(true))
  }, [])

  useEffect(reload, [reload])
  useEffect(() => onLaunchChanged((info) => {
    setRows((cur) => cur.map((r) => (r.id !== info.profileId ? r : service.ProfileRow.createFrom({
      ...r, state: info.state, stateError: info.error, cdpPort: info.cdpPort,
      lastOpenAt: info.state === 'running' && r.state === 'starting' ? new Date().toISOString() : r.lastOpenAt,
    }))))
    if (info.error) setError(info.error)
  }), [])

  return { rows, loaded, error, setError, reload }
}
