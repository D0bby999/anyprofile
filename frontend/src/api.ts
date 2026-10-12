// SPDX-License-Identifier: AGPL-3.0-or-later
// The only module that touches the generated Wails bindings.
export * as Api from '../wailsjs/go/main/App'
export { engine, service, site, store, netproxy } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'

export type LaunchInfo = { profileId: string; state: string; pid: number; cdpPort: number; error: string }
export type EngineProgress = { done: number; total: number }

export function onLaunchChanged(fn: (info: LaunchInfo) => void): () => void {
  return EventsOn('launch:changed', fn)
}

export function onEngineProgress(fn: (p: EngineProgress) => void): () => void {
  return EventsOn('engine:progress', fn)
}
