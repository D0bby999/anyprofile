// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { MODELS, parseIdentity } from '../lib/mobile-presets'
import { Icon } from './icons'
import { InlineName } from './drawer/inline-name'
import { NotesGroup } from './drawer/notes-group'
import { ProxySelect, RealIpWarning } from './proxy-select'
import { useToast } from './toast'

type Props = {
  app: store.MobileAPK
  proxies: service.ProxyRow[]
  canUse: boolean
  working: boolean
  onClose: () => void
  onOpen: (app: store.MobileAPK) => void
  onCreateShortcut: (pkg: string) => void
  onRemove: (app: store.MobileAPK) => void
  onChanged: () => void
}

/** Standalone APK editor panel: mirrors ProfileDrawer's split-drawer design next to the table. */
export function ApkDrawer({
  app, proxies, canUse, working, onClose, onOpen, onCreateShortcut, onRemove, onChanged,
}: Props) {
  const toast = useToast()
  const [proxyId, setProxyId] = useState(app.proxyId || '')
  const [applyingProxy, setApplyingProxy] = useState(false)
  const [advanced, setAdvanced] = useState(false)

  useEffect(() => { setProxyId(app.proxyId || '') }, [app.package, app.proxyId])

  const update = async (patch: Partial<{ label: string; orientation: string; proxyId: string; notes: string; identityTemplate: string }>) => {
    try {
      await Api.UpdateAPK(service.APKUpdate.createFrom({
        package: app.package,
        label: patch.label ?? app.label,
        orientation: patch.orientation ?? (app.orientation || 'auto'),
        proxyId: patch.proxyId ?? (app.proxyId || ''),
        notes: patch.notes ?? (app.notes || ''),
        identityTemplate: patch.identityTemplate ?? '',
      }))
      onChanged()
      return true
    } catch (e) {
      toast.error(errorText(e))
      return false
    }
  }

  const proxyChanged = proxyId !== (app.proxyId || '')
  const currentProxy = app.proxyId ? proxies.find((p) => p.id === app.proxyId) : undefined
  const proxyFacts = currentProxy
    ? [currentProxy.country, currentProxy.exitIp || `${currentProxy.host}:${currentProxy.port}`, currentProxy.timezone].filter(Boolean).join(' · ')
    : t.common.direct

  return (
    <aside className="drawer card" aria-label={t.apk.settingsTitle(app.label)}>
      <header className="drawer-head">
        <div className="drawer-title">
          <div className="apk-icon sm">
            {app.iconData ? <img src={app.iconData} alt="" /> : <span>{app.label.slice(0, 2).toUpperCase()}</span>}
          </div>
          <InlineName value={app.label} onSave={(label) => update({ label })} />
          <span className="spacer" />
          <button type="button" className="iconbtn sm" onClick={onClose} aria-label={t.drawer.close} title={t.drawer.close}>
            <Icon name="x" />
          </button>
        </div>
        <div className="drawer-sub">
          <span>{app.package} · v{app.versionName}</span>
        </div>
        <div className="drawer-actions">
          <span className="chip">{app.abis?.length ? app.abis[0] : t.apk.universalAbi}</span>
          {app.shortcutPath && <span className="chip ok">{t.apk.shortcutCreated}</span>}
          <span className="spacer" />
          <button
            type="button"
            className={`btn sm primary actbtn${working ? ' busy' : ''}`}
            aria-busy={working}
            disabled={!canUse || working}
            onClick={() => onOpen(app)}
          >
            <Icon name={working ? 'refresh' : 'play'} className="icon sm" />
            {working ? t.apk.opening : t.apk.open}
          </button>
        </div>
      </header>

      <div className="drawer-body">
        <section className="dgroup" aria-label={t.drawer.network}>
          <h3>{t.drawer.network}</h3>
          <div className="row nowrap-row">
            <ProxySelect
              value={proxyId}
              onChange={setProxyId}
              proxies={proxies}
              currentProfile={app.label}
              disabled={!canUse}
            />
            {proxyChanged && (
              <button
                type="button"
                className={`btn sm primary${applyingProxy ? ' busy' : ''}`}
                aria-busy={applyingProxy}
                disabled={applyingProxy || !canUse}
                onClick={async () => {
                  setApplyingProxy(true)
                  if (!(await update({ proxyId }))) setProxyId(app.proxyId || '')
                  setApplyingProxy(false)
                }}
              >
                {t.drawer.apply}
              </button>
            )}
          </div>
          <span className="dfacts">{proxyFacts}</span>
          {proxyId === '' && <RealIpWarning />}
        </section>

        <section className="dgroup" aria-label={t.mobile.deviceModel}>
          <h3>{t.mobile.deviceModel}</h3>
          {(() => {
            const idn = parseIdentity(app.identityJson)
            const curTpl = idn?.template ?? ''
            return (
              <>
                <label className="field">
                  <select
                    className="select"
                    aria-label={t.mobile.deviceModel}
                    value={curTpl}
                    disabled={!canUse || working}
                    onChange={(e) => { void update({ identityTemplate: e.target.value || 'auto' }) }}
                  >
                    <option value="">{t.mobile.modelAuto}</option>
                    {curTpl && !MODELS.some(([value]) => value === curTpl) && (
                      <option value={curTpl}>{idn?.model ? `${idn.manufacturer || idn.brand || ''} ${idn.model}`.trim() : curTpl}</option>
                    )}
                    {MODELS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                  </select>
                </label>
                {(idn || app.androidId) && (
                  <>
                    <dl className="facts" style={{ marginTop: 8 }}>
                      {idn?.model && <><dt>Model</dt><dd>{idn.manufacturer || idn.brand} {idn.model}</dd></>}
                      {idn?.glRenderer && <><dt>{t.mobile.gpuLabel}</dt><dd>{idn.glVendor ? `${idn.glVendor} ` : ''}{idn.glRenderer}</dd></>}
                      {idn?.imei && <><dt>{t.mobile.imeiLabel}</dt><dd className="mono">{idn.imei}</dd></>}
                      {idn?.serial && <><dt>{t.mobile.serialNum}</dt><dd className="mono">{idn.serial}</dd></>}
                      {app.androidId && <><dt>Android ID</dt><dd className="mono">{app.androidId}</dd></>}
                    </dl>
                    <div className="row" style={{ marginTop: 8 }}>
                      <button
                        type="button"
                        className="btn sm"
                        disabled={!canUse || working}
                        onClick={async () => {
                          if (await update({ identityTemplate: curTpl ? `regen:${curTpl}` : 'regen' })) {
                            toast.success(t.mobile.identityRegenerated)
                          }
                        }}
                      >
                        <Icon name="refresh" className="icon sm" />{t.mobile.regenIdentity}
                      </button>
                    </div>
                  </>
                )}
              </>
            )
          })()}
        </section>

        <section className="dgroup" aria-label={t.apk.orientationField}>
          <h3>{t.apk.orientationField}</h3>
          <div className="seg" role="group" aria-label={t.apk.orientationField}>
            {(['auto', 'portrait', 'landscape'] as const).map((ori) => (
              <button
                key={ori}
                type="button"
                disabled={!canUse || working}
                aria-pressed={(app.orientation || 'auto') === ori}
                onClick={() => { void update({ orientation: ori }) }}
              >
                {ori === 'auto' ? t.apk.oriAuto : ori === 'portrait' ? t.apk.oriPortrait : t.apk.oriLandscape}
              </button>
            ))}
          </div>
        </section>

        <section className="dgroup" aria-label={t.apk.colShortcut}>
          <h3>{t.apk.colShortcut}</h3>
          <div className="row">
            <button
              type="button"
              className="btn sm"
              disabled={!canUse || working}
              onClick={() => onCreateShortcut(app.package)}
            >
              <Icon name="apk" className="icon xs" />
              {app.shortcutPath ? t.apk.shortcutCreated : t.apk.createShortcut}
            </button>
          </div>
          {app.shortcutPath && <span className="dfacts">{app.shortcutPath}</span>}
        </section>

        <NotesGroup value={app.notes || ''} onSave={(notes) => update({ notes })} />

        <section className="dgroup">
          <button type="button" className="disclosure" aria-expanded={advanced} onClick={() => setAdvanced(!advanced)}>
            <Icon name="chevron" className="icon sm" />{t.drawer.advanced}
          </button>
          {advanced && (
            <>
              <span className="dfacts">
                {t.apk.colAbi}: {app.abis?.length ? app.abis.join(', ') : t.apk.universalAbi}
              </span>
              {app.lastOpenAt && (
                <span className="dfacts">{relativeTime(app.lastOpenAt)}</span>
              )}
              <button
                type="button"
                className="btn sm danger textdanger"
                disabled={working}
                onClick={() => onRemove(app)}
              >
                <Icon name="trash" className="icon sm" />{t.apk.remove}
              </button>
            </>
          )}
        </section>
      </div>
    </aside>
  )
}
