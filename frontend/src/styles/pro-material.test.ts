// SPDX-License-Identifier: AGPL-3.0-or-later
// Guards the Pro decision: membership is a material (seal, firmer rule, deeper
// surface), never a second accent. A paid tier that recolours the working
// accent changes what a colour means for the people who paid, and the violet
// ramp it used was the default every generated UI reaches for.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const css = readFileSync(fileURLToPath(new URL('./shell.css', import.meta.url)), 'utf8')
const tokens = readFileSync(fileURLToPath(new URL('./tokens.css', import.meta.url)), 'utf8')

describe('Pro is a material, not a hue', () => {
  it('never overrides the working accent for Pro', () => {
    expect(css).not.toMatch(/is-pro[^{]*\{[^}]*--accent/)
  })

  it('carries no violet ramp (Tailwind violet/purple) anywhere', () => {
    for (const hex of ['#7c3aed', '#6d28d9', '#5b21b6', '#8b5cf6', '#a78bfa', '#c4b5fd', '#d8b4fe', '#f3e8ff', '#ddd0fb']) {
      expect((css + tokens).toLowerCase()).not.toContain(hex)
    }
  })

  it('paints the rail flat instead of washing it with a gradient', () => {
    const sidebar = /\.app\.is-pro \.sidebar[^}]*}/.exec(css)?.[0] ?? ''
    expect(sidebar).not.toMatch(/gradient/)
  })

  it('keeps one membership marker, drawn in ink', () => {
    expect(css).not.toContain('.propill')
    expect(css).toMatch(/\.seal\s*\{[^}]*border: 1px solid var\(--line-strong\)/)
  })
})
