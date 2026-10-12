// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api } from '../api'
import type { Site } from '../lib/sites'

/** The platform catalog (built-in + custom), loaded once per mount. */
export function useSites() {
  const [sites, setSites] = useState<Site[]>([])
  const reload = useCallback(() => {
    Api.Sites().then((s) => setSites(s ?? [])).catch(() => {})
  }, [])
  useEffect(reload, [reload])
  return { sites, reload }
}
