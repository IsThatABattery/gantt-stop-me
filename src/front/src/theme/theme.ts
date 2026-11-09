import { useSyncExternalStore } from 'react'
import { defaultTheme, isThemeId, type ThemeId } from './themes'

/** Where this browser keeps the chosen theme. index.html reads it too, to apply it before first paint. */
export const storageKey = 'gsm.theme'

/** The old dark and light toggle's key. Its two values were the space palettes. */
const legacyKey = 'theme'

const listeners = new Set<() => void>()

/** The theme in use: the root's data-theme, which is what the stylesheets match. */
export function currentTheme(): ThemeId {
  const id = document.documentElement.dataset.theme
  return isThemeId(id) ? id : defaultTheme
}

function apply(id: ThemeId) {
  document.documentElement.dataset.theme = id
  for (const listener of listeners) listener()
}

/** The saved theme, or the default when none is saved or storage is blocked. */
export function savedTheme(): ThemeId {
  try {
    const id = localStorage.getItem(storageKey)
    return isThemeId(id) ? id : defaultTheme
  } catch {
    return defaultTheme
  }
}

/** Switches to a theme and saves it in this browser. */
export function setTheme(id: ThemeId) {
  apply(id)
  try {
    localStorage.setItem(storageKey, id)
  } catch {
    // Private mode or blocked storage: the theme still applies for this visit.
  }
}

/** Carries a choice from the old toggle over to its theme here. */
function migrateLegacy() {
  try {
    const old = localStorage.getItem(legacyKey)
    if (old === null) return
    localStorage.removeItem(legacyKey)
    // The toggle saved 'dark' on every visit, so only 'light' was a real choice.
    if (old === 'light' && localStorage.getItem(storageKey) === null) localStorage.setItem(storageKey, 'space-light')
  } catch {
    // Blocked storage: nothing was saved to carry over.
  }
}

/** Applies the saved theme and follows changes made in other tabs. Call once, before the first render. Returns a stop function. */
export function startTheme() {
  migrateLegacy()
  apply(savedTheme())
  const follow = (e: StorageEvent) => {
    if (e.key === storageKey || e.key === null) apply(savedTheme())
  }
  window.addEventListener('storage', follow)
  return () => window.removeEventListener('storage', follow)
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** The theme in use, and a setter that applies and saves another. */
export function useTheme(): [ThemeId, (id: ThemeId) => void] {
  return [useSyncExternalStore(subscribe, currentTheme), setTheme]
}
