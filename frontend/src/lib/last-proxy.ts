// SPDX-License-Identifier: AGPL-3.0-or-later
import type { service } from '../api'

const KEY = 'anyprofile.lastProxy'

/** The proxy choice of the last profile created by hand: "auto" or a proxy ID. */
export function readLastProxy(storage: Pick<Storage, 'getItem'> = localStorage): string {
  return storage.getItem(KEY) ?? ''
}

/** "No proxy" is never remembered: the real IP must always be a fresh choice. */
export function writeLastProxy(choice: string, storage: Pick<Storage, 'setItem' | 'removeItem'> = localStorage) {
  if (choice === '') storage.removeItem(KEY)
  else storage.setItem(KEY, choice)
}

/** The remembered choice while it can still be used, otherwise the fallback.
 *  Until the proxy list has loaded, the remembered choice is kept. */
export function usableLastProxy(last: string, proxies: service.ProxyRow[] | null, fallback: string): string {
  if (last === 'auto') return last
  if (!last) return fallback
  if (proxies === null) return last
  return proxies.some((p) => p.id === last && !p.rotating) ? last : fallback
}
