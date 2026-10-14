// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, type ReactNode } from 'react'
import { Icon, type IconName } from './icons'
import { t } from '../i18n'

type Props = { title: string; icon: IconName; onClose: () => void; footer: ReactNode; children: ReactNode; onSubmit?: () => void }

/** A centered dialog. Escape and the backdrop close it; Enter submits its form.
 *  With dialogs stacked, only the top one answers Escape. */
export function Dialog({ title, icon, onClose, footer, children, onSubmit }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      const all = document.querySelectorAll('.backdrop')
      if (all[all.length - 1] === ref.current) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="backdrop" ref={ref} onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <form className="dialog" role="dialog" aria-modal="true" aria-label={title}
        onSubmit={(e) => { e.preventDefault(); onSubmit?.() }}>
        <div className="dialog-head">
          <Icon name={icon} />
          <h2>{title}</h2>
          <span className="spacer" />
          <button type="button" className="iconbtn sm" onClick={onClose} aria-label={t.common.close} title={t.common.close}>
            <Icon name="x" />
          </button>
        </div>
        <div className="dialog-body">{children}</div>
        <div className="dialog-foot">{footer}</div>
      </form>
    </div>
  )
}
