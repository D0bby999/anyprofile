// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { partnersOf } from '../lib/partners'

const openExternal = vi.fn()
vi.mock('../api', async (orig) => ({ ...(await orig<typeof import('../api')>()), openExternal: (u: string) => openExternal(u) }))
const { ToolsDialog } = await import('./tools-dialog')

afterEach(cleanup)

describe('ToolsDialog', () => {
  it('shows every tool under its section and opens PostClaw with UTM tags', () => {
    render(<ToolsDialog onClose={vi.fn()} />)
    expect(screen.getByRole('dialog', { name: t.tools.title })).toBeTruthy()
    expect(screen.getAllByRole('listitem')).toHaveLength(partnersOf('tool').length)
    expect(screen.getByRole('heading', { name: t.partners.groups.publish })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /PostClaw/ }))
    expect(openExternal).toHaveBeenCalledWith('https://postclaw.fun/?utm_source=anyprofile&utm_medium=app&utm_campaign=creator-tools')
  })

  it('shows the Pro invitation only when an upgrade action is available', () => {
    const openPro = vi.fn()
    const { rerender } = render(<ToolsDialog onClose={vi.fn()} onOpenPro={openPro} />)
    fireEvent.click(screen.getByRole('button', { name: t.tools.proOpen }))
    expect(openPro).toHaveBeenCalledOnce()
    rerender(<ToolsDialog onClose={vi.fn()} />)
    expect(screen.queryByRole('button', { name: t.tools.proOpen })).toBeNull()
  })

  it('opens an open-source tool at its own page, untagged', () => {
    render(<ToolsDialog onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /FilmCraft/ }))
    expect(openExternal).toHaveBeenLastCalledWith('https://getartcraft.com/apps/filmcraft')
    fireEvent.click(screen.getByRole('button', { name: /Craft Anything/ }))
    expect(openExternal).toHaveBeenLastCalledWith('https://github.com/storytold/craft-anything')
    fireEvent.click(screen.getByRole('button', { name: /SolidCraft/ }))
    expect(openExternal).toHaveBeenLastCalledWith('https://github.com/tangency-labs/solidcraft')
  })
})
