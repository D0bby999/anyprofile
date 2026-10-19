// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'

/** "macos-m4pro-1728x1117" -> "Mac M4 Pro · 1728×1117". Unknown ids pass through. */
export function personaLabel(id: string): string {
  const m = /^macos-(m\d)(-?pro|-?max)?-(\d+)x(\d+)$/.exec(id)
  if (!m) return id
  const tier = m[2] ? ` ${m[2].replace('-', '').replace(/^./, (c) => c.toUpperCase())}` : ''
  return `Mac ${m[1].toUpperCase()}${tier} · ${m[3]}×${m[4]}`
}

/**
 * The emulated machine of a profile. Fury names it in the persona id; other
 * engines draw it from the seed, so it is known only after the first measurement.
 */
export function machineLabel(row: { engine: string; personaId: string; machine: string }): string {
  if (row.engine === 'fury') return personaLabel(row.personaId)
  return row.machine || t.engines.machinePending
}

/** Whether the machine is known yet; lists leave an unknown one out instead of repeating the placeholder. */
export const machineKnown = (row: { engine: string; machine: string }) => row.engine === 'fury' || !!row.machine
