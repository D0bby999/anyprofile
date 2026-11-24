// SPDX-License-Identifier: AGPL-3.0-or-later
// Answers the generated Wails bindings with fixtures so the UI renders in a
// plain browser. Loaded before src/main.tsx by preview.html; never bundled into
// the app (Vite only builds index.html).
import { defaults, engines, mobileAPKs, mobileAvailability, mobileDevices, profiles, proxies } from './fixtures'

let proActive = true
// ?theme=light|dark and ?pro=0 pin the two states a screenshot needs.
const params = new URLSearchParams(location.search)
const wanted = params.get('theme')
if (wanted === 'light' || wanted === 'dark') localStorage.setItem('anyprofile.theme', wanted)
if (params.get('pro') === '0') proActive = false
const wantedTab = params.get('tab')
if (wantedTab) {
  window.addEventListener('DOMContentLoaded', () => {
    setTimeout(() => {
      const btn = [...document.querySelectorAll('nav button, .nav button')].find((b) =>
        (b.textContent || '').toLowerCase().includes(wantedTab.toLowerCase()),
      ) as HTMLButtonElement | undefined
      btn?.click()
    }, 350)
  })
}

const noop = () => {}
const unsubscribe = () => noop

const previewProxies = [...proxies]
let previewAPKs = [...(mobileAPKs as unknown as Array<Record<string, unknown>>)]

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
  MobileAvailability: async () => ({ ...(mobileAvailability as Record<string, unknown>), pro: proActive }),
  ListAPKs: async () => previewAPKs,
  PickAPKFile: async () => '/Users/dobby/Downloads/TikTok-37.4.xapk',
  InspectAPK: async () => ({
    filePath: '/Users/dobby/Downloads/TikTok-37.4.xapk',
    sizeBytes: 148 * 1024 * 1024,
    package: 'com.zhiliaoapp.musically',
    label: 'TikTok',
    versionName: '37.4.3',
    versionCode: 2023704030,
    minSdk: 21,
    targetSdk: 35,
    launchActivity: 'com.ss.android.ugc.aweme.splash.SplashActivity',
    orientation: 'portrait',
    abis: ['arm64-v8a'],
    isSplitBundle: true,
    iconData: '',
    abiCompatible: true,
    abiWarning: '',
  }),
  AddAPK: async (input: Record<string, unknown>) => {
    const rec = {
      package: 'com.zhiliaoapp.musically',
      label: (input.label as string) || 'TikTok',
      versionName: '37.4.3',
      versionCode: 2023704030,
      launchActivity: 'com.ss.android.ugc.aweme.splash.SplashActivity',
      orientation: input.orientation || 'portrait',
      abis: ['arm64-v8a'],
      filePath: input.filePath || '',
      iconData: '',
      deviceId: input.deviceId || '',
      proxyId: input.proxyId || '',
      notes: input.notes || '',
      shortcutPath: input.createShortcut ? '/Users/dobby/Applications/AnyProfile Apps/TikTok.app' : '',
      installedAt: new Date().toISOString(),
      lastOpenAt: '',
    }
    previewAPKs = [rec, ...previewAPKs.filter((a) => a.package !== rec.package)]
    return rec
  },
  UpdateAPK: async (input: Record<string, unknown>) => {
    const pkg = String(input.package || '')
    let updated: Record<string, unknown> | undefined
    previewAPKs = previewAPKs.map((a) => {
      if (a.package !== pkg) return a
      updated = { ...a, label: input.label || a.label, orientation: input.orientation || a.orientation, proxyId: input.proxyId ?? '', notes: input.notes ?? '' }
      return updated
    })
    return updated ?? previewAPKs[0]
  },
  SetAPKOrientation: async (pkg: string, orientation: string) => {
    previewAPKs = previewAPKs.map((a) => (a.package === pkg ? { ...a, orientation } : a))
  },
  SetAPKDevice: async (pkg: string, deviceId: string) => {
    previewAPKs = previewAPKs.map((a) => (a.package === pkg ? { ...a, deviceId } : a))
  },
  CreateAPKShortcut: async (pkg: string) => {
    const sc = `/Users/dobby/Applications/AnyProfile Apps/${pkg}.app`
    previewAPKs = previewAPKs.map((a) => (a.package === pkg ? { ...a, shortcutPath: sc } : a))
    return sc
  },
  DeleteAPK: async (pkg: string) => {
    previewAPKs = previewAPKs.filter((a) => a.package !== pkg)
  },
  OpenAPKStore: async () => {},
  InstallStoreAPK: async (input: Record<string, unknown>) => {
    const pkg = String(input.package || 'com.zhiliaoapp.musically')
    const rec = {
      package: pkg,
      label: String(input.label || pkg),
      versionName: '47.9.3',
      versionCode: 2024709030,
      launchActivity: '',
      orientation: String(input.orientation || 'auto'),
      abis: ['arm64-v8a'],
      filePath: `/tmp/${pkg}.apks`,
      iconData: '',
      deviceId: 'apk-runner',
      proxyId: String(input.proxyId || ''),
      notes: String(input.notes || ''),
      shortcutPath: input.createShortcut ? `/Users/dobby/Applications/AnyProfile Apps/${pkg}.app` : '',
      installedAt: new Date().toISOString(),
      lastOpenAt: new Date().toISOString(),
    }
    previewAPKs = [rec, ...previewAPKs.filter((a) => a.package !== pkg)]
    return rec
  },
  SearchAPKStore: async (query: string) => {
    const catalog = [
      { package: 'com.zhiliaoapp.musically', title: 'TikTok', developer: 'TikTok Pte. Ltd.', iconUrl: 'https://play-lh.googleusercontent.com/2o7iuqPe-0msP3DvNhzoOgNo4eZ6GkL_RtGVeKyBn5UBQAW5cw-GW5V3wDOHB1yTQuqWzrFLFOv2u4_5ea1uhg=s96', rating: '4.4', category: 'Social' },
      { package: 'com.instagram.android', title: 'Instagram', developer: 'Instagram', iconUrl: 'https://play-lh.googleusercontent.com/yHi59jmO_lVamcyJ1i3rM1_E8bAiAspShnGjjURq05ipQQSUksO3QVEsXTegRSqul038-4YNA7O644XAcx251Q=s96', rating: '4.4', category: 'Social' },
      { package: 'com.facebook.katana', title: 'Facebook', developer: 'Meta Platforms, Inc.', iconUrl: 'https://play-lh.googleusercontent.com/12KEJDhgk6oyE1VgmfCuFzFrQripZ_endElbvEhU1rZawQgV3RI-v3II8fDslI1sdC_WKBkmtxS3jxuIihk33Q=s96', rating: '4.3', category: 'Social' },
      { package: 'com.twitter.android', title: 'X', developer: 'X Corp.', iconUrl: 'https://play-lh.googleusercontent.com/IsLsCD4eLC2CYvlkgTI6Z5TROj0BJIBgUF7BZ5eliTzyyTjqG_mbofeG4kw2s7oG_JZMSi5ErLz_qsDi169C8w=s96', rating: '4.2', category: 'Social' },
      { package: 'com.whatsapp', title: 'WhatsApp Messenger', developer: 'WhatsApp LLC', iconUrl: 'https://play-lh.googleusercontent.com/Gqxk4T0uZsDwFp07DE-508hkyvcNmgFuRwPiwTEfF7D7OzGv1FdHDzEyMxNsSBZLOJlGpe3ULvVM2RgrRAlBqA=s96', rating: '4.3', category: 'Communication' },
      { package: 'org.telegram.messenger', title: 'Telegram', developer: 'Telegram FZ-LLC', iconUrl: 'https://play-lh.googleusercontent.com/PyZ3akMGXPV0tKKirKNwfO--PSQW3FHR6rD_H9mAaukZ8LiHyYFuBLeU8UZ2ok6r5SP79-3prkfybWKh98AZAD0=s96', rating: '4.5', category: 'Communication' },
      { package: 'com.lemon.lvoverseas', title: 'CapCut', developer: 'Bytedance Pte. Ltd.', iconUrl: 'https://play-lh.googleusercontent.com/M78HyakHaxKrjoeqYx41E9DXfVYYtx67nvc7Ks4G4zFQeaAJdGCi8gzzGSrHIwlrmnJS6zD9S4fAXqdEwfuHQAQ=s96', rating: '4.4', category: 'Video' },
      { package: 'com.shopee.vn', title: 'Shopee', developer: 'Shopee', iconUrl: 'https://play-lh.googleusercontent.com/mlS7AEDM9Ef-bEd_kc25xhtmJQN6hgpEEd3BQm20kIWMJNKWYfO93tuNJBT9WVcs8oZ9tpHdAPbEXI6FSsyw=s96', rating: '4.6', category: 'Shopping' },
    ]
    const q = (query || '').trim().toLowerCase()
    if (!q) return catalog
    return catalog.filter((a) =>
      a.title.toLowerCase().includes(q) || a.package.toLowerCase().includes(q) || a.developer.toLowerCase().includes(q),
    )
  },
  ListUncatalogedAPKs: async () => [
    { package: 'com.instagram.android', deviceId: 'apk-runner' },
  ],
  ImportInstalledAPK: async (input: Record<string, unknown>) => {
    const pkg = String(input.package || 'com.instagram.android')
    const rec = {
      package: pkg,
      label: String(input.label || (pkg === 'com.instagram.android' ? 'Instagram' : pkg)),
      versionName: '355.0',
      versionCode: 355000,
      launchActivity: '',
      orientation: String(input.orientation || 'portrait'),
      abis: ['arm64-v8a'],
      filePath: '',
      iconData: '',
      deviceId: 'apk-runner',
      proxyId: String(input.proxyId || ''),
      notes: String(input.notes || ''),
      shortcutPath: input.createShortcut ? '/Users/dobby/Applications/AnyProfile Apps/Instagram.app' : '',
      installedAt: new Date().toISOString(),
      lastOpenAt: '',
    }
    previewAPKs = [rec, ...previewAPKs.filter((a) => a.package !== rec.package)]
    return rec
  },
  DownloadAPKFromURL: async () => ({
    filePath: '/Users/dobby/Downloads/app-from-url.apk',
    sizeBytes: 92 * 1024 * 1024,
    package: 'org.telegram.messenger',
    label: 'Telegram',
    versionName: '11.4.2',
    versionCode: 54000,
    minSdk: 21,
    targetSdk: 35,
    launchActivity: '',
    orientation: 'portrait',
    abis: ['arm64-v8a'],
    isSplitBundle: false,
    iconData: '',
    abiCompatible: true,
    abiWarning: '',
  }),
  LaunchAPK: async () => {},
  MobileFastStart: async () => true,
  MaxConcurrentLaunches: async () => 0,
  SetMaxConcurrentLaunches: async () => {},
  AgentProfiles: async () => [],
  ListExtensions: async () => [],
  APIStatus: async () => ({ enabled: false, port: 0, running: false }),
  Storage: async () => ({
    profilesBytes: 3.9 * 1024 * 1024 * 1024,
    snapshotsBytes: 798 * 1024 * 1024,
    mobileBytes: 30 * 1024 * 1024 * 1024,
    mobileRuntimeBytes: 7.8 * 1024 * 1024 * 1024,
    enginesBytes: 1.0 * 1024 * 1024 * 1024,
    apkCacheBytes: 516 * 1024 * 1024,
    uploadsBytes: 1.2 * 1024 * 1024 * 1024,
    logsBytes: 45 * 1024 * 1024,
    totalBytes: 45.26 * 1024 * 1024 * 1024,
    orphans: [],
  }),
  SnapshotSettings: async () => ({ auto: true, keep: 1 }),
  SetSnapshotSettings: async () => {},
  ClearCache: async () => 580 * 1024 * 1024,
  ClearUploads: async () => 1.2 * 1024 * 1024 * 1024,
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
