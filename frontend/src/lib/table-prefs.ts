// SPDX-License-Identifier: AGPL-3.0-or-later

export type Density = 'comfortable' | 'compact'
export type Column = 'site' | 'proxy' | 'health' | 'lastOpen'
export const COLUMNS: Column[] = ['site', 'proxy', 'health', 'lastOpen']
export type TablePrefs = { density: Density; hidden: Column[] }

const KEY = 'anyprofile.table'
const DEFAULTS: TablePrefs = { density: 'comfortable', hidden: [] }
type Storage = Pick<globalThis.Storage, 'getItem' | 'setItem'>

/** Reads the table preferences; anything broken falls back to the defaults. */
export function readTablePrefs(store: Storage = localStorage): TablePrefs {
  try {
    const v = JSON.parse(store.getItem(KEY) ?? 'null')
    return {
      density: v?.density === 'compact' ? 'compact' : 'comfortable',
      hidden: Array.isArray(v?.hidden) ? COLUMNS.filter((c) => v.hidden.includes(c)) : [],
    }
  } catch {
    return { ...DEFAULTS }
  }
}

export function writeTablePrefs(p: TablePrefs, store: Storage = localStorage) {
  try {
    store.setItem(KEY, JSON.stringify(p))
  } catch {
    // storage full or blocked: the choice lasts for this session only
  }
}
