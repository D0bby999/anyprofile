// SPDX-License-Identifier: AGPL-3.0-or-later
import { afterEach, describe, expect, it, vi } from 'vitest'

const winUA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0'

describe('on Windows', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.resetModules()
  })

  async function load() {
    vi.stubGlobal('navigator', { ...navigator, userAgent: winUA })
    vi.resetModules()
    return {
      platform: await import('./platform'),
      shortcuts: await import('./shortcuts'),
      en: (await import('../i18n/en')).en,
      vi: (await import('../i18n/vi')).vi,
    }
  }

  it('uses Ctrl as the command key and leaves the Windows key alone', async () => {
    const { platform, shortcuts } = await load()
    expect(platform.isWindows).toBe(true)
    expect(shortcuts.comboOf({ key: 'k', ctrlKey: true, metaKey: false, altKey: false })).toBe('mod+k')
    expect(shortcuts.comboOf({ key: 'k', ctrlKey: false, metaKey: true, altKey: false })).toBe('k')
  })

  it('names Windows Credential Manager, BitLocker and Ctrl', async () => {
    const { en, vi: viText } = await load()
    for (const m of [en, viText]) {
      expect(m.settings.secretsNote).toContain('Windows Credential Manager')
      expect(m.settings.fileVaultOff).toContain('BitLocker')
      expect(m.welcome.created).toContain('Ctrl+Enter')
      expect(JSON.stringify(m)).not.toMatch(/Keychain|FileVault|⌘|this Mac|máy Mac/)
    }
  })
})

describe('on a Mac', () => {
  it('keeps ⌘, the Keychain and FileVault', async () => {
    const { en } = await import('../i18n/en')
    const { comboOf } = await import('./shortcuts')
    expect(en.welcome.created).toContain('⌘Enter')
    expect(en.settings.secretsNote).toContain('Keychain')
    expect(comboOf({ key: 'k', ctrlKey: false, metaKey: true, altKey: false })).toBe('mod+k')
  })
})
