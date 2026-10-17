// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { Icon } from './icons'

/** Shown while an AI agent works in the profile's browser. */
export function AgentChip() {
  return <span className="chip accent" title={t.agent.title}><Icon name="spark" className="icon xs" />{t.agent.label}</span>
}
