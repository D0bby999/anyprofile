// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useSites } from '../hooks/use-sites'
import { SitePicker } from './site-picker'
import { Icon } from './icons'

/** The saved defaults. A change is applied to a fresh read, so two sections
 *  of the Settings page never write back each other's stale values. */
function useDefaults() {
  const [d, setD] = useState<service.Defaults>(service.Defaults.createFrom({ site: 'other', proxy: 'auto', onOpen: 'start' }))
  const [error, setError] = useState('')

  useEffect(() => { Api.Defaults().then(setD).catch(() => {}) }, [])

  const set = async (patch: Partial<service.Defaults>) => {
    setD(service.Defaults.createFrom({ ...d, ...patch }))
    setError('')
    try {
      const next = service.Defaults.createFrom({ ...(await Api.Defaults()), ...patch })
      await Api.SetDefaults(next)
      setD(next)
    } catch (e) {
      setError(errorText(e))
    }
  }
  return { d, set, error }
}

function SaveError({ error }: { error: string }) {
  return error ? <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div> : null
}

export function DefaultsSettings() {
  const { sites } = useSites()
  const { d, set, error } = useDefaults()

  return (
    <div className="field">
      <div className="field">
        <span className="label">{t.defaults.site}</span>
        <SitePicker value={d.site} onChange={(site) => set({ site })} sites={sites} />
      </div>
      <div className="field">
        <span className="label">{t.defaults.proxy}</span>
        <div className="seg" role="group" aria-label={t.defaults.proxy}>
          <button type="button" aria-pressed={d.proxy === 'auto'} onClick={() => set({ proxy: 'auto' })}>{t.defaults.proxyAuto}</button>
          <button type="button" aria-pressed={d.proxy === 'none'} onClick={() => set({ proxy: 'none' })}>{t.defaults.proxyNone}</button>
        </div>
      </div>
      <SaveError error={error} />
    </div>
  )
}

/** What every profile opens with: the tabs of last time, or its start pages. */
export function OnOpenSettings() {
  const { d, set, error } = useDefaults()
  return (
    <div className="field">
      <div className="seg" role="group" aria-label={t.onOpen.title}>
        <button type="button" aria-pressed={d.onOpen !== 'restore'} onClick={() => set({ onOpen: 'start' })}>{t.onOpen.start}</button>
        <button type="button" aria-pressed={d.onOpen === 'restore'} onClick={() => set({ onOpen: 'restore' })}>{t.onOpen.restore}</button>
      </div>
      <span className="hint">{d.onOpen === 'restore' ? t.onOpen.restoreHint : t.onOpen.startHint}</span>
      <SaveError error={error} />
    </div>
  )
}
