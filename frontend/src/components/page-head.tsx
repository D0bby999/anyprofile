// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'

/** Page title row. It doubles as the window's drag handle. */
export function PageHead({ title, sub, children }: { title: string; sub?: string; children?: ReactNode }) {
  return (
    <header className="pagehead drag">
      <h1>{title}</h1>
      {sub && <span className="sub">{sub}</span>}
      <span className="spacer" />
      {children}
    </header>
  )
}
