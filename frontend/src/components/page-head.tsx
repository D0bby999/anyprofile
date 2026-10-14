// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'

/** Page title row. It doubles as the window's drag handle. A narrow page
 * centres its title over a reading-width column (see .narrow). */
export function PageHead({ title, sub, narrow, children }: { title: string; sub?: string; narrow?: boolean; children?: ReactNode }) {
  return (
    <header className={`pagehead drag${narrow ? ' narrow' : ''}`}>
      <h1>{title}</h1>
      {sub && <span className="sub">{sub}</span>}
      <span className="spacer" />
      {children}
    </header>
  )
}
