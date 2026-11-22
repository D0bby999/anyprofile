// SPDX-License-Identifier: AGPL-3.0-or-later
export const RAM_CHOICES = [2048, 3072, 4096, 6144, 8192]
export const CPU_CHOICES = [1, 2, 3, 4, 6, 8]
export const PRESETS = {
  automation: { ram: 2048, cpus: 2, screenW: 540, screenH: 1170, density: 220, withGms: false },
  balanced: { ram: 4096, cpus: 3, screenW: 780, screenH: 1688, density: 320, withGms: true },
  interactive: { ram: 4096, cpus: 6, screenW: 780, screenH: 1688, density: 320, withGms: true },
} as const
export type Preset = keyof typeof PRESETS

export const SCREEN_CHOICES = [
  { key: 'iphone2x', w: 780, h: 1688, dpi: 320 },
  { key: 'fhd', w: 1080, h: 2400, dpi: 420 },
  { key: 'iphone3x', w: 1170, h: 2532, dpi: 460 },
  { key: 'qhd', w: 1440, h: 2560, dpi: 480 },
  { key: 'light', w: 540, h: 1170, dpi: 220 },
] as const

export const MODELS: readonly [string, string][] = [
  ['pixel8', 'Google Pixel 8'],
  ['pixel9', 'Google Pixel 9'],
  ['s24ultra', 'Samsung Galaxy S24 Ultra'],
  ['s24', 'Samsung Galaxy S24'],
  ['s23ultra', 'Samsung Galaxy S23 Ultra'],
  ['a54', 'Samsung Galaxy A54 5G'],
  ['xiaomi14pro', 'Xiaomi 14 Pro'],
  ['xiaomi13ultra', 'Xiaomi 13 Ultra'],
  ['oneplus12', 'OnePlus 12'],
  ['oppofindx6pro', 'OPPO Find X6 Pro'],
]

export type DeviceIdentity = {
  template?: string
  model?: string
  brand?: string
  manufacturer?: string
  serial?: string
  imei?: string
  glVendor?: string
  glRenderer?: string
}

export function parseIdentity(raw?: string): DeviceIdentity | null {
  if (!raw) return null
  try {
    return JSON.parse(raw) as DeviceIdentity
  } catch {
    return null
  }
}
