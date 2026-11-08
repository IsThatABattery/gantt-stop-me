import { describe, expect, it } from 'vitest'
import { countdown, isFinalDay, timeAgo } from './time'

const HOUR = 60 * 60 * 1000
const now = Date.UTC(2026, 10, 7, 12, 0)
const iso = (ms: number) => new Date(now + ms).toISOString()

describe('countdown', () => {
  it('shows days and hours', () => {
    expect(countdown(iso(3 * 24 * HOUR + 6 * HOUR), now)).toBe('3d 06h')
  })
  it('shows hours and minutes in the last day', () => {
    expect(countdown(iso(5 * HOUR + 12 * 60 * 1000), now)).toBe('05h 12m')
  })
  it('never goes negative', () => {
    expect(countdown(iso(-HOUR), now)).toBe('00h 00m')
  })
})

describe('isFinalDay', () => {
  it('is true inside the last 24 hours only', () => {
    expect(isFinalDay(iso(23 * HOUR), now)).toBe(true)
    expect(isFinalDay(iso(25 * HOUR), now)).toBe(false)
  })
})

describe('timeAgo', () => {
  it('picks minutes, hours or days', () => {
    expect(timeAgo(iso(-12 * 60 * 1000), now)).toBe('12m ago')
    expect(timeAgo(iso(-3 * HOUR), now)).toBe('3h ago')
    expect(timeAgo(iso(-50 * HOUR), now)).toBe('2d ago')
  })
})
