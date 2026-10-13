// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useSites } from '../hooks/use-sites'
import { SitePicker } from './site-picker'
import { Icon } from './icons'

export function DefaultsSettings() {
  const { sites } = useSites()
  const [d, setD] = useState<service.Defaults>(service.Defaults.createFrom({ site: 'other', proxy: 'auto' }))
  const [error, setError] = useState('')

  useEffect(() => { Api.Defaults().then(setD).catch(() => {}) }, [])

  const set = async (next: service.Defaults) => {
    setD(next)
    setError('')
    try {
      await Api.SetDefaults(next)
    } catch (e) {
      setError(errorText(e))
    }
  }

  return (
    <div className="field">
      <div className="field">
        <span className="label">{t.defaults.site}</span>
        <SitePicker value={d.site} onChange={(site) => set(service.Defaults.createFrom({ ...d, site }))} sites={sites} recent={[]} />
      </div>
      <div className="field">
        <span className="label">{t.defaults.proxy}</span>
        <div className="seg" role="group" aria-label={t.defaults.proxy}>
          <button type="button" aria-pressed={d.proxy === 'auto'} onClick={() => set(service.Defaults.createFrom({ ...d, proxy: 'auto' }))}>{t.defaults.proxyAuto}</button>
          <button type="button" aria-pressed={d.proxy === 'none'} onClick={() => set(service.Defaults.createFrom({ ...d, proxy: 'none' }))}>{t.defaults.proxyNone}</button>
        </div>
      </div>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}
