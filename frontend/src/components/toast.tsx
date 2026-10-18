// SPDX-License-Identifier: AGPL-3.0-or-later
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { t } from '../i18n'
import { Icon, type IconName } from './icons'

export type ToastKind = 'success' | 'info' | 'error'
export type ToastAction = { label: string; run: () => void }
type Toast = { id: number; kind: ToastKind; message: string; action?: ToastAction }

export type ToastApi = {
  success: (message: string, action?: ToastAction) => void
  info: (message: string, action?: ToastAction) => void
  error: (message: string, action?: ToastAction) => void
}

const MAX = 3
const LIFETIME = 4000
const icons: Record<ToastKind, IconName> = { success: 'check', info: 'info', error: 'alert' }
const noop: ToastApi = { success: () => {}, info: () => {}, error: () => {} }
const ToastContext = createContext<ToastApi>(noop)

/** Transient feedback in the bottom-right corner. Lasting conditions stay as banners. */
export function useToast(): ToastApi {
  return useContext(ToastContext)
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const next = useRef(1)
  const dismiss = useCallback((id: number) => setToasts((cur) => cur.filter((x) => x.id !== id)), [])
  const push = useCallback((kind: ToastKind, message: string, action?: ToastAction) => {
    if (!message) return
    const id = next.current++
    setToasts((cur) => [...cur, { id, kind, message, action }].slice(-MAX))
  }, [])
  const api = useMemo<ToastApi>(() => ({
    success: (m, a) => push('success', m, a),
    info: (m, a) => push('info', m, a),
    error: (m, a) => push('error', m, a),
  }), [push])

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="toasts" role="region" aria-live="polite" aria-label={t.toast.region}>
        {toasts.map((x) => <ToastItem key={x.id} toast={x} onDismiss={() => dismiss(x.id)} />)}
      </div>
    </ToastContext.Provider>
  )
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const [paused, setPaused] = useState(false)
  // Errors stay until dismissed; the rest leave after LIFETIME, paused while hovered.
  useEffect(() => {
    if (toast.kind === 'error' || paused) return
    const timer = setTimeout(onDismiss, LIFETIME)
    return () => clearTimeout(timer)
  }, [toast.kind, paused, onDismiss])

  return (
    <div className={`toast ${toast.kind}`} role={toast.kind === 'error' ? 'alert' : 'status'}
      onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      <Icon name={icons[toast.kind]} className="icon sm" />
      <span className="grow selectable">{toast.message}</span>
      {toast.action && (
        <button type="button" className="btn sm" onClick={() => { toast.action!.run(); onDismiss() }}>{toast.action.label}</button>
      )}
      <button type="button" className="iconbtn sm" onClick={onDismiss} aria-label={t.common.close} title={t.common.close}>
        <Icon name="x" />
      </button>
    </div>
  )
}
