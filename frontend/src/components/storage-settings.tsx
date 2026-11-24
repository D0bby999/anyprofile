// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onDataChanged, service } from '../api'
import { t } from '../i18n'
import { errorText, formatBytes } from '../lib/format'
import { Icon } from './icons'
import { useConfirm } from './confirm'

/** Storage use of the profiles folder, with cache and orphan cleanup. */
export function StorageSettings() {
  const [info, setInfo] = useState<service.StorageInfo | null>(null)
  const [error, setError] = useState('')
  const [freed, setFreed] = useState('')
  const [busy, setBusy] = useState('')
  const [snap, setSnap] = useState<service.SnapshotSettings | null>(null)
  const confirm = useConfirm()
  useEffect(() => { Api.SnapshotSettings().then(setSnap).catch(() => {}) }, [])

  const saveSnap = async (next: service.SnapshotSettings) => {
    setSnap(next)
    setError('')
    try {
      await Api.SetSnapshotSettings(next)
      load()
    } catch (e) {
      setError(errorText(e))
    }
  }

  const deleteSnapshots = async () => {
    if (!(await confirm({ ...t.confirm.deleteSnapshots, danger: true }))) return
    setBusy('snapshots')
    setError('')
    try {
      await Api.DeleteAllSnapshots()
      load()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy('')
    }
  }

  const load = useCallback(() => {
    Api.Storage().then(setInfo).catch((e) => setError(errorText(e)))
  }, [])
  useEffect(load, [load])
  useEffect(() => onDataChanged(['profiles', 'snapshots', 'settings', 'mobile'], load), [load])

  const orphanBytes = (info?.orphans ?? []).reduce((n, o) => n + Number(o.bytes), 0)

  const clearCache = async () => {
    setBusy('cache')
    setError('')
    setFreed('')
    try {
      const n = await Api.ClearCache()
      setFreed(t.storage.cleared(formatBytes(Number(n))))
      load()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy('')
    }
  }

  const clearUploads = async () => {
    if (!(await confirm({ ...t.confirm.clearUploads, danger: true }))) return
    setBusy('uploads')
    setError('')
    setFreed('')
    try {
      const n = await Api.ClearUploads()
      setFreed(t.storage.cleared(formatBytes(Number(n))))
      load()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy('')
    }
  }

  const deleteOrphans = async () => {
    const ids = (info?.orphans ?? []).map((o) => o.id)
    if (!ids.length || !(await confirm({ ...t.confirm.orphans(ids.length), danger: true }))) return
    setBusy('orphans')
    setError('')
    try {
      await Api.DeleteOrphans(ids)
      load()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy('')
    }
  }

  const profilesBytes = Number(info?.profilesBytes || 0)
  const snapshotsBytes = Number(info?.snapshotsBytes || 0)
  const mobileBytes = Number(info?.mobileBytes || 0)
  const runtimeBytes = Number(info?.enginesBytes || 0) + Number(info?.mobileRuntimeBytes || 0)
  const apkCacheBytes = Number(info?.apkCacheBytes || 0)
  const uploadsBytes = Number(info?.uploadsBytes || 0)
  const logsBytes = Number(info?.logsBytes || 0)
  const totalBytes = Number(info?.totalBytes || 0) ||
    (profilesBytes + snapshotsBytes + mobileBytes + runtimeBytes + apkCacheBytes + uploadsBytes + logsBytes)

  const rows: [string, number][] = [
    [t.storage.profiles, profilesBytes],
    [t.storage.snapshots, snapshotsBytes],
    ...(mobileBytes > 0 ? [[t.storage.mobileDevices, mobileBytes] as [string, number]] : []),
    ...(runtimeBytes > 0 ? [[t.storage.runtimeAndEngines, runtimeBytes] as [string, number]] : []),
    ...(apkCacheBytes > 0 ? [[t.storage.apkCache, apkCacheBytes] as [string, number]] : []),
    ...(uploadsBytes > 0 ? [[t.storage.uploads, uploadsBytes] as [string, number]] : []),
    ...(logsBytes > 0 ? [[t.storage.logs, logsBytes] as [string, number]] : []),
  ]

  return (
    <div className="field">
      {info ? (
        <dl className="storage-breakdown">
          {rows.map(([label, bytes]) => (
            <div className="storage-row" key={label}>
              <dt>{label}</dt>
              <dd>{formatBytes(bytes)}</dd>
            </div>
          ))}
          <div className="storage-row storage-total">
            <dt>{t.storage.total}</dt>
            <dd>{formatBytes(totalBytes)}</dd>
          </div>
        </dl>
      ) : (
        <p className="desc" style={{ margin: 0 }}>{t.common.loading}</p>
      )}
      <div className="row" style={{ flexWrap: 'wrap' }}>
        <button className="btn" disabled={busy !== ''} onClick={clearCache}>
          <Icon name="trash" className="icon sm" />{t.storage.clearCache}
        </button>
        {uploadsBytes > 0 && (
          <button className="btn" disabled={busy !== ''} onClick={clearUploads}>
            <Icon name="trash" className="icon sm" />{t.storage.clearUploads(formatBytes(uploadsBytes))}
          </button>
        )}
      </div>
      <span className="hint">{t.storage.clearCacheHint}</span>
      {!!info?.orphans?.length && (
        <div className="banner warn">
          <Icon name="alert" />
          <span className="grow">{t.storage.orphans(info.orphans.length, formatBytes(orphanBytes))}</span>
          <button className="btn sm danger" disabled={busy !== ''} onClick={deleteOrphans}>{t.storage.deleteOrphans}</button>
        </div>
      )}
      {snap && (
        <div className="snapsettings">
          <label className="row checkrow">
            <input type="checkbox" checked={snap.auto} onChange={(e) => saveSnap(service.SnapshotSettings.createFrom({ ...snap, auto: e.target.checked }))} />
            {t.snapshots.auto}
          </label>
          <label className="row checkrow">
            {t.snapshots.keep}
            <input className="input keepinput" type="number" min={1} max={10} value={snap.keep}
              onChange={(e) => saveSnap(service.SnapshotSettings.createFrom({ ...snap, keep: Math.max(1, Math.min(10, Number(e.target.value) || 1)) }))} />
          </label>
          <span className="hint">{info ? t.snapshots.total(formatBytes(Number(info.snapshotsBytes))) : ''}</span>
          {!!info && Number(info.snapshotsBytes) > 0 && (
            <button className="btn sm danger" disabled={busy !== ''} onClick={deleteSnapshots}>{t.snapshots.deleteAll}</button>
          )}
        </div>
      )}
      {freed && <div className="banner ok"><Icon name="check" /><span className="grow">{freed}</span></div>}
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}
