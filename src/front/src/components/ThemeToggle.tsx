import { useEffect, useState } from 'react'

export type Theme = 'dark' | 'light'

const key = 'theme'

/** The saved theme, dark when none is saved or storage is blocked. */
export function savedTheme(): Theme {
  try {
    return localStorage.getItem(key) === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

/** Switches between the dark and light palettes, remembering the choice in this browser. */
export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(savedTheme)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem(key, theme)
    } catch {
      // Private mode or blocked storage: the theme still applies for this visit.
    }
  }, [theme])

  const next = theme === 'dark' ? 'light' : 'dark'
  return (
    <button type="button" aria-label={`Switch to ${next} theme`} aria-pressed={theme === 'light'} onClick={() => setTheme(next)}>
      {theme === 'dark' ? '☀' : '☾'}
    </button>
  )
}
