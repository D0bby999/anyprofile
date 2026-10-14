// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { t } from '../../i18n'
import type { Site } from '../../lib/sites'
import { SitePicker } from '../site-picker'

type Props = {
  site: string
  startUrl: string
  sites: Site[]
  recent: string[]
  running: boolean
  onSite: (id: string) => void
  onStartUrl: (urls: string) => Promise<boolean>
}

/** Platform saves on pick; start pages save on leaving the field. */
export function PlatformGroup({ site, startUrl, sites, recent, running, onSite, onStartUrl }: Props) {
  const [urls, setUrls] = useState(startUrl)
  const [invalid, setInvalid] = useState(false)
  useEffect(() => setUrls(startUrl), [startUrl])
  const commit = async () => {
    if (urls.trim() === startUrl.trim()) return
    setInvalid(!(await onStartUrl(urls.trim())))
  }
  return (
    <section className="dgroup" aria-label={t.drawer.platform}>
      <h3>{t.drawer.platform}</h3>
      <SitePicker value={site} onChange={onSite} sites={sites} recent={recent} />
      <textarea className="textarea" rows={2} value={urls} placeholder={t.create.startUrlHint} aria-label={t.create.startUrl}
        aria-invalid={invalid} onChange={(e) => { setUrls(e.target.value); setInvalid(false) }} onBlur={commit} />
      {running && <span className="hint">{t.drawer.nextTime}</span>}
    </section>
  )
}
