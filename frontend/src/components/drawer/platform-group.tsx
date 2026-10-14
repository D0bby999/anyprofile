// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { t } from '../../i18n'
import { syncStartUrls, type Site } from '../../lib/sites'
import { SitesPicker } from '../site-picker'

type Props = {
  value: string[]
  startUrl: string
  sites: Site[]
  recent: string[]
  running: boolean
  /** Saves the platforms together with the start pages they bring or take away. */
  onSites: (ids: string[], startUrl: string) => void
  onStartUrl: (urls: string) => Promise<boolean>
}

/** Platforms save on pick; start pages save on leaving the field. */
export function PlatformGroup({ value, startUrl, sites, recent, running, onSites, onStartUrl }: Props) {
  const [urls, setUrls] = useState(startUrl)
  const [invalid, setInvalid] = useState(false)
  useEffect(() => setUrls(startUrl), [startUrl])
  const commit = async () => {
    if (urls.trim() === startUrl.trim()) return
    setInvalid(!(await onStartUrl(urls.trim())))
  }
  const pick = (ids: string[]) => {
    const next = syncStartUrls(urls, value, ids, sites)
    setUrls(next)
    onSites(ids, next)
  }
  return (
    <section className="dgroup" aria-label={t.drawer.platform}>
      <h3>{t.drawer.platform}</h3>
      <SitesPicker value={value} onChange={pick} sites={sites} recent={recent} />
      <textarea className="textarea" rows={Math.min(5, Math.max(2, urls.split('\n').length))} value={urls}
        placeholder={t.create.startUrlHint} aria-label={t.create.startUrl}
        aria-invalid={invalid} onChange={(e) => { setUrls(e.target.value); setInvalid(false) }} onBlur={commit} />
      {running && <span className="hint">{t.drawer.nextTime}</span>}
    </section>
  )
}
