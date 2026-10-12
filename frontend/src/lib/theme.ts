// SPDX-License-Identifier: AGPL-3.0-or-later
export type ThemeChoice = 'system' | 'light' | 'dark'

const KEY = 'anyprofile.theme'

export function readTheme(storage: Pick<Storage, 'getItem'> = localStorage): ThemeChoice {
  const v = storage.getItem(KEY)
  return v === 'light' || v === 'dark' || v === 'system' ? v : 'system'
}

export function resolveTheme(choice: ThemeChoice, prefersDark: boolean): 'light' | 'dark' {
  if (choice === 'system') return prefersDark ? 'dark' : 'light'
  return choice
}

/** Applies the choice now and keeps following the OS while it is "system". */
export function applyTheme(choice: ThemeChoice): () => void {
  localStorage.setItem(KEY, choice)
  const mq = window.matchMedia('(prefers-color-scheme: dark)')
  const set = () => { document.documentElement.dataset.theme = resolveTheme(choice, mq.matches) }
  set()
  mq.addEventListener('change', set)
  return () => mq.removeEventListener('change', set)
}
