// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { main, service } from '../api'

const installEngine = vi.fn().mockResolvedValue(undefined)
const installVersion = vi.fn().mockResolvedValue(undefined)
const removeVersion = vi.fn().mockResolvedValue(undefined)
const setDefaults = vi.fn().mockResolvedValue(undefined)
const rel = (engine: string, version: string, installed: boolean, profiles = 0) => service.EngineRelease.createFrom({
  engine, version, chromium: '155.0.8059.39', installed, problem: '', date: '2026-10-15', notes: '', profiles })
const releases: Record<string, service.EngineRelease[]> = {
  apostate: [rel('apostate', '0.5.1', false)],
  fury: [rel('fury', '0.4.5', false), rel('fury', '0.3.5', true, 2)],
}
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onEngineProgress: () => () => {},
  onDataChanged: () => () => {},
  Api: {
    InstallEngine: (...a: unknown[]) => installEngine(...a),
    InstallEngineVersion: (...a: unknown[]) => installVersion(...a),
    RemoveEngineVersion: (...a: unknown[]) => removeVersion(...a),
    EngineReleases: (id: string) => Promise.resolve(releases[id]),
    SetDefaults: (...a: unknown[]) => setDefaults(...a),
  },
}))
const { EngineSettings } = await import('./engine-settings')
const { ConfirmProvider } = await import('./confirm')

const engines = [
  main.EngineInfo.createFrom({ engine: 'apostate', version: '0.5.1', chromium: '155.0.8059.31', installed: false, problem: '', default: true }),
  main.EngineInfo.createFrom({ engine: 'fury', version: '0.3.5', chromium: '155.0.8059.39', installed: true, problem: '', default: false }),
]
const defaults = service.Defaults.createFrom({ engine: 'apostate', site: 'other', proxy: 'auto' })

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('EngineSettings', () => {
  it('shows both engines and recommends only Apostate', () => {
    render(<EngineSettings engines={engines} counts={{ fury: 2 }} defaults={defaults} onChanged={vi.fn()} />)
    expect(screen.getByText('Apostate')).toBeTruthy()
    expect(screen.getByText('Fury')).toBeTruthy()
    expect(screen.getAllByText(t.engines.recommended)).toHaveLength(1)
    expect(screen.getByText(t.engines.usedBy(2))).toBeTruthy()
  })

  it('folds away an engine that is not installed, not chosen and not used', () => {
    const fresh = [engines[0], main.EngineInfo.createFrom({ ...engines[1], installed: false })]
    const { container } = render(<EngineSettings engines={fresh} counts={{}} defaults={defaults} onChanged={vi.fn()} />)
    const older = container.querySelector('details.engine-older')!
    expect(older.querySelector('summary')!.textContent).toBe(`${t.engines.older}: Fury`)
    expect(older.querySelector('.btn.primary')).toBeNull()
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

  it('offers a newer version and lists every version', async () => {
    const onChanged = vi.fn()
    render(<ConfirmProvider><EngineSettings engines={engines} counts={{}} defaults={defaults} onChanged={onChanged} /></ConfirmProvider>)
    // Fury has 0.3.5 installed and 0.4.5 not yet: offered. Apostate has none installed: no banner.
    expect(await screen.findByText(t.engines.newer('0.4.5'))).toBeTruthy()
    expect(screen.queryByText(t.engines.newer('0.5.1'))).toBeNull()
    expect(screen.getByText(t.engines.versions(2))).toBeTruthy()
    expect(screen.getByText(t.engines.profilesOn(2))).toBeTruthy()

    fireEvent.click(screen.getAllByRole('button', { name: t.engines.download })[0])
    await waitFor(() => expect(installVersion).toHaveBeenCalledWith('fury', '0.4.5'))

    // Remove asks first; its profiles are named in the question.
    fireEvent.click(screen.getByRole('button', { name: t.engines.remove }))
    expect(await screen.findByText(t.engines.removeBody(2))).toBeTruthy()
    fireEvent.click(screen.getAllByRole('button', { name: t.engines.remove }).at(-1)!)
    await waitFor(() => expect(removeVersion).toHaveBeenCalledWith('fury', '0.3.5'))
  })
})
