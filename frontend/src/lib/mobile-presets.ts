// SPDX-License-Identifier: AGPL-3.0-or-later
export const RAM_CHOICES = [2048, 3072, 4096, 6144, 8192]
export const CPU_CHOICES = [1, 2, 3, 4, 6, 8]
export const PRESETS = {
  automation: { ram: 2048, cpus: 2, screenW: 540, screenH: 1170, density: 220, withGms: false },
  balanced: { ram: 4096, cpus: 3, screenW: 780, screenH: 1688, density: 320, withGms: true },
  interactive: { ram: 4096, cpus: 6, screenW: 780, screenH: 1688, density: 320, withGms: true },
} as const
export type Preset = keyof typeof PRESETS
