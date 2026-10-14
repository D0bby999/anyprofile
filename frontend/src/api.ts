// SPDX-License-Identifier: AGPL-3.0-or-later
// The only module that touches the generated Wails bindings.
export * as Api from '../wailsjs/go/main/App'
export { engine, main, service, site, store, netproxy } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { ClipboardSetText } from '../wailsjs/runtime/runtime'
import { debounce } from './lib/debounce'

export type LaunchInfo = { profileId: string; state: string; pid: number; cdpPort: number; error: string }
export type EngineProgress = { engine: string; done: number; total: number }
export type DataKind = 'profiles' | 'proxies' | 'sites' | 'settings' | 'snapshots' | 'extensions'

/** Write to the OS clipboard; navigator.clipboard may be blocked in the WebView. */
export const copyText = (s: string) => ClipboardSetText(s)

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
