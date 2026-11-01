// SPDX-License-Identifier: AGPL-3.0-or-later
// Thin client for AnyProfile's local automation API. It reimplements no
// waiting or retry: the app already does that inside each action.

export type Target = {
  ref?: string
  text?: string
  role?: string
  name?: string
  label?: string
  desc?: string
  testId?: string
  point?: { x: number; y: number }
}

export type Match = {
  ref: string
  role: string
  name: string
  testId?: string
  enabled: boolean
  x1?: number; y1?: number; x2?: number; y2?: number
}

export type SessionOptions = {
  profile?: string
  device?: string
  base?: string
  token?: string
}

const baseURL = (o: SessionOptions) => (o.base ?? process.env.ANYPROFILE_API ?? 'http://127.0.0.1:51525').replace(/\/$/, '')
const token = (o: SessionOptions) => o.token ?? process.env.ANYPROFILE_TOKEN ?? ''

export class Session {
  constructor(private readonly opts: SessionOptions) {
    if (!opts.profile && !opts.device) throw new Error('give profile or device')
  }

  async act<T = unknown>(action: string, args: Record<string, unknown> = {}): Promise<T> {
    const res = await fetch(`${baseURL(this.opts)}/v1/act`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token(this.opts)}` },
      body: JSON.stringify({ profile: this.opts.profile, device: this.opts.device, action, ...args }),
    })
    const body = await res.json() as { ok?: boolean; result?: T; error?: { code: string; message: string } }
    if (!res.ok || !body.ok) {
      const e = body.error
      throw new Error(e ? `${e.code}: ${e.message}` : `HTTP ${res.status}`)
    }
    return body.result as T
  }

  goto(url: string) { return this.act('navigate', { url }) }
  find(target: Target, limit = 10) { return this.act<Match[]>('find', { target, limit }) }
  click(target: Target | string) { return this.act('click', typeof target === 'string' ? { ref: target } : { target }) }
  type(target: Target, text: string, opts: { keep?: boolean; submit?: boolean } = {}) {
    return this.act('type', { target, text, keep: opts.keep, submit: opts.submit })
  }
  tap(target: Target | string) { return this.act('tap', typeof target === 'string' ? { ref: target } : { target }) }
  read(target?: Target) { return this.act<string>('read', target ? { target } : {}) }
  screenshot() { return this.act<{ base64: string; contentType: string }>('screenshot') }
  wait(text: string, opts: { gone?: boolean; seconds?: number } = {}) {
    return this.act('wait', { text, gone: opts.gone, seconds: opts.seconds })
  }

  /** Nothing is leased yet (phase 5); close is explicit and safe to omit. */
  async close() { return }
}

export async function session(opts: SessionOptions) { return new Session(opts) }

export const anyprofile = { session }
