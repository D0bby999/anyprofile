// SPDX-License-Identifier: AGPL-3.0-or-later
// The app's wording and shortcuts follow the computer it runs on. WebView2
// reports Windows in its user agent; WKWebView and the test DOM do not.

export const isWindows = typeof navigator !== 'undefined' && /Windows/.test(navigator.userAgent)

/** The modifier key as written in the interface: "⌘" on a Mac, "Ctrl+" on Windows. */
export const modLabel = isWindows ? 'Ctrl+' : '⌘'

/** Whether the platform's command modifier is down (⌘ on a Mac, Ctrl on Windows). */
export function modDown(e: Pick<KeyboardEvent, 'metaKey' | 'ctrlKey'>): boolean {
  return isWindows ? e.ctrlKey : e.metaKey
}
