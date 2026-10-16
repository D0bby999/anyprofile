// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onDataChanged, type service } from '../api'

/** What the app knows about new releases; refreshed when a check finds one. */
export function useUpdate() {
  const [st, setSt] = useState<service.UpdateStatus | null>(null)
  useEffect(() => {
    const load = () => { Api.UpdateStatus().then(setSt).catch(() => {}) }
    load()
    return onDataChanged(['update'], load)
  }, [])
  return [st, setSt] as const
}
