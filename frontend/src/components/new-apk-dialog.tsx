// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { countryFlag } from '../lib/flags'
import { MODELS } from '../lib/mobile-presets'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { ProxySelect } from './proxy-select'
import { useToast } from './toast'

export type PendingStoreConfig = {
  package: string
  label: string
  proxyId: string
  orientation: string
  identityTemplate: string
  notes: string
  createShortcut: boolean
}

type MobileRow = { id: string; name: string; state: string }

type Props = {
  proxies: service.ProxyRow[]
  existingPackages: Set<string>
  runningMobile: MobileRow[]
  detected: service.UncatalogedAPK[]
  onClose: () => void
  onAdded: (rec: store.MobileAPK) => void
  onStoreOpened: (cfg: PendingStoreConfig) => void
}

type SourceMode = 'search' | 'file'
type ExtraPanel = 'none' | 'url' | 'mobile'

type SelectedTarget =
  | { kind: 'store'; app: service.APKStoreApp }
  | { kind: 'file'; inspect: service.APKInspect }
  | { kind: 'import'; item: service.UncatalogedAPK; label: string }

const PKG_RE = /^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$/

function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 MB'
  const mb = n / (1024 * 1024)
  return mb >= 1000 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(1)} MB`
}

function defaultProxyId(proxies: service.ProxyRow[]): string {
  const free = proxies.find((p) => p.assignable && (!p.usedBy || p.usedBy.length === 0))
  return free ? free.id : ''
}

function StoreAppIcon({ title, iconUrl, size = 'md' }: { title: string; iconUrl?: string; size?: 'md' | 'lg' }) {
  const [failed, setFailed] = useState(false)
  const cls = size === 'lg' ? 'apk-icon lg' : 'apk-icon sm'
  if (iconUrl && !failed) {
    return (
      <div className={cls}>
        <img src={iconUrl} alt="" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
      </div>
    )
  }
  return (
    <div className={cls}>
      <span>{(title || 'AP').slice(0, 2).toUpperCase()}</span>
    </div>
  )
}

/** Unified 2-step New APK modal: pick from live store search, local file, URL or Mobile,
 *  then confirm proxy and optional advanced settings in one consistent screen. */
export function NewApkDialog({
  proxies,
  existingPackages,
  runningMobile,
  detected,
  onClose,
  onAdded,
  onStoreOpened,
}: Props) {
  const toast = useToast()

  // Step 1: source selection
  const [mode, setMode] = useState<SourceMode>('search')
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<service.APKStoreApp[]>([])
  const [searching, setSearching] = useState(false)
  const [highlight, setHighlight] = useState(0)
  const [extraPanel, setExtraPanel] = useState<ExtraPanel>('none')
  const [urlInput, setUrlInput] = useState('')
  const [downloadingUrl, setDownloadingUrl] = useState(false)
  const [pickingFile, setPickingFile] = useState(false)
  const [mobileSourceId, setMobileSourceId] = useState(() => runningMobile[0]?.id ?? '')
  const [mobileApps, setMobileApps] = useState<service.UncatalogedAPK[]>([])
  const [mobileScanned, setMobileScanned] = useState(false)
  const [scanningMobile, setScanningMobile] = useState(false)

  // Step 2: unified confirmation & configuration
  const [target, setTarget] = useState<SelectedTarget | null>(null)
  const [label, setLabel] = useState('')
  const [proxyId, setProxyId] = useState(() => defaultProxyId(proxies))
  const [orientation, setOrientation] = useState('auto')
  const [model, setModel] = useState('')
  const [notes, setNotes] = useState('')
  const [makeShortcut, setMakeShortcut] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  const searchInputRef = useRef<HTMLInputElement>(null)
  const searchSeqRef = useRef(0)

  const trimmedQuery = query.trim()
  const isHttpsUrl = /^https:\/\/\S+/i.test(trimmedQuery)
  const busy = submitting || downloadingUrl || pickingFile

  // Live debounced search via Go backend (empty query returns popular apps instantly).
  useEffect(() => {
    if (mode !== 'search' || target !== null) return
    if (isHttpsUrl) {
      setResults([])
      setSearching(false)
      return
    }
    const seq = ++searchSeqRef.current
    const delay = trimmedQuery ? 240 : 0
    if (trimmedQuery) setSearching(true)
    const timer = window.setTimeout(() => {
      Api.SearchAPKStore(trimmedQuery, proxyId)
        .then((list) => {
          if (searchSeqRef.current !== seq) return
          setResults(list ?? [])
          setHighlight(0)
        })
        .catch(() => {
          if (searchSeqRef.current !== seq) return
          setResults([])
        })
        .finally(() => {
          if (searchSeqRef.current === seq) setSearching(false)
        })
    }, delay)
    return () => window.clearTimeout(timer)
  }, [mode, target, trimmedQuery, isHttpsUrl, proxyId])

  const enterConfigure = (next: SelectedTarget) => {
    setTarget(next)
    setModel('')
    setNotes('')
    setMakeShortcut(true)
    if (next.kind === 'file') {
      setLabel(next.inspect.label || '')
      setOrientation(next.inspect.orientation || 'auto')
    } else if (next.kind === 'store') {
      setLabel(next.app.title || '')
      setOrientation('auto')
    } else {
      setLabel(next.label || '')
      setOrientation('auto')
    }
  }

  const pickLocalFile = async () => {
    setPickingFile(true)
    try {
      const file = await Api.PickAPKFile()
      if (!file) return
      const meta = await Api.InspectAPK(file)
      enterConfigure({ kind: 'file', inspect: meta })
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setPickingFile(false)
    }
  }

  const downloadFromUrl = async (raw: string) => {
    const clean = raw.trim()
    if (!clean) return
    setDownloadingUrl(true)
    try {
      const meta = await Api.DownloadAPKFromURL(clean, proxyId)
      setUrlInput('')
      enterConfigure({ kind: 'file', inspect: meta })
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setDownloadingUrl(false)
    }
  }

  const scanMobile = async () => {
    const id = mobileSourceId || runningMobile[0]?.id || ''
    if (!id) return
    setScanningMobile(true)
    try {
      const list = (await Api.ListUncatalogedAPKs(id)) ?? []
      setMobileApps(list)
      setMobileScanned(true)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setScanningMobile(false)
    }
  }

  const submitConfigure = async () => {
    if (!target || submitting) return
    if (target.kind === 'file' && !target.inspect.abiCompatible) return
    setSubmitting(true)
    try {
      if (target.kind === 'file') {
        const rec = await Api.AddAPK(service.APKInput.createFrom({
          filePath: target.inspect.filePath,
          label: label.trim(),
          deviceId: '',
          proxyId,
          notes: notes.trim(),
          orientation,
          identityTemplate: model,
          createShortcut: makeShortcut,
        }))
        toast.success(t.apk.installed(rec.label))
        onAdded(rec)
        return
      }
      if (target.kind === 'import') {
        const rec = await Api.ImportInstalledAPK(service.APKImportInput.createFrom({
          package: target.item.package,
          deviceId: target.item.deviceId,
          label: label.trim(),
          proxyId,
          notes: notes.trim(),
          orientation,
          identityTemplate: model,
          createShortcut: makeShortcut,
        }))
        toast.success(t.apk.installed(rec.label))
        onAdded(rec)
        return
      }
      await Api.OpenAPKStore(service.APKStoreRequest.createFrom({
        query: target.app.package || target.app.title,
        proxyId,
        deviceId: '',
      }))
      onStoreOpened({
        package: target.app.package,
        label: label.trim(),
        proxyId,
        orientation,
        identityTemplate: model,
        notes: notes.trim(),
        createShortcut: makeShortcut,
      })
    } catch (e) {
      toast.error(errorText(e))
      setSubmitting(false)
    }
  }

  const visibleResults = useMemo(() => results.slice(0, 6), [results])

  const selectFallbackQuery = () => {
    if (!trimmedQuery) return
    if (isHttpsUrl) {
      void downloadFromUrl(trimmedQuery)
      return
    }
    const isPkg = PKG_RE.test(trimmedQuery)
    enterConfigure({
      kind: 'store',
      app: service.APKStoreApp.createFrom({
        package: isPkg ? trimmedQuery : '',
        title: trimmedQuery,
        developer: isPkg ? t.apk.directPackage : t.apk.storeSearchFallbackDev,
        iconUrl: '',
      }),
    })
  }

  const onSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (visibleResults.length > 0) setHighlight((h) => (h + 1) % visibleResults.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (visibleResults.length > 0) setHighlight((h) => (h - 1 + visibleResults.length) % visibleResults.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (isHttpsUrl) {
        void downloadFromUrl(trimmedQuery)
      } else if (visibleResults[highlight]) {
        enterConfigure({ kind: 'store', app: visibleResults[highlight] })
      } else if (trimmedQuery) {
        selectFallbackQuery()
      }
    }
  }

  const selectedProxy = useMemo(
    () => (proxyId ? proxies.find((p) => p.id === proxyId) ?? null : null),
    [proxyId, proxies],
  )
  const regionBadge = selectedProxy?.country
    ? [countryFlag(selectedProxy.country), selectedProxy.country].filter(Boolean).join(' ')
    : ''
  const proxySummary = selectedProxy
    ? [countryFlag(selectedProxy.country), selectedProxy.label, selectedProxy.country].filter(Boolean).join(' ')
    : t.common.direct

  const oriLabel = orientation === 'portrait'
    ? t.apk.oriPortrait
    : orientation === 'landscape'
      ? t.apk.oriLandscape
      : t.apk.oriAuto
  const modelLabel = MODELS.find(([v]) => v === model)?.[1] ?? t.mobile.modelAuto
  const abiBlocked = target?.kind === 'file' && !target.inspect.abiCompatible

  const primaryBtnLabel = (() => {
    if (!target) return ''
    if (target.kind === 'file') return submitting ? t.apk.installing : t.apk.installBtn
    if (target.kind === 'import') return submitting ? t.apk.importing : t.apk.importBtn
    return submitting ? t.apk.openingStore : t.apk.openStoreBtn
  })()

  return (
    <Dialog
      title={target ? t.apk.confirmTitle : t.apk.newTitle}
      icon="apk"
      className="apk-dialog"
      onClose={() => !busy && onClose()}
      onSubmit={target ? () => void submitConfigure() : undefined}
      footer={target ? (
        <>
          <button
            type="button"
            className="btn ghost"
            disabled={submitting}
            onClick={() => setTarget(null)}
          >
            {t.apk.backBtn}
          </button>
          <button
            type="submit"
            className="btn primary"
            disabled={abiBlocked || submitting}
          >
            <Icon name={target.kind === 'store' ? 'download' : 'plus'} className="icon xs" />
            {primaryBtnLabel}
          </button>
        </>
      ) : (
        <button type="button" className="btn ghost" disabled={busy} onClick={onClose}>
          {t.common.close}
        </button>
      )}
    >
      {!target ? (
        <div className="apk-step">
          <label className="field">
            <span className="label">{t.apk.proxyField}</span>
            <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} disabled={busy} />
          </label>
          {proxyId === '' && (
            <p className="apk-ip-note">
              <Icon name="alert" className="icon xs" />
              <span>{t.apk.realIpNote}</span>
            </p>
          )}

          <div className="seg apk-mode-tabs" role="tablist" aria-label={t.apk.newTitle}>
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'search'}
              aria-pressed={mode === 'search'}
              onClick={() => setMode('search')}
            >
              {t.apk.tabSearch}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === 'file'}
              aria-pressed={mode === 'file'}
              onClick={() => setMode('file')}
            >
              {t.apk.tabLocalFile}
            </button>
          </div>

          {mode === 'search' ? (
            <div className="apk-search-step">
              <div className="apk-combobox">
                <Icon name="search" className="icon sm apk-combobox-ico" />
                <input
                  ref={searchInputRef}
                  className="input apk-combobox-input"
                  autoFocus
                  role="combobox"
                  aria-expanded="true"
                  aria-controls="apk-store-listbox"
                  aria-label={t.apk.storeSearchLabel}
                  value={query}
                  placeholder={t.apk.storeSearchPlaceholder}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={onSearchKeyDown}
                />
                {searching && <Icon name="refresh" className="icon xs spin apk-combobox-spin" />}
                {query && !searching && (
                  <button
                    type="button"
                    className="iconbtn sm apk-combobox-clear"
                    aria-label={t.common.clear}
                    onClick={() => { setQuery(''); searchInputRef.current?.focus() }}
                  >
                    <Icon name="x" className="icon xs" />
                  </button>
                )}
              </div>

              <div id="apk-store-listbox" className="apk-results" role="listbox" aria-label={t.apk.storeResultsLabel}>
                {isHttpsUrl ? (
                  <button
                    type="button"
                    role="option"
                    aria-selected="true"
                    className="apk-result-row active"
                    disabled={downloadingUrl}
                    onClick={() => void downloadFromUrl(trimmedQuery)}
                  >
                    <div className="apk-icon sm"><Icon name="download" className="icon sm" /></div>
                    <div className="min0 grow">
                      <div className="apk-result-title">
                        {downloadingUrl ? t.apk.downloadingUrl : t.apk.smartUrlAction}
                      </div>
                      <div className="apk-result-sub">{trimmedQuery}</div>
                    </div>
                  </button>
                ) : (
                  <>
                    <div className="apk-results-head">
                      <span>
                        {trimmedQuery
                          ? `${t.apk.storeResultsLabel}${regionBadge ? ` · ${regionBadge}` : ''}`
                          : t.apk.storePopularLabel}
                      </span>
                      {searching && <span className="apk-results-status">{t.apk.searchingStore}</span>}
                    </div>

                    {visibleResults.map((app, idx) => {
                      const already = existingPackages.has(app.package)
                      return (
                        <button
                          key={app.package}
                          type="button"
                          role="option"
                          aria-selected={highlight === idx}
                          className={`apk-result-row${highlight === idx ? ' active' : ''}`}
                          onMouseEnter={() => setHighlight(idx)}
                          onClick={() => enterConfigure({ kind: 'store', app })}
                        >
                          <StoreAppIcon title={app.title} iconUrl={app.iconUrl} />
                          <div className="min0 grow">
                            <div className="apk-result-title">{app.title}</div>
                            <div className="apk-result-sub">
                              {app.developer ? `${app.developer} · ${app.package}` : app.package}
                            </div>
                          </div>
                          {already ? (
                            <span className="apk-result-tag">{t.apk.alreadyInstalled}</span>
                          ) : app.rating ? (
                            <span className="apk-result-meta">{app.rating} ★</span>
                          ) : app.category ? (
                            <span className="apk-result-meta">{app.category}</span>
                          ) : null}
                        </button>
                      )
                    })}

                    {!searching && visibleResults.length === 0 && trimmedQuery && (
                      <button
                        type="button"
                        role="option"
                        aria-selected="true"
                        className="apk-result-row active"
                        onClick={selectFallbackQuery}
                      >
                        <div className="apk-icon sm"><Icon name="search" className="icon sm" /></div>
                        <div className="min0 grow">
                          <div className="apk-result-title">{t.apk.searchOnStore(trimmedQuery)}</div>
                          <div className="apk-result-sub">{t.apk.storeSearchFallbackHint}</div>
                        </div>
                      </button>
                    )}
                  </>
                )}
              </div>

              {detected.length > 0 && (
                <div className="apk-detected-strip">
                  <span className="apk-detected-label">{t.apk.detectedApps}</span>
                  <div className="apk-detected-items">
                    {detected.map((item) => (
                      <button
                        key={item.package}
                        type="button"
                        className="apk-detected-btn"
                        onClick={() => enterConfigure({ kind: 'import', item, label: '' })}
                      >
                        <span className="apk-found-pkg">{item.package}</span>
                        <span className="apk-detected-act">{t.apk.importBtn}</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="apk-file-step">
              <button
                type="button"
                className="apk-dropzone"
                disabled={pickingFile}
                onClick={() => void pickLocalFile()}
              >
                <Icon name="apk" className="icon apk-dropzone-ico" />
                <span className="apk-dropzone-title">
                  {pickingFile ? t.common.loading : t.apk.chooseFileBtn}
                </span>
                <span className="apk-dropzone-sub">{t.apk.fromComputerHint}</span>
              </button>

              <div className="apk-alt-links">
                <button
                  type="button"
                  className={`apk-alt-link${extraPanel === 'url' ? ' on' : ''}`}
                  aria-expanded={extraPanel === 'url'}
                  onClick={() => setExtraPanel((cur) => (cur === 'url' ? 'none' : 'url'))}
                >
                  <Icon name="download" className="icon xs" />
                  {t.apk.fromUrlLabel}
                </button>
                {runningMobile.length > 0 && (
                  <button
                    type="button"
                    className={`apk-alt-link${extraPanel === 'mobile' ? ' on' : ''}`}
                    aria-expanded={extraPanel === 'mobile'}
                    onClick={() => setExtraPanel((cur) => (cur === 'mobile' ? 'none' : 'mobile'))}
                  >
                    <Icon name="mobile" className="icon xs" />
                    {t.apk.fromMobileLabel}
                  </button>
                )}
              </div>

              {extraPanel === 'url' && (
                <div className="apk-alt-body">
                  <div className="row nowrap-row">
                    <input
                      className="input grow"
                      value={urlInput}
                      placeholder={t.apk.fromUrlPlaceholder}
                      onChange={(e) => setUrlInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && urlInput.trim() && !downloadingUrl) {
                          e.preventDefault()
                          void downloadFromUrl(urlInput)
                        }
                      }}
                    />
                    <button
                      type="button"
                      className="btn"
                      disabled={!urlInput.trim() || downloadingUrl}
                      onClick={() => void downloadFromUrl(urlInput)}
                    >
                      {downloadingUrl ? t.apk.downloadingUrl : t.apk.downloadUrlBtn}
                    </button>
                  </div>
                </div>
              )}

              {extraPanel === 'mobile' && runningMobile.length > 0 && (
                <div className="apk-alt-body">
                  <div className="row nowrap-row">
                    <select
                      className="select grow"
                      value={mobileSourceId || runningMobile[0].id}
                      onChange={(e) => { setMobileSourceId(e.target.value); setMobileScanned(false); setMobileApps([]) }}
                    >
                      {runningMobile.map((m) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                    <button type="button" className="btn" disabled={scanningMobile} onClick={() => void scanMobile()}>
                      {scanningMobile ? t.apk.syncing : t.apk.scanMobileBtn}
                    </button>
                  </div>
                  {mobileScanned && mobileApps.length === 0 && (
                    <p className="hint" style={{ margin: 0 }}>{t.apk.noMobileApps}</p>
                  )}
                  {mobileApps.length > 0 && (
                    <div className="apk-detected-items">
                      {mobileApps.map((item) => (
                        <button
                          key={item.package}
                          type="button"
                          className="apk-detected-btn"
                          onClick={() => enterConfigure({ kind: 'import', item, label: '' })}
                        >
                          <span className="apk-found-pkg">{item.package}</span>
                          <span className="apk-detected-act">{t.apk.importBtn}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      ) : (
        <div className="apk-step">
          <div className="apk-selected-head">
            {target.kind === 'file' ? (
              <div className="apk-icon lg">
                {target.inspect.iconData ? (
                  <img src={target.inspect.iconData} alt="" />
                ) : (
                  <span>{(target.inspect.label || 'AP').slice(0, 2).toUpperCase()}</span>
                )}
              </div>
            ) : target.kind === 'store' ? (
              <StoreAppIcon title={target.app.title} iconUrl={target.app.iconUrl} size="lg" />
            ) : (
              <div className="apk-icon lg">
                <span>{target.item.package.split('.').pop()?.slice(0, 2).toUpperCase() || 'AP'}</span>
              </div>
            )}

            <div className="min0 grow">
              <div className="apk-selected-name">
                {target.kind === 'file'
                  ? target.inspect.label
                  : target.kind === 'store'
                    ? target.app.title
                    : target.item.package}
              </div>
              <div className="apk-selected-meta">
                {target.kind === 'file' ? (
                  <>
                    {target.inspect.package} · v{target.inspect.versionName} · {formatBytes(target.inspect.sizeBytes)}
                    {target.inspect.abiCompatible && (
                      <> · {target.inspect.abis?.length ? target.inspect.abis.join(', ') : t.apk.universalAbi}</>
                    )}
                  </>
                ) : target.kind === 'store' ? (
                  target.app.developer && target.app.package
                    ? `${target.app.developer} · ${target.app.package}`
                    : target.app.package || target.app.developer
                ) : (
                  t.apk.importFromDeviceHint
                )}
              </div>
            </div>
          </div>

          {abiBlocked && target.kind === 'file' && (
            <div className="banner danger" role="alert">
              <Icon name="alert" className="icon sm" />
              <span className="grow">{target.inspect.abiWarning}</span>
            </div>
          )}

          {proxyId === '' && (
            <p className="apk-ip-note">
              <Icon name="alert" className="icon xs" />
              <span>{t.apk.realIpNote}</span>
            </p>
          )}

          <div className="mobile-create-summary">
            <span>{proxySummary}</span>
            <span>{oriLabel}</span>
            <span>{modelLabel}</span>
            {makeShortcut && <span>{t.apk.shortcutShort}</span>}
          </div>

          <details className="mobile-create-advanced">
            <summary>{t.drawer.advanced}</summary>
            <div className="mobile-create-fields">
              <label className="field">
                <span className="label">{t.apk.nameField}</span>
                <input
                  className="input"
                  value={label}
                  placeholder={t.apk.namePlaceholder}
                  onChange={(e) => setLabel(e.target.value)}
                />
              </label>

              <label className="field">
                <span className="label">{t.mobile.deviceModel}</span>
                <select className="select" value={model} onChange={(e) => setModel(e.target.value)}>
                  <option value="">{t.mobile.modelAuto}</option>
                  {MODELS.map(([value, text]) => <option key={value} value={value}>{text}</option>)}
                </select>
              </label>

              <div className="field">
                <span className="label">{t.apk.orientationField}</span>
                <div className="seg" role="group" aria-label={t.apk.orientationField}>
                  {(['auto', 'portrait', 'landscape'] as const).map((ori) => (
                    <button
                      key={ori}
                      type="button"
                      aria-pressed={orientation === ori}
                      onClick={() => setOrientation(ori)}
                    >
                      {ori === 'auto' ? t.apk.oriAuto : ori === 'portrait' ? t.apk.oriPortrait : t.apk.oriLandscape}
                    </button>
                  ))}
                </div>
              </div>

              <label className="field">
                <span className="label">{t.apk.notesField}</span>
                <input
                  className="input"
                  value={notes}
                  placeholder={t.apk.notesPlaceholder}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </label>

              <label className="row checkrow">
                <input type="checkbox" checked={makeShortcut} onChange={(e) => setMakeShortcut(e.target.checked)} />
                <span>{t.apk.shortcutField}</span>
              </label>
            </div>
          </details>

          {target.kind === 'store' && (
            <p className="hint" style={{ margin: 0 }}>{t.apk.storeHint}</p>
          )}
        </div>
      )}
    </Dialog>
  )
}
