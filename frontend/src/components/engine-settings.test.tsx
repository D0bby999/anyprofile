// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { main, service } from '../api'

const installEngine = vi.fn().mockResolvedValue(undefined)
const setDefaults = vi.fn().mockResolvedValue(undefined)
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onEngineProgress: () => () => {},
  Api: {
    InstallEngine: (...a: unknown[]) => installEngine(...a),
    SetDefaults: (...a: unknown[]) => setDefaults(...a),
  },
}))
const { EngineSettings } = await import('./engine-settings')

const engines = [
  main.EngineInfo.createFrom({ engine: 'apostate', version: '0.5.1', chromium: '155.0.8059.31', installed: false, problem: '', default: true }),
  main.EngineInfo.createFrom({ engine: 'fury', version: '0.3.5', chromium: '155.0.8059.39', installed: true, problem: '', default: false }),
]
const defaults = service.Defaults.createFrom({ engine: 'apostate', site: 'other', proxy: 'auto' })

afterEach(() => { cleanup(); installEngine.mockClear(); setDefaults.mockClear() })

describe('EngineSettings', () => {
  it('shows both engines and recommends only Apostate', () => {
    render(<EngineSettings engines={engines} counts={{ fury: 2 }} defaults={defaults} onChanged={vi.fn()} />)
    expect(screen.getByText('Apostate')).toBeTruthy()
    expect(screen.getByText('Fury')).toBeTruthy()
    expect(screen.getAllByText(t.engines.recommended)).toHaveLength(1)
    expect(screen.getByText(t.engines.usedBy(2))).toBeTruthy()
  })

  it('locks the default choice for an engine that is not installed', () => {
    render(<EngineSettings engines={engines} counts={{}} defaults={defaults} onChanged={vi.fn()} />)
    const radios = screen.getAllByRole('radio') as HTMLInputElement[]
    expect(radios[0].disabled).toBe(true)
    expect(radios[1].disabled).toBe(false)
    fireEvent.click(radios[1])
    expect(setDefaults.mock.calls[0][0]).toMatchObject({ engine: 'fury', site: 'other', proxy: 'auto' })
  })

  it('installs the engine whose button was pressed', async () => {
    const onChanged = vi.fn()
    render(<EngineSettings engines={engines} counts={{}} defaults={defaults} onChanged={onChanged} />)
    fireEvent.click(screen.getByRole('button', { name: t.engines.install }))
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
    expect(installEngine).toHaveBeenCalledWith('apostate')
  })
})
