// SPDX-License-Identifier: AGPL-3.0-or-later
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'
import { t } from '../i18n'
import { Dialog } from './dialog'

export type ConfirmOptions = { title: string; body: string; confirm: string; danger?: boolean }
type Ask = (opts: ConfirmOptions) => Promise<boolean>

const ConfirmContext = createContext<Ask>(() => Promise.resolve(false))

/** Asks before a destructive or surprising action. Cancel is the default button. */
export function useConfirm(): Ask {
  return useContext(ConfirmContext)
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null)
  const resolve = useRef<(ok: boolean) => void>(() => {})
  const ask = useCallback<Ask>((o) => new Promise((res) => {
    resolve.current(false) // a newer question replaces an unanswered one
    resolve.current = res
    setOpts(o)
  }), [])
  const answer = useCallback((ok: boolean) => {
    setOpts(null)
    resolve.current(ok)
    resolve.current = () => {}
  }, [])

  return (
    <ConfirmContext.Provider value={ask}>
      {children}
      {opts && (
        <Dialog title={opts.title} icon="alert" onClose={() => answer(false)} onSubmit={() => answer(true)}
          footer={<>
            <button type="button" className="btn ghost" autoFocus onClick={() => answer(false)}>{t.common.cancel}</button>
            <button type="submit" className={`btn ${opts.danger ? 'stop' : 'primary'}`}>{opts.confirm}</button>
          </>}>
          <p className="confirm-body">{opts.body}</p>
        </Dialog>
      )}
    </ConfirmContext.Provider>
  )
}
