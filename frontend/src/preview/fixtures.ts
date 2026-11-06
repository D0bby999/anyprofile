// SPDX-License-Identifier: AGPL-3.0-or-later
// Fixtures the preview shell answers with. Every value here is invented: the
// shapes come from internal/store and internal/service, so the table, sidebar
// and drawer render their real layouts against plausible content.
import type { service } from '../api'

const now = new Date('2026-11-04T20:30:00+07:00')
const ago = (minutes: number) => new Date(now.getTime() - minutes * 60000).toISOString()

const row = (p: Partial<service.ProfileRow> & { id: string; name: string }): service.ProfileRow => ({
  engine: 'anybrowser',
  engineVersion: '0.5.1.157',
  engineMeta: { hwc: 12, memGiB: 24, canvasNoise: false },
  personaId: 'anybrowser-macos',
  seedHex: '9be5d655392a8efc',
  platform: 'macos',
  proxyId: 'px1',
  timezone: 'Europe/Amsterdam',
  languages: 'nl-NL,nl,en',
  uiLocale: 'nl-NL',
  lat: 52.37,
  lon: 4.89,
  sites: ['facebook'],
  serial: 1,
  accounts: [],
  extensions: [],
  fpVerdict: 'match',
  fpDiff: [],
  fpCheckedAt: ago(180),
  machine: 'Apple M4 Pro · 1512×982',
  startUrl: '',
  notes: '',
  tags: [],
  cdpNextLaunch: false,
  pinned: false,
  createdAt: ago(6000),
  updatedAt: ago(120),
  lastOpenAt: ago(120),
  deletedAt: '',
  folderId: '',
  state: 'stopped',
  stateError: '',
  cdpPort: 0,
  proxyLabel: 'Amsterdam 3',
  proxyCountry: 'NL',
  proxyExitIp: '185.104.2.31',
  proxyExpiry: '',
  lastExit: null,
  flags: [],
  checks: [],
  position: 0,
  ...p,
} as service.ProfileRow)

export const profiles: service.ProfileRow[] = [
  row({ id: 'p1', name: 'Bluebird — NL', state: 'running', cdpPort: 9333, position: 1, tags: ['ads', 'client-a'], lastOpenAt: ago(3), proxyLabel: 'Amsterdam 3', notes: 'Meta Ads, 3 accounts' }),
  row({ id: 'p2', name: 'Cobalt — US', state: 'stopped', position: 2, sites: ['tiktok', 'instagram'], tags: ['client-a'], proxyLabel: 'Ashburn 2', proxyCountry: 'US', timezone: 'America/New_York', proxyExitIp: '45.33.70.12', lastOpenAt: ago(900), fpVerdict: 'drift', fpDiff: ['webglRenderer'], machine: 'Apple M2 · 1440×900' }),
  row({ id: 'p3', name: 'Lumen — DE', state: 'stopped', position: 3, tags: ['ads'], proxyLabel: 'Frankfurt 1', proxyCountry: 'DE', timezone: 'Europe/Berlin', proxyExpiry: 'soon', lastOpenAt: ago(2600), machine: 'Apple M1 · 1280×800' }),
  row({ id: 'p4', name: 'Harbor — SG', state: 'error', stateError: 'browser exited before the window appeared', fpVerdict: 'unchecked', position: 4, sites: ['youtube'], proxyLabel: 'Singapore 4', proxyCountry: 'SG', timezone: 'Asia/Singapore', lastOpenAt: ago(60) }),
  row({ id: 'p5', name: 'Quiet — CA', state: 'stopped', position: 5, tags: ['client-b'], proxyLabel: 'Toronto 2', proxyCountry: 'CA', timezone: 'America/Toronto', lastOpenAt: ago(5000), machine: 'Apple M4 · 1512×982' }),
  row({ id: 'p6', name: 'No proxy yet', state: 'stopped', position: 6, proxyId: '', proxyLabel: '', proxyCountry: '', proxyExitIp: '', sites: ['other'], fpVerdict: '', fpCheckedAt: '', lastOpenAt: '', flags: [{ id: 'direct', level: 'info', params: {} } as never] }),
]

export const proxies = [
  { id: 'px1', label: 'Amsterdam 3', scheme: 'socks5', host: 'nl3.example.net', port: 1080, username: 'u1', exitIp: '185.104.2.31', country: 'NL', timezone: 'Europe/Amsterdam', exitIp6: '', timezone6: '', lat: 52.37, lon: 4.89, rotating: false, checkedAt: ago(30), failedAt: '', usedBy: ['p1'], assignable: true },
  { id: 'px2', label: 'Ashburn 2', scheme: 'http', host: 'us2.example.net', port: 8080, username: 'u2', exitIp: '45.33.70.12', country: 'US', timezone: 'America/New_York', exitIp6: '', timezone6: '', lat: 39.04, lon: -77.48, rotating: true, checkedAt: ago(45), failedAt: '', usedBy: ['p2'], assignable: false },
  { id: 'px3', label: 'Frankfurt 1', scheme: 'socks5', host: 'de1.example.net', port: 1080, username: 'u3', exitIp: '91.198.174.5', country: 'DE', timezone: 'Europe/Berlin', exitIp6: '', timezone6: '', lat: 50.11, lon: 8.68, rotating: false, checkedAt: ago(90), failedAt: ago(90), usedBy: ['p3'], assignable: true },
]

export const engines = [
  { engine: 'anybrowser', version: '0.5.1.157', installed: true } as never,
  { engine: 'fury', version: '0.4.5', installed: true } as never,
]

export const defaults = {
  engine: 'anybrowser', site: 'facebook', proxy: 'auto', onOpen: 'start',
  timezone: 'auto', languages: 'auto', tags: [], folder: '',
} as never

export const mobileAvailability = {
  supported: true, driver: 'vm', pro: true, ready: true, reason: '', message: '',
  install: {
    runtime: { engine: 'anymobile', installed: '1.3.0', latest: '1.3.0' },
    image: { engine: 'anymobile-image', installed: '23.2.20261019', latest: '23.2.20261019' },
  },
} as never

export const mobileDevices = [
  {
    id: 'm1', name: 'TikTok — NL', driver: 'vm', platform: 'darwin', driverRef: 'm1', proxyId: 'px1',
    ramMb: 3072, cpus: 4, imageVersion: '23.2.20261019', notes: '', tags: ['ads', 'tiktok'],
    pinned: false, serial: 1, createdAt: ago(2400), updatedAt: ago(15), lastOpenAt: ago(5),
    backedUpAt: ago(720), deletedAt: '', state: 'running', error: '',
  },
  {
    id: 'm2', name: 'Shopee — US', driver: 'vm', platform: 'darwin', driverRef: 'm2', proxyId: 'px2',
    ramMb: 3072, cpus: 4, imageVersion: '23.2.20261019', notes: '', tags: ['client-a'],
    pinned: false, serial: 2, createdAt: ago(4800), updatedAt: ago(300), lastOpenAt: ago(300),
    backedUpAt: '', deletedAt: '', state: 'stopped', error: '',
  },
] as never
