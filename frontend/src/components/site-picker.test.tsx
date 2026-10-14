// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { site } from '../api'
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

  it('picks several platforms, each once, and shows Other when none', () => {
    const onChange = vi.fn()
    const { rerender } = render(<SitesPicker value={[]} onChange={onChange} sites={sites} recent={[]} />)
    expect(screen.getByRole('button', { name: 'Platform' }).textContent).toContain('Other')
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    expect(screen.queryByRole('option', { name: /Other/ })).toBeNull()
    fireEvent.click(screen.getByRole('option', { name: /Etsy/ }))
    expect(onChange).toHaveBeenLastCalledWith(['etsy'])
    rerender(<SitesPicker value={['etsy']} onChange={onChange} sites={sites} recent={[]} />)
    fireEvent.click(screen.getByRole('option', { name: /TikTok/ }))
    expect(onChange).toHaveBeenLastCalledWith(['etsy', 'tiktok'])
    rerender(<SitesPicker value={['etsy', 'tiktok']} onChange={onChange} sites={sites} recent={[]} />)
    expect(screen.getByRole('button', { name: 'Platform' }).textContent).toContain('Etsy, TikTok')
    fireEvent.click(screen.getByRole('option', { name: /Etsy/ }))
    expect(onChange).toHaveBeenLastCalledWith(['tiktok'])
  })
})
