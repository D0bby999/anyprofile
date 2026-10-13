// SPDX-License-Identifier: AGPL-3.0-or-later
/** Calls fn once, wait ms after the last trigger. cancel() drops a pending call. */
export function debounce(fn: () => void, wait: number) {
  let timer: ReturnType<typeof setTimeout> | undefined
  const trigger = () => { clearTimeout(timer); timer = setTimeout(fn, wait) }
  trigger.cancel = () => clearTimeout(timer)
  return trigger
}
