// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId } from 'react'
import { t } from '../i18n'
import { Icon } from './icons'

/** Shows whether an open browser is in Agent mode (locked to AI agents) or Normal mode;
 *  hover or keyboard focus explains the mode and how to switch. */
export function ModePill({ mode, state }: { mode?: string; state?: string }) {
  const id = useId()
  if (!state || state === 'stopped' || state === 'error') return null
  const isAgent = mode === 'agent'
  return (
    <span className="health mode-pill" tabIndex={0} aria-describedby={id}>
      <span className={`chip${isAgent ? ' accent' : ''}`}>
        <Icon name={isAgent ? 'spark' : 'monitor'} className="icon xs" />
        <span className="label">{isAgent ? t.mode.agent : t.mode.normal}</span>
      </span>
      <span className="healthpop" role="tooltip" id={id}>
        <strong>{isAgent ? t.mode.agentTitle : t.mode.normalTitle}</strong>
        <span>{isAgent ? t.mode.agentDesc : t.mode.normalDesc}</span>
        {isAgent && <span>{t.mode.agentTakeover}</span>}
      </span>
    </span>
  )
}
