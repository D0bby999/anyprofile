// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { donateLinks } from '../lib/donate'

const openExternal = vi.fn()
vi.mock('../api', async (orig) => ({ ...(await orig<typeof import('../api')>()), openExternal: (u: string) => openExternal(u) }))
const { DonateSettings } = await import('./donate-settings')

const saved = donateLinks.map((l) => ({ ...l }))
afterEach(() => {
  cleanup()
  donateLinks.forEach((l, i) => { l.url = saved[i].url })
})

describe('DonateSettings', () => {
  it('ships only https links', () => {
    for (const l of donateLinks) if (l.url) expect(new URL(l.url).protocol, l.id).toBe('https:')
  })

  it('opens each link in the browser and names PostClaw for card donations', () => {
    donateLinks.find((l) => l.id === 'stripe')!.url = 'https://donate.stripe.com/test_x'
    render(<DonateSettings />)
    fireEvent.click(screen.getByRole('button', { name: t.donate.github }))
    expect(openExternal).toHaveBeenLastCalledWith('https://github.com/sponsors/D0bby999')
    fireEvent.click(screen.getByRole('button', { name: t.donate.stripe }))
    expect(openExternal).toHaveBeenLastCalledWith('https://donate.stripe.com/test_x')
    expect(screen.getByText(t.donate.stripeNote)).toBeTruthy()
  })

  it('hides a button whose link is empty', () => {
    donateLinks.find((l) => l.id === 'stripe')!.url = ''
    render(<DonateSettings />)
    expect(screen.queryByRole('button', { name: t.donate.stripe })).toBeNull()
    expect(screen.queryByText(t.donate.stripeNote)).toBeNull()
    expect(screen.getByRole('button', { name: t.donate.github })).toBeTruthy()
  })
})
