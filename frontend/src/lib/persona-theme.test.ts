// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { machineLabel, personaLabel } from './persona'
import { t } from '../i18n'
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

describe('machineLabel', () => {
  it('uses the persona for Fury and the measured machine for other engines', () => {
    expect(machineLabel({ engine: 'fury', personaId: 'macos-m5-1470x956', machine: '' })).toBe('Mac M5 · 1470×956')
    expect(machineLabel({ engine: 'apostate', personaId: 'apostate-macos', machine: 'Apple M3 Pro · 2560×1440' })).toBe('Apple M3 Pro · 2560×1440')
    expect(machineLabel({ engine: 'apostate', personaId: 'apostate-macos', machine: '' })).toBe(t.engines.machinePending)
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
