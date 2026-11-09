// Every color theme, in the order Settings lists them. Each one's colors are in
// src/styles/themes/<id>.css, imported by src/styles/index.css.
export const themes = [
  { id: 'space-dark', name: 'Space dark', description: 'The navy mission-control console over a starfield.' },
  { id: 'space-light', name: 'Space light', description: 'The space console on daylight surfaces.' },
  { id: 'dark', name: 'Dark', description: 'Neutral grays with a blue accent.' },
  { id: 'light', name: 'Light', description: 'White and light grays with a blue accent.' },
] as const

export type ThemeId = (typeof themes)[number]['id']

/** Used when none is saved. Its stylesheet also matches the bare root, so it shows before any script runs. */
export const defaultTheme: ThemeId = 'space-dark'

export function isThemeId(value: unknown): value is ThemeId {
  return themes.some((t) => t.id === value)
}
