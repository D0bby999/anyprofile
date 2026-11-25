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
  mode: '',
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
  row({ id: 'p1', name: 'Bluebird - NL', state: 'running', mode: 'normal', cdpPort: 9333, position: 1, tags: ['ads', 'client-a'], lastOpenAt: ago(3), proxyLabel: 'Amsterdam 3', notes: 'Meta Ads, 3 accounts' }),
  row({ id: 'p2', name: 'Cobalt - US', state: 'running', mode: 'agent', position: 2, sites: ['tiktok', 'instagram'], tags: ['client-a'], proxyLabel: 'Ashburn 2', proxyCountry: 'US', timezone: 'America/New_York', proxyExitIp: '45.33.70.12', lastOpenAt: ago(8), fpVerdict: 'match', machine: 'Apple M2 · 1440×900' }),
  row({ id: 'p3', name: 'Lumen - DE', state: 'stopped', position: 3, tags: ['ads'], proxyLabel: 'Frankfurt 1', proxyCountry: 'DE', timezone: 'Europe/Berlin', proxyExpiry: 'soon', lastOpenAt: ago(2600), machine: 'Apple M1 · 1280×800' }),
  row({ id: 'p4', name: 'Harbor - SG', state: 'error', stateError: 'browser exited before the window appeared', fpVerdict: 'unchecked', position: 4, sites: ['youtube'], proxyLabel: 'Singapore 4', proxyCountry: 'SG', timezone: 'Asia/Singapore', lastOpenAt: ago(60) }),
  row({ id: 'p5', name: 'Quiet - CA', state: 'stopped', position: 5, tags: ['client-b'], proxyLabel: 'Toronto 2', proxyCountry: 'CA', timezone: 'America/Toronto', lastOpenAt: ago(5000), machine: 'Apple M4 · 1512×982' }),
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
  supported: true, driver: 'avo', pro: true, ready: true, reason: '', message: '',
  install: {
    runtime: { engine: 'anymobile-avo', installed: '1.0.0', latest: '1.0.0' },
    image: { engine: 'anymobile-avo-image', installed: '16968706', latest: '16968706' },
  },
} as never

export const mobileDevices = [
  {
    id: 'm1', name: 'TikTok — NL', driver: 'avo', platform: 'darwin', driverRef: 'm1', proxyId: 'px1',
    ramMb: 4096, cpus: 6, preset: 'interactive', screenW: 780, screenH: 1688, density: 320, withGms: true,
    identityJson: '{"template":"pixel8","model":"Pixel 8","brand":"google","manufacturer":"Google","imei":"353918114820193","serial":"9BE5D655392A8E","glVendor":"ARM","glRenderer":"Mali-G715"}',
    imageVersion: 'cuttlefish-16968706', notes: '', tags: ['ads', 'tiktok'],
    pinned: false, serial: 1, createdAt: ago(2400), updatedAt: ago(15), lastOpenAt: ago(5),
    backedUpAt: ago(720), deletedAt: '', state: 'running', error: '',
  },
  {
    id: 'm2', name: 'YouTube — US', driver: 'avo', platform: 'darwin', driverRef: 'm2', proxyId: 'px2',
    ramMb: 4096, cpus: 3, preset: 'balanced', screenW: 1440, screenH: 2560, density: 480, withGms: true,
    identityJson: '{"template":"s24ultra","model":"SM-S928B","brand":"samsung","manufacturer":"samsung","imei":"350894127391048","serial":"C4F18A92B70E3D","glVendor":"Qualcomm","glRenderer":"Adreno (TM) 750"}',
    imageVersion: 'cuttlefish-16968706', notes: '', tags: ['client-a'],
    pinned: true, serial: 2, createdAt: ago(4800), updatedAt: ago(300), lastOpenAt: ago(300),
    backedUpAt: '', deletedAt: '', state: 'stopped', error: '',
  },
] as never

export const mobileAPKs = [
  {
    package: 'com.zhiliaoapp.musically',
    label: 'TikTok',
    versionName: '37.4.3',
    versionCode: 2023704030,
    launchActivity: 'com.ss.android.ugc.aweme.splash.SplashActivity',
    orientation: 'portrait',
    abis: ['arm64-v8a'],
    filePath: '/Users/dobby/Library/Application Support/AnyProfile/mobile/apks/com.zhiliaoapp.musically.xapk',
    iconData: '',
    deviceId: 'm1',
    shortcutPath: '/Users/dobby/Applications/AnyProfile Apps/TikTok.app',
    installedAt: ago(900),
    lastOpenAt: ago(12),
  },
  {
    package: 'com.supercell.clashofclans',
    label: 'Clash of Clans',
    versionName: '16.512.14',
    versionCode: 16512,
    launchActivity: 'com.supercell.titan.GameApp',
    orientation: 'landscape',
    abis: ['arm64-v8a'],
    filePath: '/Users/dobby/Library/Application Support/AnyProfile/mobile/apks/com.supercell.clashofclans.apk',
    iconData: '',
    deviceId: '',
    shortcutPath: '',
    installedAt: ago(1600),
    lastOpenAt: ago(180),
  },
  {
    package: 'com.aurora.store',
    label: 'Aurora Store',
    versionName: '4.8.4',
    versionCode: 76,
    launchActivity: 'com.aurora.store.MainActivity',
    orientation: 'auto',
    abis: [],
    filePath: '/Users/dobby/Library/Application Support/AnyProfile/mobile/apks/com.aurora.store.apk',
    iconData: '',
    deviceId: 'm2',
    shortcutPath: '/Users/dobby/Applications/AnyProfile Apps/Aurora Store.app',
    installedAt: ago(2000),
    lastOpenAt: '',
  },
] as never
