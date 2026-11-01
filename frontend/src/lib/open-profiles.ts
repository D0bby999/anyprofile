// SPDX-License-Identifier: AGPL-3.0-or-later

type Running = { id: string; state: string }

/**
 * The nth profile with a browser window (1-based), in the order the list shows
 * them — the same order Arrange windows lays them out in, so ⌘3 is the third
 * pane. A profile that is still starting has no window to raise yet.
 */
export function nthRunning<T extends Running>(rows: T[], n: number): T | undefined {
  if (!Number.isInteger(n) || n < 1) return undefined
  return rows.filter((r) => r.state === 'running')[n - 1]
}
