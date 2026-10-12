// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { t } from '../i18n'
import { allTags, errorText, parseTags, relativeTime } from './format'

describe('relativeTime', () => {
  const now = new Date('2026-10-12T12:00:00Z')
  it('handles never and bad input', () => {
    expect(relativeTime('', now)).toBe(t.time.never)
    expect(relativeTime('garbage', now)).toBe(t.time.never)
  })
  it('scales units', () => {
    expect(relativeTime('2026-10-12T11:59:40Z', now)).toBe(t.time.justNow)
    expect(relativeTime('2026-10-12T11:15:00Z', now)).toBe(t.time.minutes(45))
    expect(relativeTime('2026-10-12T07:00:00Z', now)).toBe(t.time.hours(5))
    expect(relativeTime('2026-10-09T12:00:00Z', now)).toBe(t.time.days(3))
  })
})

describe('tags', () => {
  it('parses, trims and dedupes', () => {
    expect(parseTags(' tiktok, kênh phụ ,,tiktok')).toEqual(['tiktok', 'kênh phụ'])
  })
  it('collects sorted unique tags', () => {
    expect(allTags([{ tags: ['b', 'a'] }, { tags: ['a'] }])).toEqual(['a', 'b'])
  })
})

describe('errorText', () => {
  it('unwraps strings and errors', () => {
    expect(errorText('boom')).toBe('boom')
    expect(errorText(new Error('bad'))).toBe('bad')
  })
  it('translates coded errors and falls back to the message', () => {
    expect(errorText('anyprofile:proxy_in_use: x')).toBe(t.errors.proxy_in_use)
    expect(errorText(new Error('anyprofile:unknown_code: hello'))).toBe('hello')
    expect(errorText('plain')).toBe('plain')
  })
})
