// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { InstallProgress } from './install-progress'

afterEach(cleanup)

describe('InstallProgress', () => {
  it('shows the download, then says it is unpacking once every byte is in', () => {
    const { rerender } = render(<InstallProgress progress={{ engine: 'android-vm-image', done: 524288000, total: 1048576000 }} />)
    expect(screen.getByText(t.engines.installing(50, '500 MB / 1000 MB'))).toBeTruthy()
    rerender(<InstallProgress progress={{ engine: 'android-vm-image', done: 1048576000, total: 1048576000 }} />)
    expect(screen.getByText(t.engines.unpacking)).toBeTruthy()
  })
})
