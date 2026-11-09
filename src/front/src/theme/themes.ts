// Every color theme, in the order Settings lists them. Each one's colors are in
// src/styles/themes/<id>.css, imported by src/styles/index.css.
export const themes = [
  { id: 'light', name: 'Light', description: 'Stepped gray stages under ink tabs, on white cards.' },
  { id: 'dark', name: 'Dark', description: 'The same board inverted: light tabs over charcoal.' },
  { id: 'space-light', name: 'Space light', description: 'Navy-tinted daylight, with teal for what is live.' },
  { id: 'space-dark', name: 'Space dark', description: 'The navy console, cyan for what is live, and stars.' },
] as const

export type ThemeId = (typeof themes)[number]['id']

/** Used when none is saved. Its stylesheet also matches the bare root, so it shows before any script runs. */
export const defaultTheme: ThemeId = 'light'

export function isThemeId(value: unknown): value is ThemeId {
  return themes.some((t) => t.id === value)
}
