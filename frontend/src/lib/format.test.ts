// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { allTags, errorText, parseTags, relativeTime } from './format'

describe('relativeTime', () => {
  const now = new Date('2026-10-12T12:00:00Z')
  it('handles never and bad input', () => {
    expect(relativeTime('', now)).toBe('Chưa mở')
    expect(relativeTime('garbage', now)).toBe('Chưa mở')
  })
  it('scales units', () => {
    expect(relativeTime('2026-10-12T11:59:40Z', now)).toBe('vừa xong')
    expect(relativeTime('2026-10-12T11:15:00Z', now)).toBe('45 phút trước')
    expect(relativeTime('2026-10-12T07:00:00Z', now)).toBe('5 giờ trước')
    expect(relativeTime('2026-10-09T12:00:00Z', now)).toBe('3 ngày trước')
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

it('errorText unwraps strings and errors', () => {
  expect(errorText('boom')).toBe('boom')
  expect(errorText(new Error('bad'))).toBe('bad')
})
