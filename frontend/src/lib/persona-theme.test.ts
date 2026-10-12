// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { personaLabel } from './persona'
import { readTheme, resolveTheme } from './theme'

describe('personaLabel', () => {
  it('reads Fury macOS persona ids', () => {
    expect(personaLabel('macos-m5-1470x956')).toBe('Mac M5 · 1470×956')
    expect(personaLabel('macos-m3-pro-1728x1117')).toBe('Mac M3 Pro · 1728×1117')
    expect(personaLabel('macos-m4pro-1728x1117')).toBe('Mac M4 Pro · 1728×1117')
  })
  it('passes unknown ids through', () => {
    expect(personaLabel('windows-11-rtx')).toBe('windows-11-rtx')
  })
})

describe('theme', () => {
  it('defaults to system and ignores junk', () => {
    expect(readTheme({ getItem: () => null })).toBe('system')
    expect(readTheme({ getItem: () => 'purple' })).toBe('system')
    expect(readTheme({ getItem: () => 'dark' })).toBe('dark')
  })
  it('resolves system from the OS preference', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
    expect(resolveTheme('light', true)).toBe('light')
  })
})
