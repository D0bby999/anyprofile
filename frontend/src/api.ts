// SPDX-License-Identifier: AGPL-3.0-or-later
// The only module that touches the generated Wails bindings.
export * as Api from '../wailsjs/go/main/App'
export { engine, main, service, site, store, netproxy } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { BrowserOpenURL, ClipboardSetText } from '../wailsjs/runtime/runtime'
import { debounce } from './lib/debounce'

export type LaunchInfo = { profileId: string; state: string; pid: number; cdpPort: number; error: string }
export type EngineProgress = { engine: string; version?: string; done: number; total: number }
export type DataKind = 'profiles' | 'proxies' | 'sites' | 'settings' | 'snapshots' | 'extensions' | 'update' | 'agent' | 'engines'

/** Write to the OS clipboard; navigator.clipboard may be blocked in the WebView. */
export const copyText = (s: string) => ClipboardSetText(s)

/** Opens an https link in the default browser, outside every profile. */
export function openExternal(url: string) {
  if (new URL(url).protocol !== 'https:') throw new Error(`not an https link: ${url}`)
  BrowserOpenURL(url)
}

/** Runs fn (debounced) when the backend reports a write of one of these kinds. */
export function onDataChanged(kinds: DataKind[], fn: () => void): () => void {
  const run = debounce(fn, 150)
  const off = EventsOn('data:changed', (kind: DataKind) => { if (kinds.includes(kind)) run() })
  return () => { run.cancel(); off() }
}

export function onLaunchChanged(fn: (info: LaunchInfo) => void): () => void {
  return EventsOn('launch:changed', fn)
}

export function onEngineProgress(fn: (p: EngineProgress) => void): () => void {
  return EventsOn('engine:progress', fn)
}

/** Download progress of an update, in bytes; total is -1 when unknown. */
/** A choice in the macOS menu bar: "settings" or "check-update". */
export function onMenu(fn: (what: string) => void): () => void {
  return EventsOn('menu', fn)
}

export function onUpdateProgress(fn: (done: number, total: number) => void): () => void {
  return EventsOn('update:progress', fn)
}
