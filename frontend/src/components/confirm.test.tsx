// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider, useConfirm, type ConfirmOptions } from './confirm'

let ask: ReturnType<typeof useConfirm>
function Grab() { ask = useConfirm(); return null }
const opts: ConfirmOptions = { title: 'Delete 3 folders?', body: 'This cannot be undone.', confirm: 'Delete folders', danger: true }

afterEach(cleanup)

const open = async (o = opts) => {
  render(<ConfirmProvider><Grab /></ConfirmProvider>)
  let answer!: Promise<boolean>
  await act(async () => { answer = ask(o) })
  return { answer } // wrapped: an async function would otherwise wait for the answer
}

describe('confirm', () => {
  it('answers false on Cancel, which has the focus', async () => {
    const { answer } = await open()
    expect(document.activeElement?.textContent).toBe(t.common.cancel)
    fireEvent.click(screen.getByRole('button', { name: t.common.cancel }))
    expect(await answer).toBe(false)
  })

  it('answers true on the confirm button, styled as danger', async () => {
    const { answer } = await open()
    const btn = screen.getByRole('button', { name: 'Delete folders' })
    expect(btn.className).toContain('stop')
    fireEvent.click(btn)
    expect(await answer).toBe(true)
  })

  it('answers false on Escape', async () => {
    const { answer } = await open({ ...opts, danger: false })
    expect(screen.getByRole('button', { name: 'Delete folders' }).className).toContain('primary')
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(await answer).toBe(false)
  })
})
