// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

// WorkArea is the work area of the screen a window opens on, for the app to
// hand the launcher as Launcher.Screen. Reading it needs the app's main
// thread, which is why the launcher does not reach for it by itself.
func WorkArea() (Rect, bool) { return workArea() }
