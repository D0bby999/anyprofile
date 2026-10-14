// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onDataChanged, store } from '../api'

/** The extension library, kept fresh when it or a profile's use of it changes. */
export function useExtensions() {
  const [exts, setExts] = useState<store.Extension[]>([])
  const reload = useCallback(() => {
    Api.ListExtensions().then((e) => setExts(e ?? [])).catch(() => {})
  }, [])
  useEffect(reload, [reload])
  useEffect(() => onDataChanged(['extensions', 'profiles'], reload), [reload])
  return { exts, reload }
}
