#!/usr/bin/env node
// SPDX-License-Identifier: AGPL-3.0-or-later
// Runs a flow JSON against the local automation API.
//   anyprofile run flow.json [--var k=v] [--dry-run]
import { readFile, writeFile } from 'node:fs/promises'

const base = (process.env.ANYPROFILE_API ?? 'http://127.0.0.1:51525').replace(/\/$/, '')
const token = process.env.ANYPROFILE_TOKEN ?? ''

function subst(value, vars) {
  if (typeof value === 'string') {
    return value.replace(/\{\{([A-Za-z0-9_]+)\}\}/g, (_, k) => {
      if (!(k in vars)) throw new Error(`missing variable ${k}`)
      return vars[k]
    })
  }
  if (Array.isArray(value)) return value.map((v) => subst(v, vars))
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, subst(v, vars)]))
  return value
}

const [cmd, file, ...rest] = process.argv.slice(2)

if (cmd === 'record') {
  const opt = (name, def = '') => {
    const i = rest.indexOf(name)
    return i >= 0 && rest[i + 1] ? rest[i + 1] : def
  }
  const profile = opt('--profile'), device = opt('--device'), out = opt('--out', 'flow.json')
  if (!profile && !device) { console.error('usage: anyprofile record --profile "Shop VN 01" [--out flow.json]'); process.exit(2) }
  const res = await fetch(`${base}/v1/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ profile, device, record: true }),
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) { console.error(body.error?.message ?? `HTTP ${res.status}`); process.exit(1) }
  console.log(`Recording session ${body.id}. Run actions with "session": "${body.id}" in /v1/act, then Ctrl-C to save.`)
  const stop = async () => {
    const rec = await fetch(`${base}/v1/sessions/${body.id}/record`, { headers: { Authorization: `Bearer ${token}` } })
    const flow = await rec.json()
    await writeFile(out, JSON.stringify(flow, null, 2) + '\n')
    await fetch(`${base}/v1/sessions/${body.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
    console.log(`Saved ${flow.steps?.length ?? 0} steps to ${out}`)
    process.exit(0)
  }
  process.on('SIGINT', stop)
  await new Promise(() => {})
}

if (cmd !== 'run' || !file) {
  console.error('usage: anyprofile run flow.json [--var k=v] [--dry-run]')
  console.error('       anyprofile record --profile "Shop VN 01" [--out flow.json]')
  process.exit(2)
}
const vars = { ...process.env }
for (let i = 0; i < rest.length; i++) {
  if (rest[i] === '--var' && rest[i + 1]) { const [k, ...v] = rest[++i].split('='); vars[k] = v.join('=') }
}
const flow = subst(JSON.parse(await readFile(file, 'utf8')), vars)
if (rest.includes('--dry-run')) { console.log(JSON.stringify(flow, null, 2)); process.exit(0) }

const res = await fetch(`${base}/v1/run`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
  body: JSON.stringify(flow),
})
const body = await res.json().catch(() => ({}))
if (!res.ok) { console.error(body.error?.message ?? `HTTP ${res.status}`); process.exit(1) }
console.log(JSON.stringify(body, null, 2))
process.exit(body.steps?.every((s) => s.ok) ? 0 : 1)
