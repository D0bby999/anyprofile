// SPDX-License-Identifier: 0BSD
// AnyProfile + Puppeteer starter. Put your steps in task(); the rest opens the
// profile through the local API, connects to its browser and closes it again.
//
//   ANYPROFILE_TOKEN=… node run.mjs "<profile id, exact name or #serial>" [url]
import puppeteer from 'puppeteer-core'

const port = process.env.ANYPROFILE_PORT ?? '51525'
const token = process.env.ANYPROFILE_TOKEN
const api = `http://127.0.0.1:${port}/v1`

async function call(method, path, body) {
  const r = await fetch(api + path, {
    method,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: body && JSON.stringify(body),
  })
  const j = await r.json()
  if (!r.ok) throw new Error(`${method} ${path}: ${j.error?.code}: ${j.error?.message}`)
  return j
}

// A profile by id, exact name (any case) or #serial, as AnyProfile finds them.
async function findProfile(ref) {
  const { profiles } = await call('GET', '/profiles')
  const serial = ref.replace(/^#/, '')
  const hits = profiles.filter((p) => p.id === ref || String(p.serial) === serial || p.name.toLowerCase() === ref.toLowerCase())
  if (hits.length !== 1) throw new Error(hits.length ? `${hits.length} profiles match "${ref}"; use the id` : `no profile "${ref}"`)
  return hits[0]
}

// Your steps. Wait for selectors or navigation, never for a fixed time.
async function task(page, url) {
  await page.goto(url, { waitUntil: 'domcontentloaded' })
  return { title: await page.title(), url: page.url() }
}

async function main() {
  const [ref, url = 'https://www.youtube.com/'] = process.argv.slice(2)
  if (!token || !ref) {
    console.error('usage: ANYPROFILE_TOKEN=… node run.mjs "<profile>" [url]')
    process.exit(2)
  }
  const profile = await findProfile(ref)
  const wasOpen = profile.state === 'running'
  // A profile open without DevTools is refused (api_running_without_devtools): close it first.
  const opened = await call('POST', `/profiles/${profile.id}/start`, { devtools: true })
  let browser
  try {
    // defaultViewport: null keeps the window's own size, which matches the fingerprint.
    browser = await puppeteer.connect({ browserWSEndpoint: opened.webSocketDebuggerUrl, defaultViewport: null })
    const pages = await browser.pages() // the profile's own pages, with its cookies
    const page = pages[0] ?? (await browser.newPage())
    console.log(JSON.stringify(await task(page, url)))
  } finally {
    await browser?.disconnect() // never browser.close(): AnyProfile closes it and saves the session
    if (!wasOpen) await call('POST', `/profiles/${profile.id}/stop`).catch((e) => console.error(e.message))
  }
}

main().catch((e) => {
  console.error(e.message)
  process.exit(1)
})
