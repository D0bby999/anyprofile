// SPDX-License-Identifier: AGPL-3.0-or-later
// Every user-facing string lives in en.ts and vi.ts. English is the default.
import { en, type Messages } from './en'
import { vi } from './vi'

export type Language = 'en' | 'vi'
export const catalogs: Record<Language, Messages> = { en, vi }
export let t: Messages = en
export let language: Language = 'en'

export function setLanguage(lang: string): Language {
  language = lang === 'vi' ? 'vi' : 'en'
  t = catalogs[language]
  if (typeof document !== 'undefined') document.documentElement.lang = language
  return language
}

export type { Messages }
