import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { currentTheme, savedTheme, setTheme, startTheme, storageKey } from './theme'

const root = document.documentElement

beforeEach(() => {
  localStorage.clear()
  delete root.dataset.theme
})

let stop: (() => void) | undefined
afterEach(() => stop?.())

describe('startTheme', () => {
  it('applies the default when nothing is saved, without saving it', () => {
    stop = startTheme()
    expect(root.dataset.theme).toBe('space-dark')
    expect(localStorage.getItem(storageKey)).toBeNull()
  })

  it('applies the saved theme, and the default for one it no longer knows', () => {
    localStorage.setItem(storageKey, 'light')
    stop = startTheme()
    expect(root.dataset.theme).toBe('light')
    stop()

    localStorage.setItem(storageKey, 'sepia')
    stop = startTheme()
    expect(root.dataset.theme).toBe('space-dark')
  })

  it("carries the old toggle's light choice over to Space light, and drops its saved dark", () => {
    localStorage.setItem('theme', 'light')
    stop = startTheme()
    expect(root.dataset.theme).toBe('space-light')
    expect(localStorage.getItem(storageKey)).toBe('space-light')
    expect(localStorage.getItem('theme')).toBeNull()
    stop()

    localStorage.clear()
    localStorage.setItem('theme', 'dark')
    stop = startTheme()
    expect(root.dataset.theme).toBe('space-dark')
    expect(localStorage.getItem(storageKey)).toBeNull()
    expect(localStorage.getItem('theme')).toBeNull()
  })

  it('keeps a theme picked here over the old toggle', () => {
    localStorage.setItem(storageKey, 'dark')
    localStorage.setItem('theme', 'light')
    stop = startTheme()
    expect(root.dataset.theme).toBe('dark')
  })

  it('follows a theme picked in another tab', () => {
    stop = startTheme()
    localStorage.setItem(storageKey, 'light')
    window.dispatchEvent(new StorageEvent('storage', { key: storageKey, newValue: 'light' }))
    expect(currentTheme()).toBe('light')
  })
})

describe('setTheme', () => {
  it('applies and saves the theme', () => {
    setTheme('dark')
    expect(root.dataset.theme).toBe('dark')
    expect(savedTheme()).toBe('dark')
  })
})
