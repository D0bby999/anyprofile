// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Api, onDataChanged, service, store } from '../api'
import { t } from '../i18n'
import { errorText, fold } from '../lib/format'
import { MODELS } from '../lib/mobile-presets'
import type { useMobile } from '../hooks/use-mobile'
import { ApkDrawer } from '../components/apk-drawer'
import { Dialog } from '../components/dialog'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { PageHead } from '../components/page-head'
import { ProxySelect, RealIpWarning } from '../components/proxy-select'
import { useConfirm } from '../components/confirm'
import { useToast } from '../components/toast'

type Props = {
  mobile: ReturnType<typeof useMobile>
  onOpenPro: () => void
  onOpenMobile: () => void
  onChanged?: () => void
}

type OriFilter = 'all' | 'portrait' | 'landscape'
type NewTab = 'store' | 'file'

const POPULAR_APPS: ReadonlyArray<readonly [label: string, pkg: string]> = [
  ['TikTok', 'com.zhiliaoapp.musically'],
  ['Instagram', 'com.instagram.android'],
  ['Facebook', 'com.facebook.katana'],
  ['Telegram', 'org.telegram.messenger'],
  ['WhatsApp', 'com.whatsapp'],
  ['Shopee', 'com.shopee.vn'],
  ['X', 'com.twitter.android'],
  ['Zalo', 'com.zing.zalo'],
]

function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 MB'
  const mb = n / (1024 * 1024)
  return mb >= 1000 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(1)} MB`
}

/** Standalone Android APK apps: an AnyProfile Pro feature with its own background Android host. */
export function ApkPage({ mobile, onOpenPro, onOpenMobile, onChanged }: Props) {
  const { avail } = mobile
  const [apps, setApps] = useState<store.MobileAPK[]>([])
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [query, setQuery] = useState('')
  const [oriFilter, setOriFilter] = useState<OriFilter>('all')
  const [activePkg, setActivePkg] = useState<string | null>(null)
  const [busyPkg, setBusyPkg] = useState('')
  const [error, setError] = useState('')
  const [inspect, setInspect] = useState<service.APKInspect | null>(null)
  const [targetLabel, setTargetLabel] = useState('')
  const [targetProxy, setTargetProxy] = useState('')
  const [targetOri, setTargetOri] = useState('auto')
  const [targetModel, setTargetModel] = useState('')
  const [targetNotes, setTargetNotes] = useState('')
  const [makeShortcut, setMakeShortcut] = useState(true)
  const [adding, setAdding] = useState(false)

  // New APK modal state (Store deep-link + auto-sync, local file, direct URL, running Mobile import).
  const [newOpen, setNewOpen] = useState(false)
  const [newTab, setNewTab] = useState<NewTab>('store')
  const [storeQuery, setStoreQuery] = useState('')
  const [storeProxy, setStoreProxy] = useState('')
  const [openingStore, setOpeningStore] = useState(false)
  const [storeWatching, setStoreWatching] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [detected, setDetected] = useState<service.UncatalogedAPK[]>([])
  const [importingPkg, setImportingPkg] = useState('')
  const [urlInput, setUrlInput] = useState('')
  const [urlProxy, setUrlProxy] = useState('')
  const [downloadingUrl, setDownloadingUrl] = useState(false)
  const [mobileSourceId, setMobileSourceId] = useState('')
  const [mobileApps, setMobileApps] = useState<service.UncatalogedAPK[]>([])
  const [mobileScanned, setMobileScanned] = useState(false)
  const storeBaselineRef = useRef<Set<string>>(new Set())
  const importingRef = useRef<Set<string>>(new Set())

  const toast = useToast()
  const confirm = useConfirm()

  const canUse = !!avail?.pro && !!avail?.ready
  const runningMobile = useMemo(
    () => (mobile.rows ?? []).filter((r) => r.state === 'running'),
    [mobile.rows],
  )

  const reload = useCallback(() => {
    Api.ListAPKs()
      .then((list) => {
        setApps(list ?? [])
        onChanged?.()
      })
      .catch((e) => setError(errorText(e)))
  }, [onChanged])

  const reloadProxies = () => {
    Api.ListProxies()
      .then((list) => setProxies(list ?? []))
      .catch(() => {})
  }

  useEffect(reload, [reload])
  useEffect(reloadProxies, [])
  useEffect(() => onDataChanged(['apk'], reload), [reload])
  useEffect(() => onDataChanged(['proxies'], reloadProxies), [])

  const openInspect = (meta: service.APKInspect, initialProxy = '') => {
    setNewOpen(false)
    setInspect(meta)
    setTargetLabel(meta.label || '')
    setTargetProxy(initialProxy)
    setTargetOri(meta.orientation || 'auto')
    setTargetModel('')
    setTargetNotes('')
    setMakeShortcut(true)
  }

  const syncFromStore = useCallback(async (autoImportNew: boolean) => {
    if (!canUse) return
    setSyncing(true)
    try {
      const list = (await Api.ListUncatalogedAPKs('')) ?? []
      setDetected(list)
      if (!autoImportNew) return
      for (const item of list) {
        if (storeBaselineRef.current.has(item.package) || importingRef.current.has(item.package)) continue
        importingRef.current.add(item.package)
        setImportingPkg(item.package)
        try {
          const rec = await Api.ImportInstalledAPK(service.APKImportInput.createFrom({
            package: item.package,
            deviceId: item.deviceId,
            label: '',
            proxyId: storeProxy,
            notes: '',
            orientation: 'auto',
            identityTemplate: '',
            createShortcut: true,
          }))
          storeBaselineRef.current.add(item.package)
          setDetected((cur) => cur.filter((x) => x.package !== item.package))
          toast.success(t.apk.autoImported(rec.label))
          reload()
        } catch (e) {
          toast.error(errorText(e))
        } finally {
          importingRef.current.delete(item.package)
          setImportingPkg('')
        }
      }
    } catch {
      // Host may still be starting or paused.
    } finally {
      setSyncing(false)
    }
  }, [canUse, reload, storeProxy, toast])

  useEffect(() => {
    if (!storeWatching) return
    const timer = window.setInterval(() => { void syncFromStore(true) }, 3000)
    const onFocus = () => { void syncFromStore(true) }
    window.addEventListener('focus', onFocus)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('focus', onFocus)
    }
  }, [storeWatching, syncFromStore])

  const openNewDialog = () => {
    setError('')
    setNewTab('store')
    setMobileScanned(false)
    setMobileApps([])
    if (runningMobile.length > 0 && !mobileSourceId) {
      setMobileSourceId(runningMobile[0].id)
    }
    setNewOpen(true)
    void syncFromStore(false)
  }

  const openStore = async () => {
    setOpeningStore(true)
    try {
      const existing = (await Api.ListUncatalogedAPKs('')) ?? []
      storeBaselineRef.current = new Set(existing.map((x) => x.package))
      await Api.OpenAPKStore(service.APKStoreRequest.createFrom({
        query: storeQuery.trim(),
        proxyId: storeProxy,
        deviceId: '',
      }))
      setStoreWatching(true)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setOpeningStore(false)
    }
  }

  const importOne = async (item: service.UncatalogedAPK, proxyId: string) => {
    setImportingPkg(item.package)
    try {
      const rec = await Api.ImportInstalledAPK(service.APKImportInput.createFrom({
        package: item.package,
        deviceId: item.deviceId,
        label: '',
        proxyId,
        notes: '',
        orientation: 'auto',
        identityTemplate: '',
        createShortcut: true,
      }))
      storeBaselineRef.current.add(item.package)
      setDetected((cur) => cur.filter((x) => x.package !== item.package))
      setMobileApps((cur) => cur.filter((x) => x.package !== item.package))
      toast.success(t.apk.installed(rec.label))
      reload()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setImportingPkg('')
    }
  }

  const downloadFromUrl = async () => {
    const raw = urlInput.trim()
    if (!raw) return
    setDownloadingUrl(true)
    try {
      const meta = await Api.DownloadAPKFromURL(raw, urlProxy)
      setUrlInput('')
      openInspect(meta, urlProxy)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setDownloadingUrl(false)
    }
  }

  const scanMobileDevice = async () => {
    const id = mobileSourceId || runningMobile[0]?.id || ''
    if (!id) return
    setSyncing(true)
    try {
      const list = (await Api.ListUncatalogedAPKs(id)) ?? []
      setMobileApps(list)
      setMobileScanned(true)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setSyncing(false)
    }
  }

  const active = activePkg ? apps.find((a) => a.package === activePkg) ?? null : null
  const proxyById = useMemo(() => new Map(proxies.map((p) => [p.id, p])), [proxies])

  const visible = useMemo(() => {
    const q = fold(query.trim())
    return apps.filter((a) => {
      if (oriFilter !== 'all' && a.orientation !== oriFilter) return false
      if (!q) return true
      const p = a.proxyId ? proxyById.get(a.proxyId) : undefined
      return (
        fold(a.label).includes(q) ||
        fold(a.package).includes(q) ||
        fold(a.notes || '').includes(q) ||
        fold(p?.label || '').includes(q) ||
        fold(p?.country || '').includes(q)
      )
    })
  }, [apps, query, oriFilter, proxyById])

  const pickAndInspect = async () => {
    setError('')
    try {
      const file = await Api.PickAPKFile()
      if (!file) return
      const meta = await Api.InspectAPK(file)
      openInspect(meta)
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  const confirmAdd = async () => {
    if (!inspect || !inspect.abiCompatible) return
    setAdding(true)
    try {
      const rec = await Api.AddAPK(service.APKInput.createFrom({
        filePath: inspect.filePath,
        label: targetLabel.trim(),
        deviceId: '',
        proxyId: targetProxy,
        notes: targetNotes.trim(),
        orientation: targetOri,
        identityTemplate: targetModel,
        createShortcut: makeShortcut,
      }))
      setInspect(null)
      toast.success(t.apk.installed(rec.label))
      reload()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setAdding(false)
    }
  }

  const renderProxy = (proxyId: string) => {
    if (!proxyId) {
      return (
        <span className="proxy-pill">
          <Icon name="direct" className="icon xs" />
          <span className="faint">{t.common.direct}</span>
        </span>
      )
    }
    const p = proxyById.get(proxyId)
    if (!p) {
      return (
        <span className="proxy-pill">
          <Icon name="proxy" className="icon xs" />
          <span className="faint">…</span>
        </span>
      )
    }
    return (
      <span className="proxy-pill" title={`${p.label} (${p.scheme}://${p.host}:${p.port})`}>
        <Icon name="proxy" className="icon xs" />
        <span>
          {p.label}
          {p.country && <span className="faint"> · {p.country}</span>}
        </span>
      </span>
    )
  }

  const launch = async (app: store.MobileAPK) => {
    setBusyPkg(app.package)
    try {
      await Api.LaunchAPK(app.package, '')
      reload()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusyPkg('')
    }
  }

  const changeOrientation = async (pkg: string, orientation: string) => {
    try {
      await Api.SetAPKOrientation(pkg, orientation)
      reload()
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  const createShortcut = async (pkg: string) => {
    setBusyPkg(pkg)
    try {
      const path = await Api.CreateAPKShortcut(pkg)
      toast.success(t.apk.shortcutSaved(path))
      reload()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusyPkg('')
    }
  }

  const remove = async (app: store.MobileAPK) => {
    const ok = await confirm({
      title: t.apk.removeTitle(app.label),
      body: t.apk.removeBody,
      confirm: t.apk.remove,
      danger: true,
    })
    if (!ok) return
    setBusyPkg(app.package)
    try {
      await Api.DeleteAPK(app.package, true)
      if (activePkg === app.package) setActivePkg(null)
      toast.success(t.apk.removed(app.label))
      reload()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusyPkg('')
    }
  }

  return (
    <>
      <PageHead title={t.apk.title} badge={t.apk.badge} sub={t.apk.total(apps.length)}>
        <button type="button" className="btn primary nodrag" disabled={!canUse} onClick={openNewDialog}>
          <Icon name="plus" />{t.apk.addApk}
        </button>
      </PageHead>

      <div className="pagebody">
        {storeWatching && !newOpen && (
          <div className="banner" role="status">
            <Icon name="download" />
            <span className="grow">{t.apk.storeWatching}</span>
            <button type="button" className="btn sm" disabled={syncing} onClick={() => void syncFromStore(true)}>
              <Icon name="refresh" className="icon xs" />{syncing ? t.apk.syncing : t.apk.syncNow}
            </button>
            <button type="button" className="iconbtn sm" onClick={() => setStoreWatching(false)} aria-label={t.common.close}>
              <Icon name="x" />
            </button>
          </div>
        )}
        {avail && !avail.pro && (
          <section className="card pad">
            <h2>{t.apk.proTitle}</h2>
            <p className="desc">{apps.length ? t.apk.proLapsed : t.apk.proText}</p>
            <p className="desc">{t.apk.proHonest}</p>
            <div className="row">
              <button type="button" className="btn primary" onClick={onOpenPro}>
                <Icon name="spark" />{t.apk.getPro}
              </button>
            </div>
          </section>
        )}

        {avail?.pro && !avail.ready && (
          <section className="card pad">
            <h2>{t.apk.needRuntimeTitle}</h2>
            <p className="desc">{t.apk.needRuntimeText}</p>
            <div className="row">
              <button type="button" className="btn primary" onClick={onOpenMobile}>
                <Icon name="mobile" />{t.apk.goMobile}
              </button>
            </div>
          </section>
        )}

        {error && (
          <div className="banner danger" role="alert">
            <Icon name="alert" />
            <span className="grow">{error}</span>
            <button type="button" className="iconbtn sm" onClick={() => setError('')} aria-label={t.common.close}>
              <Icon name="x" />
            </button>
          </div>
        )}

        <div className={`profiles-split${active ? ' open' : ''}`}>
          <section className="card fill" style={{ minHeight: 240 }}>
            {apps.length > 0 && (
              <div className="toolbar">
                <div className="searchfield">
                  <Icon name="search" />
                  <input
                    id="apk-search"
                    className="input"
                    placeholder={t.apk.search}
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    aria-label={t.apk.search}
                  />
                </div>
                {query && (
                  <button type="button" className="btn sm ghost" onClick={() => setQuery('')}>
                    <Icon name="x" className="icon xs" />
                  </button>
                )}
                <span className="spacer" />
                <div className="seg" role="group" aria-label={t.apk.colOrientation}>
                  {(['all', 'portrait', 'landscape'] as OriFilter[]).map((k) => (
                    <button key={k} type="button" aria-pressed={oriFilter === k} onClick={() => setOriFilter(k)}>
                      {k === 'all' ? t.apk.allTab : k === 'portrait' ? t.apk.portraitTab : t.apk.landscapeTab}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <div className="tablewrap">
              {apps.length === 0 ? (
                <EmptyState icon="apk" title={t.apk.emptyTitle} text={canUse ? t.apk.emptyText : undefined}>
                  {canUse && (
                    <button type="button" className="btn primary" onClick={openNewDialog}>
                      <Icon name="plus" />{t.apk.addApk}
                    </button>
                  )}
                </EmptyState>
              ) : visible.length === 0 ? (
                <EmptyState icon="search" title={t.profiles.noMatchTitle} />
              ) : (
                <table className="table apk-table" aria-label={t.apk.title}>
                  <thead>
                    <tr>
                      <th scope="col" className="col-no">{t.profiles.colNo}</th>
                      <th scope="col">{t.apk.colApp}</th>
                      <th scope="col" className="apk-col-proxy">{t.apk.colProxy}</th>
                      <th scope="col" className="apk-col-ori">{t.apk.colOrientation}</th>
                      <th scope="col" className="apk-col-abi">{t.apk.colAbi}</th>
                      <th scope="col" className="apk-col-shortcut">{t.apk.colShortcut}</th>
                      <th scope="col" className="col-actions" />
                    </tr>
                  </thead>
                  <tbody>
                    {visible.map((app, i) => {
                      const working = busyPkg === app.package
                      const isActive = activePkg === app.package
                      return (
                        <tr
                          key={app.package}
                          className={isActive ? 'active' : undefined}
                          aria-current={isActive ? 'true' : undefined}
                          onClick={(e) => {
                            if (e.detail <= 1 && !(e.target as HTMLElement).closest('button, select, input, a')) {
                              setActivePkg((cur) => (cur === app.package ? null : app.package))
                            }
                          }}
                          onDoubleClick={(e) => {
                            if (!canUse || working) return
                            if ((e.target as HTMLElement).closest('button, select, input, a')) return
                            void launch(app)
                          }}
                        >
                          <td className="col-no">{i + 1}</td>
                          <td>
                            <div className="apk-id">
                              <div className="apk-icon">
                                {app.iconData ? (
                                  <img src={app.iconData} alt="" />
                                ) : (
                                  <span>{app.label.slice(0, 2).toUpperCase()}</span>
                                )}
                              </div>
                              <div className="min0">
                                <div className="apk-name">{app.label}</div>
                                <div className="apk-meta">
                                  {app.package} · v{app.versionName}
                                  {app.notes ? ` · ${app.notes}` : ''}
                                </div>
                              </div>
                            </div>
                          </td>
                          <td className="apk-col-proxy">{renderProxy(app.proxyId)}</td>
                          <td className="apk-col-ori">
                            <select
                              className="input apk-select"
                              aria-label={`${app.label} ${t.apk.colOrientation}`}
                              disabled={!canUse || working}
                              value={app.orientation}
                              onChange={(e) => changeOrientation(app.package, e.target.value)}
                            >
                              <option value="auto">{t.apk.oriAuto}</option>
                              <option value="portrait">{t.apk.oriPortrait}</option>
                              <option value="landscape">{t.apk.oriLandscape}</option>
                            </select>
                          </td>
                          <td className="apk-col-abi">
                            <div className="apk-meta" title={app.abis?.join(', ') || t.apk.universalAbi}>
                              {app.abis && app.abis.length > 0 ? app.abis.join(', ') : t.apk.universalAbi}
                            </div>
                          </td>
                          <td className="apk-col-shortcut">
                            {app.shortcutPath ? (
                              <span className="pill ok">{t.apk.shortcutCreated}</span>
                            ) : (
                              <button
                                type="button"
                                className="btn sm"
                                disabled={!canUse || working}
                                onClick={() => createShortcut(app.package)}
                              >
                                {t.apk.createShortcut}
                              </button>
                            )}
                          </td>
                          <td className="col-actions">
                            <div className="row">
                              <button
                                type="button"
                                className={`iconbtn sm${isActive ? ' on' : ''}`}
                                aria-pressed={isActive}
                                disabled={working}
                                aria-label={`${t.apk.settings} ${app.label}`}
                                title={t.apk.settings}
                                onClick={() => setActivePkg((cur) => (cur === app.package ? null : app.package))}
                              >
                                <Icon name="settings" className="icon sm" />
                              </button>
                              <button
                                type="button"
                                className="btn sm open"
                                disabled={!canUse || working}
                                onClick={() => launch(app)}
                              >
                                <Icon name="play" className="icon xs" />
                                {working ? t.apk.opening : t.apk.open}
                              </button>
                              <button
                                type="button"
                                className="iconbtn sm danger"
                                disabled={working}
                                aria-label={`${t.apk.remove} ${app.label}`}
                                title={t.apk.remove}
                                onClick={() => remove(app)}
                              >
                                <Icon name="trash" className="icon sm" />
                              </button>
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
          </section>
          {active && (
            <ApkDrawer
              key={active.package}
              app={active}
              proxies={proxies}
              canUse={canUse}
              working={busyPkg === active.package}
              onClose={() => setActivePkg(null)}
              onOpen={launch}
              onCreateShortcut={createShortcut}
              onRemove={remove}
              onChanged={reload}
            />
          )}
        </div>
      </div>

      {newOpen && (
        <Dialog
          title={t.apk.newTitle}
          icon="apk"
          onClose={() => !openingStore && !downloadingUrl && setNewOpen(false)}
          footer={newTab === 'store' ? (
            <>
              <button type="button" className="btn" disabled={openingStore} onClick={() => setNewOpen(false)}>
                {t.common.close}
              </button>
              <button type="button" className="btn primary" disabled={openingStore} onClick={() => void openStore()}>
                <Icon name="download" className="icon xs" />
                {openingStore ? t.apk.openingStore : t.apk.openStoreBtn}
              </button>
            </>
          ) : (
            <button type="button" className="btn" disabled={downloadingUrl} onClick={() => setNewOpen(false)}>
              {t.common.close}
            </button>
          )}
        >
          <div className="seg apk-new-tabs" role="tablist">
            <button type="button" role="tab" aria-selected={newTab === 'store'} aria-pressed={newTab === 'store'} onClick={() => setNewTab('store')}>
              {t.apk.tabStore}
            </button>
            <button type="button" role="tab" aria-selected={newTab === 'file'} aria-pressed={newTab === 'file'} onClick={() => setNewTab('file')}>
              {t.apk.tabFileUrl}
            </button>
          </div>

          {newTab === 'store' ? (
            <div className="apk-inspect-fields">
              <label className="field">
                <span>{t.apk.storeSearchLabel}</span>
                <input
                  className="input"
                  value={storeQuery}
                  placeholder={t.apk.storeSearchPlaceholder}
                  onChange={(e) => setStoreQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && !openingStore) void openStore() }}
                />
              </label>

              <div className="field">
                <span>{t.apk.storePopularLabel}</span>
                <div className="apk-chips">
                  {POPULAR_APPS.map(([label, pkg]) => {
                    const selected = storeQuery.trim() === pkg
                    return (
                      <button
                        key={pkg}
                        type="button"
                        className="apk-chip"
                        aria-pressed={selected}
                        onClick={() => setStoreQuery(selected ? '' : pkg)}
                      >
                        {label}
                      </button>
                    )
                  })}
                </div>
              </div>

              <label className="field">
                <span>{t.apk.proxyField}</span>
                <ProxySelect value={storeProxy} onChange={setStoreProxy} proxies={proxies} />
              </label>
              {storeProxy === '' && <RealIpWarning />}

              <p className="desc" style={{ margin: 0 }}>{t.apk.storeHint}</p>

              {storeWatching && (
                <div className="banner" role="status">
                  <Icon name="refresh" />
                  <span className="grow">{t.apk.storeWatching}</span>
                  <button type="button" className="btn sm" disabled={syncing} onClick={() => void syncFromStore(true)}>
                    {syncing ? t.apk.syncing : t.apk.syncNow}
                  </button>
                </div>
              )}

              {detected.length > 0 && (
                <div className="apk-found-list">
                  <div className="apk-source-head">{t.apk.detectedApps}</div>
                  {detected.map((item) => (
                    <div key={item.package} className="apk-found-row">
                      <span className="apk-found-pkg">{item.package}</span>
                      <button
                        type="button"
                        className="btn sm primary"
                        disabled={importingPkg === item.package}
                        onClick={() => void importOne(item, storeProxy)}
                      >
                        {importingPkg === item.package ? t.apk.importing : t.apk.importBtn}
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div className="apk-inspect-fields">
              <div className="apk-source-block">
                <div className="apk-source-head">{t.apk.fromComputerLabel}</div>
                <p className="desc" style={{ margin: 0 }}>{t.apk.fromComputerHint}</p>
                <div className="row">
                  <button type="button" className="btn primary" onClick={pickAndInspect}>
                    <Icon name="plus" className="icon xs" />{t.apk.chooseFileBtn}
                  </button>
                </div>
              </div>

              <div className="apk-source-block">
                <div className="apk-source-head">{t.apk.fromUrlLabel}</div>
                <input
                  className="input"
                  value={urlInput}
                  placeholder={t.apk.fromUrlPlaceholder}
                  onChange={(e) => setUrlInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && !downloadingUrl) void downloadFromUrl() }}
                />
                <ProxySelect value={urlProxy} onChange={setUrlProxy} proxies={proxies} />
                <div className="row">
                  <button
                    type="button"
                    className="btn"
                    disabled={!urlInput.trim() || downloadingUrl}
                    onClick={() => void downloadFromUrl()}
                  >
                    <Icon name="download" className="icon xs" />
                    {downloadingUrl ? t.apk.downloadingUrl : t.apk.downloadUrlBtn}
                  </button>
                </div>
              </div>

              {runningMobile.length > 0 && (
                <div className="apk-source-block">
                  <div className="apk-source-head">{t.apk.fromMobileLabel}</div>
                  <div className="row">
                    <select
                      className="input grow"
                      value={mobileSourceId || runningMobile[0].id}
                      onChange={(e) => { setMobileSourceId(e.target.value); setMobileScanned(false); setMobileApps([]) }}
                    >
                      {runningMobile.map((m) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                    <button type="button" className="btn" disabled={syncing} onClick={() => void scanMobileDevice()}>
                      {syncing ? t.apk.syncing : t.apk.scanMobileBtn}
                    </button>
                  </div>
                  {mobileScanned && mobileApps.length === 0 && (
                    <p className="desc" style={{ margin: 0 }}>{t.apk.noMobileApps}</p>
                  )}
                  {mobileApps.length > 0 && (
                    <div className="apk-found-list">
                      {mobileApps.map((item) => (
                        <div key={item.package} className="apk-found-row">
                          <span className="apk-found-pkg">{item.package}</span>
                          <button
                            type="button"
                            className="btn sm primary"
                            disabled={importingPkg === item.package}
                            onClick={() => void importOne(item, '')}
                          >
                            {importingPkg === item.package ? t.apk.importing : t.apk.importBtn}
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </Dialog>
      )}

      {inspect && (
        <Dialog
          title={t.apk.dialogTitle}
          icon="apk"
          onClose={() => !adding && setInspect(null)}
          footer={(
            <>
              <button type="button" className="btn" disabled={adding} onClick={() => setInspect(null)}>
                {t.common.cancel}
              </button>
              <button
                type="button"
                className="btn primary"
                disabled={!inspect.abiCompatible || adding}
                onClick={confirmAdd}
              >
                {adding ? t.apk.installing : t.apk.installBtn}
              </button>
            </>
          )}
        >
          <div className="apk-inspect-head">
            <div className="apk-icon lg">
              {inspect.iconData ? (
                <img src={inspect.iconData} alt="" />
              ) : (
                <span>{inspect.label.slice(0, 2).toUpperCase()}</span>
              )}
            </div>
            <div className="min0">
              <div className="apk-name">{inspect.label}</div>
              <div className="apk-meta">
                {inspect.package} · v{inspect.versionName} · {formatBytes(inspect.sizeBytes)}
              </div>
              <div style={{ marginTop: 6 }}>
                {inspect.abiCompatible ? (
                  <span className="pill ok">
                    {inspect.abis?.length ? inspect.abis.join(', ') : t.apk.universalAbi}
                  </span>
                ) : (
                  <span className="pill danger">{inspect.abiWarning}</span>
                )}
              </div>
            </div>
          </div>

          <div className="apk-inspect-fields">
            <label className="field">
              <span>{t.apk.nameField}</span>
              <input
                className="input"
                value={targetLabel}
                placeholder={t.apk.namePlaceholder}
                onChange={(e) => setTargetLabel(e.target.value)}
              />
            </label>

            <label className="field">
              <span>{t.apk.proxyField}</span>
              <ProxySelect value={targetProxy} onChange={setTargetProxy} proxies={proxies} />
            </label>
            {targetProxy === '' && <RealIpWarning />}

            <label className="field">
              <span>{t.mobile.deviceModel}</span>
              <select className="input" value={targetModel} onChange={(e) => setTargetModel(e.target.value)}>
                <option value="">{t.mobile.modelAuto}</option>
                {MODELS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </label>

            <label className="field">
              <span>{t.apk.orientationField}</span>
              <select className="input" value={targetOri} onChange={(e) => setTargetOri(e.target.value)}>
                <option value="auto">{t.apk.oriAuto}</option>
                <option value="portrait">{t.apk.oriPortrait}</option>
                <option value="landscape">{t.apk.oriLandscape}</option>
              </select>
            </label>

            <label className="field">
              <span>{t.apk.notesField}</span>
              <input
                className="input"
                value={targetNotes}
                placeholder={t.apk.notesPlaceholder}
                onChange={(e) => setTargetNotes(e.target.value)}
              />
            </label>

            <label className="row checkrow">
              <input type="checkbox" checked={makeShortcut} onChange={(e) => setMakeShortcut(e.target.checked)} />
              <span>{t.apk.shortcutField}</span>
            </label>
          </div>
        </Dialog>
      )}
    </>
  )
}
