// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { site } from '../api'
import { t } from '../i18n'
import { SitePicker, SitesPicker } from './site-picker'

const mk = (id: string, name: string, group = 'other', domains: string[] = []) =>
  site.Site.createFrom({ id, name, group, domains, startUrl: '', icon: '', color: '#6B7280', custom: false })

const sites = [
  mk('tiktok', 'TikTok', 'video', ['tiktok.com']),
  mk('etsy', 'Etsy', 'commerce', ['etsy.com']),
  mk('amazon', 'Amazon', 'commerce', ['amazon.com']),
  mk('other', 'Other', 'other'),
]

afterEach(cleanup)

describe('SitePicker', () => {
  it('filters on typing and picks with Enter', () => {
    const onChange = vi.fn()
    render(<SitePicker value="other" onChange={onChange} sites={sites} recent={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    const search = screen.getByPlaceholderText(/Search platforms/)
    fireEvent.change(search, { target: { value: 'ets' } })
    expect(screen.queryByRole('option', { name: /Amazon/ })).toBeNull()
    const etsy = screen.getByRole('option', { name: /Etsy/ })
    expect(etsy).toBeTruthy()
    fireEvent.keyDown(search, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith('etsy')
    expect(screen.queryByPlaceholderText(/Search platforms/)).toBeNull()
  })

  it('closes on Escape without picking', () => {
    const onChange = vi.fn()
    render(<SitePicker value="other" onChange={onChange} sites={sites} recent={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    const search = screen.getByPlaceholderText(/Search platforms/)
    fireEvent.keyDown(search, { key: 'Escape' })
    expect(screen.queryByPlaceholderText(/Search platforms/)).toBeNull()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('shows a Recent group first', () => {
    render(<SitePicker value="other" onChange={vi.fn()} sites={sites} recent={['etsy', 'tiktok']} />)
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    expect(screen.getByText('Recent')).toBeTruthy()
    const options = screen.getAllByRole('option').map((o) => o.textContent)
    expect(options[0]).toContain('Etsy')
  })

  it('chooses several platforms in a modal and applies them only on Done', () => {
    const onChange = vi.fn()
    const { rerender } = render(<SitesPicker value={[]} onChange={onChange} sites={sites} recent={[]} />)
    expect(screen.getByText(t.sites.noneChosen)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.sites.add }))
    const modal = screen.getByRole('dialog', { name: t.sites.pick })
    expect(within(modal).queryByRole('option', { name: /Other/ })).toBeNull()
    fireEvent.click(within(modal).getByRole('option', { name: /Etsy/ }))
    fireEvent.click(within(modal).getByRole('option', { name: /TikTok/ }))
    expect(within(modal).getByText(t.sites.samePerson)).toBeTruthy()
    expect(onChange).not.toHaveBeenCalled()
    fireEvent.click(within(modal).getByRole('button', { name: t.sites.done }))
    expect(onChange).toHaveBeenLastCalledWith(['etsy', 'tiktok'])
    expect(screen.queryByRole('dialog')).toBeNull()

    rerender(<SitesPicker value={['etsy', 'tiktok']} onChange={onChange} sites={sites} recent={[]} />)
    expect(screen.getByText(t.sites.samePerson)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.sites.remove('Etsy') }))
    expect(onChange).toHaveBeenLastCalledWith(['tiktok'])
  })

  it('keeps the platforms when the modal is cancelled, and toggles the first match with Enter', () => {
    const onChange = vi.fn()
    render(<SitesPicker value={['etsy']} onChange={onChange} sites={sites} recent={[]} />)
    fireEvent.click(screen.getByRole('button', { name: t.sites.change }))
    const search = screen.getByPlaceholderText(/Search platforms/)
    fireEvent.change(search, { target: { value: 'tik' } })
    fireEvent.keyDown(search, { key: 'Enter' })
    expect(screen.getByRole('option', { name: /TikTok/ }).getAttribute('aria-selected')).toBe('true')
    fireEvent.click(screen.getByRole('button', { name: t.common.cancel }))
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
