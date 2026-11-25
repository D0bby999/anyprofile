// SPDX-License-Identifier: AGPL-3.0-or-later

type Running = { id: string; state: string; mode?: string }

/**
 * The nth profile with a browser window (1-based), in the order the list shows
 * them — the same order Arrange windows lays them out in, so ⌘3 is the third
 * pane. Headless agent-mode profiles have no window and are skipped so ⌘1–⌘9
 * never pulls an agent browser onto the screen.
 */
export function nthRunning<T extends Running>(rows: T[], n: number): T | undefined {
  if (!Number.isInteger(n) || n < 1) return undefined
  return rows.filter((r) => r.state === 'running' && r.mode !== 'agent')[n - 1]
}
