// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Api, store, service } from '../api'
import { PageHead } from '../components/page-head'
import { t } from '../i18n'

/** One queued post: the app's own scheduler runs it with the profile's
 *  browser, so no agent has to be open at that time. */
export function PublishPage({ profiles }: { profiles: service.ProfileRow[] }) {
  const [tasks, setTasks] = useState<store.PublishTask[]>([])
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [profileId, setProfileId] = useState('')
  const [platform, setPlatform] = useState('instagram')
  const [kind, setKind] = useState('reel')
  const [source, setSource] = useState('')
  const [caption, setCaption] = useState('')
  const [when, setWhen] = useState(defaultWhen())

  const load = useCallback(() => {
    Api.PublishTasks().then((rows) => setTasks(rows ?? [])).catch((e) => setErr(String(e)))
  }, [])
  useEffect(() => { load(); const id = setInterval(load, 10000); return () => clearInterval(id) }, [load])

  useEffect(() => {
    if (!profileId && profiles.length > 0) setProfileId(profiles[0].id)
  }, [profiles, profileId])

  const canAdd = useMemo(() => profileId !== '' && source.trim() !== '' && when !== '' && !busy, [profileId, source, when, busy])

  const add = async () => {
    setBusy(true); setErr('')
    try {
      await Api.CreatePublishTask({
        profileId, platform, kind, title: source, caption, source,
        scheduleAt: new Date(when).toISOString(),
      } as service.PublishInput)
      setSource(''); setCaption(''); load()
    } catch (e) { setErr(`${t.publish.err}: ${String(e)}`) } finally { setBusy(false) }
  }

  const act = async (fn: () => Promise<void>) => {
    setErr('')
    try { await fn(); load() } catch (e) { setErr(String(e)) }
  }

  const status = (s: string) => (t.publish as Record<string, string>)[`st_${s}`] ?? s

  return (
    <>
      <PageHead title={t.publish.title} sub={t.publish.sub} />
      <div className="pagebody">
        <section className="card pad">
          <h3>{t.publish.form}</h3>
          <div className="row" style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'flex-end' }}>
            <label className="field"><span className="label">{t.publish.profile}</span>
              <select value={profileId} onChange={(e) => setProfileId(e.target.value)}>
                {profiles.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </label>
            <label className="field"><span className="label">{t.publish.platform}</span>
              <select value={platform} onChange={(e) => setPlatform(e.target.value)}>
                <option value="instagram">Instagram</option>
                <option value="facebook">Facebook</option>
              </select>
            </label>
            <label className="field"><span className="label">{t.publish.kind}</span>
              <select value={kind} onChange={(e) => setKind(e.target.value)}>
                <option value="reel">Reel</option>
                <option value="video">Video</option>
              </select>
            </label>
            <label className="field"><span className="label">{t.publish.file}</span>
              <input value={source} onChange={(e) => setSource(e.target.value)} placeholder="clip.mp4" />
            </label>
            <label className="field"><span className="label">{t.publish.when}</span>
              <input type="datetime-local" value={when} onChange={(e) => setWhen(e.target.value)} />
            </label>
            <button type="button" className="btn primary" disabled={!canAdd} onClick={add}>{t.publish.add}</button>
          </div>
          <label className="field"><span className="label">{t.publish.caption}</span>
            <textarea rows={3} value={caption} onChange={(e) => setCaption(e.target.value)} />
          </label>
          <p className="muted">{t.publish.hint}</p>
          {err && <div className="banner warn">{err}</div>}
        </section>

        <section className="card pad">
          <h3>{t.publish.queue}</h3>
          {tasks.length === 0 ? <p className="muted">{t.publish.empty}</p> : (
            <table className="table">
              <thead><tr>
                <th>{t.publish.when}</th><th>{t.publish.platform}</th><th>{t.publish.kind}</th>
                <th>{t.publish.file}</th><th>{t.publish.status}</th><th />
              </tr></thead>
              <tbody>
                {tasks.map((task) => (
                  <tr key={task.id}>
                    <td className="nowrap">{new Date(task.scheduleAt).toLocaleString()}</td>
                    <td>{task.platform}</td>
                    <td>{task.kind}</td>
                    <td title={task.caption}>{task.source}</td>
                    <td>
                      <span className={`state ${task.status === 'done' ? 'ok' : task.status === 'failed' ? 'warn' : ''}`} />
                      {status(task.status)}
                      {task.lastError && <small className="muted"> — {task.lastError}</small>}
                      {task.resultUrl && <a href={task.resultUrl} target="_blank" rel="noreferrer"> — {task.resultUrl}</a>}
                    </td>
                    <td className="nowrap">
                      {task.status === 'pending' && (
                        <button type="button" className="btn sm" onClick={() => act(() => Api.CancelPublishTask(task.id))}>{t.publish.cancel}</button>
                      )}
                      {(task.status === 'failed' || task.status === 'cancelled') && (
                        <button type="button" className="btn sm primary" onClick={() => act(() => Api.RetryPublishTask(task.id))}>{t.publish.retry}</button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>
    </>
  )
}

/** Tomorrow at 03:00 local, the channel's usual slot. */
function defaultWhen(): string {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  d.setHours(3, 0, 0, 0)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}
