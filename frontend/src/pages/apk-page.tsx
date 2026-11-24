// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Api, onDataChanged, service, store } from '../api'
import { t } from '../i18n'
import { errorText, fold } from '../lib/format'
import { countryFlag, countryName } from '../lib/flags'
import type { useMobile } from '../hooks/use-mobile'
import { ApkDrawer } from '../components/apk-drawer'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { NewApkDialog, type PendingStoreConfig } from '../components/new-apk-dialog'
import { PageHead } from '../components/page-head'
import { useConfirm } from '../components/confirm'
import { useToast } from '../components/toast'

type Props = {
  mobile: ReturnType<typeof useMobile>
  onOpenPro: () => void
  onOpenMobile: () => void
  onChanged?: () => void
}

type OriFilter = 'all' | 'portrait' | 'landscape'

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

  // Unified New APK modal + Aurora Store auto-sync watcher.
  const [newOpen, setNewOpen] = useState(false)
  const [storeWatching, setStoreWatching] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [detected, setDetected] = useState<service.UncatalogedAPK[]>([])
  const storeBaselineRef = useRef<Set<string>>(new Set())
  const importingRef = useRef<Set<string>>(new Set())
  const pendingStoreCfgRef = useRef<PendingStoreConfig | null>(null)

  const toast = useToast()
  const confirm = useConfirm()

  const canUse = !!avail?.pro && !!avail?.ready
  const runningMobile = useMemo(
    () => (mobile.rows ?? []).filter((r) => r.state === 'running'),
    [mobile.rows],
  )
  const existingPackages = useMemo(
    () => new Set(apps.map((a) => a.package)),
    [apps],
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

  const syncFromStore = useCallback(async (autoImportNew: boolean) => {
    if (!canUse) return
    setSyncing(true)
    try {
      const list = (await Api.ListUncatalogedAPKs('')) ?? []
      setDetected(list)
      if (!autoImportNew) return
      for (const item of list) {
        if (storeBaselineRef.current.has(item.package) || importingRef.current.has(item.package)) continue
        const cfg = pendingStoreCfgRef.current
        // When watching for a specific app from Aurora Store, ignore unrelated pre-existing packages.
        if (cfg?.package && cfg.package !== item.package) {
          storeBaselineRef.current.add(item.package)
          continue
        }
        importingRef.current.add(item.package)
        const matchesPkg = cfg && (!cfg.package || cfg.package === item.package)
        try {
          const rec = await Api.ImportInstalledAPK(service.APKImportInput.createFrom({
            package: item.package,
            deviceId: item.deviceId,
            label: matchesPkg ? cfg.label : '',
            proxyId: cfg?.proxyId ?? '',
            notes: matchesPkg ? cfg.notes : '',
            orientation: matchesPkg ? cfg.orientation : 'auto',
            identityTemplate: matchesPkg ? cfg.identityTemplate : '',
            createShortcut: cfg ? cfg.createShortcut : true,
          }))
          storeBaselineRef.current.add(item.package)
          setDetected((cur) => cur.filter((x) => x.package !== item.package))
          toast.success(t.apk.autoImported(rec.label))
          reload()
        } catch (e) {
          toast.error(errorText(e))
        } finally {
          importingRef.current.delete(item.package)
        }
      }
    } catch {
      // Host may still be starting or paused.
    } finally {
      setSyncing(false)
    }
  }, [canUse, reload, toast])

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
    setNewOpen(true)
    void syncFromStore(false)
  }

  const handleStoreOpened = async (cfg: PendingStoreConfig) => {
    pendingStoreCfgRef.current = cfg
    // Snapshot baseline AFTER OpenAPKStore has booted/resumed the Android host
    // so pre-existing packages never get mistaken for newly installed apps.
    let baseline = detected
    try {
      const live = (await Api.ListUncatalogedAPKs('')) ?? []
      baseline = live
      setDetected(live)
    } catch {
      // Keep current detected list if probe fails.
    }
    storeBaselineRef.current = new Set(baseline.map((x) => x.package))
    setStoreWatching(true)
    setNewOpen(false)
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
    const flag = countryFlag(p.country)
    return (
      <span className="proxy-pill" title={`${p.label}${p.country ? ` · ${countryName(p.country)}` : ''} (${p.scheme}://${p.host}:${p.port})`}>
        {flag ? <span className="cc-flag" aria-hidden="true">{flag}</span> : <Icon name="proxy" className="icon xs" />}
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
      storeBaselineRef.current.add(app.package)
      setDetected((cur) => cur.filter((x) => x.package !== app.package))
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
        <NewApkDialog
          proxies={proxies}
          existingPackages={existingPackages}
          runningMobile={runningMobile}
          detected={detected}
          onClose={() => setNewOpen(false)}
          onAdded={() => {
            setNewOpen(false)
            reload()
          }}
          onStoreOpened={handleStoreOpened}
        />
      )}
    </>
  )
}
