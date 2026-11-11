// SPDX-License-Identifier: AGPL-3.0-or-later
// Answers the generated Wails bindings with fixtures so the UI renders in a
// plain browser. Loaded before src/main.tsx by preview.html; never bundled into
// the app (Vite only builds index.html).
import { defaults, engines, mobileAvailability, mobileDevices, profiles, proxies } from './fixtures'

let proActive = true
// ?theme=light|dark and ?pro=0 pin the two states a screenshot needs.
const params = new URLSearchParams(location.search)
const wanted = params.get('theme')
if (wanted === 'light' || wanted === 'dark') localStorage.setItem('anyprofile.theme', wanted)
if (params.get('pro') === '0') proActive = false

const noop = () => {}
const unsubscribe = () => noop

const previewProxies = [...proxies]

const answers: Record<string, unknown> = {
  Language: async () => 'en',
  Onboarded: async () => true,
  StartupError: async () => '',
  FileVaultOn: async () => true,
  ListProfiles: async () => profiles,
  ListProxies: async () => previewProxies,
  AddProxy: async (input: { label?: string; scheme?: string; host: string; port: number; username?: string; note?: string; expiresOn?: string }) => {
    const row = {
      id: `px-${Date.now()}`,
      label: input.label?.trim() || `${input.host}:${input.port}`,
      scheme: input.scheme || 'http',
      host: input.host,
      port: input.port,
      username: input.username || '',
      exitIp: '198.51.100.42',
      country: 'US',
      timezone: 'America/New_York',
      exitIp6: '',
      timezone6: '',
      lat: 40.71,
      lon: -74.0,
      rotating: false,
      checkedAt: new Date().toISOString(),
      failedAt: '',
      createdAt: new Date().toISOString(),
      note: input.note || '',
      expiresOn: input.expiresOn || '',
      usedBy: [],
      expiry: '',
      assignable: true,
    }
    previewProxies.push(row)
    return { proxy: row, checkErr: '' }
  },
  Engines: async () => engines,
  Defaults: async () => defaults,
  ProStatus: async () => ({ active: proActive, plan: 'Lifetime', expiresAt: '', seats: 3, devices: [] }),
  ProDevices: async () => [],
  TakeWhatsNew: async () => ({ version: '', notes: '' }),
  UpdateStatus: async () => ({ current: '0.2.0', latest: '0.2.0', available: false, checking: false, error: '' }),
  Sites: async () => [
    { id: 'facebook', name: 'Facebook', group: 'social', domains: ['facebook.com'], startUrl: 'https://www.facebook.com/', icon: 'facebook', color: '#0866FF' },
    { id: 'instagram', name: 'Instagram', group: 'social', domains: ['instagram.com'], startUrl: 'https://www.instagram.com/', icon: 'instagram', color: '#E4405F' },
    { id: 'tiktok', name: 'TikTok', group: 'social', domains: ['tiktok.com'], startUrl: 'https://www.tiktok.com/', icon: 'tiktok', color: '#000000' },
    { id: 'youtube', name: 'YouTube', group: 'video', domains: ['youtube.com'], startUrl: 'https://www.youtube.com/', icon: 'youtube', color: '#FF0000' },
    { id: 'other', name: 'Other', group: '', domains: [], startUrl: '', icon: '', color: '#64748B' },
  ],
  ListMobileDevices: async () => mobileDevices,
  MobileAvailability: async () => mobileAvailability,
  MobileFastStart: async () => true,
  MaxConcurrentLaunches: async () => 0,
  SetMaxConcurrentLaunches: async () => {},
  AgentProfiles: async () => [],
  ListExtensions: async () => [],
  APIStatus: async () => ({ enabled: false, port: 0, running: false }),
}

const app = new Proxy(
  {},
  {
    get(_target, prop) {
      const name = String(prop)
      if (name in answers) return answers[name]
      // Anything the preview does not model: a list-shaped empty answer for
      // readers, null for writers, so a screen renders instead of crashing.
      return async () => (/^(List|Search|Find|Get)/.test(name) ? [] : null)
    },
  },
)

// The Wails runtime injects dozens of helpers; the preview answers any of them
// so a new import in app code cannot blank the page. Event subscriptions hand
// back an unsubscribe function, readers resolve an empty value.
const runtime = new Proxy(
  {},
  {
    get(_target, prop) {
      const name = String(prop)
      if (name.startsWith('Events')) return () => unsubscribe
      if (name === 'ClipboardSetText') return async () => true
      if (/^(Get|Window|Browser)/.test(name)) return async () => ''
      return noop
    },
  },
)

;(window as unknown as Record<string, unknown>).go = {
  main: { App: app },
  service: {},
  store: {},
}
;(window as unknown as Record<string, unknown>).runtime = runtime
