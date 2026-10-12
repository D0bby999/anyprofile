// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { Icon, type IconName } from './icons'

export function EmptyState({ icon, title, text, children }: { icon: IconName; title: string; text: string; children?: ReactNode }) {
  return (
    <div className="emptystate">
      <span className="glyph"><Icon name={icon} className="icon lg" /></span>
      <b>{title}</b>
      <p>{text}</p>
      {children && <div className="row">{children}</div>}
    </div>
  )
}
