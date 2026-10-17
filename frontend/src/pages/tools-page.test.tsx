// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { partnersOf } from '../lib/partners'

const openExternal = vi.fn()
vi.mock('../api', async (orig) => ({ ...(await orig<typeof import('../api')>()), openExternal: (u: string) => openExternal(u) }))
const { ToolsPage } = await import('./tools-page')

afterEach(cleanup)

describe('ToolsPage', () => {
  it('shows every tool under its section and opens PostClaw with UTM tags', () => {
    render(<ToolsPage />)
    expect(screen.getByRole('heading', { name: t.tools.title })).toBeTruthy()
    expect(screen.getAllByRole('listitem')).toHaveLength(partnersOf('tool').length)
    expect(screen.getByRole('heading', { name: t.partners.groups.publish })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /PostClaw/ }))
    expect(openExternal).toHaveBeenCalledWith('https://postclaw.fun/?utm_source=anyprofile&utm_medium=app&utm_campaign=creator-tools')
  })

  it('opens an open-source tool at its own page, untagged', () => {
    render(<ToolsPage />)
    fireEvent.click(screen.getByRole('button', { name: /FilmCraft/ }))
    expect(openExternal).toHaveBeenLastCalledWith('https://getartcraft.com/apps/filmcraft')
  })
})
