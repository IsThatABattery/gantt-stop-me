import { expect, it } from 'vitest'
import { themes } from './themes'

// WCAG contrast for every theme, on the pairs the components use. Text needs
// 4.5:1, focus rings and icons 3:1. Translucent colors and tints are composited
// over what sits behind them, as the browser does.

const sheets = import.meta.glob<string>('../styles/themes/*.css', { query: '?raw', import: 'default', eager: true })
const sheetById = Object.fromEntries(Object.entries(sheets).map(([path, css]) => [path.match(/([^/]+)\.css$/)![1], css]))

type Rgba = [number, number, number, number]

function parse(value: string): Rgba | undefined {
  const hex = value.match(/^#([0-9a-f]{6})([0-9a-f]{2})?$/i)
  if (hex) {
    const n = (i: number) => parseInt(hex[1].slice(i, i + 2), 16)
    return [n(0), n(2), n(4), hex[2] ? parseInt(hex[2], 16) / 255 : 1]
  }
  const fn = value.match(/^rgba?\(([^)]+)\)$/)
  if (fn) {
    const [r, g, b, a = 1] = fn[1].split(/[\s,/]+/).filter(Boolean).map(Number)
    return [r, g, b, a]
  }
}

/** fg drawn over an opaque bg */
const over = (fg: Rgba, bg: Rgba): Rgba => [0, 1, 2].map((i) => fg[i] * fg[3] + bg[i] * (1 - fg[3])).concat(1) as Rgba
/** A Tailwind opacity modifier, e.g. bg-warning/20, over an opaque bg */
const tint = (c: Rgba, percent: number, bg: Rgba) => over([c[0], c[1], c[2], percent / 100], bg)

function luminance([r, g, b]: Rgba) {
  const channel = (v: number) => (v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function ratio(a: Rgba, b: Rgba) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

it.each(themes.map((t) => t.id))('%s meets WCAG AA on every pair the components use', (id) => {
  const vars = Object.fromEntries([...sheetById[id].matchAll(/--([a-z-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]))
  const color = (name: string) => {
    const c = parse(vars[name] ?? '')
    if (!c) throw new Error(`--${name} in ${id}.css is not a hex or rgb() color`)
    return c
  }
  const background = color('background')
  const surface = color('surface')
  /** Opaque backgrounds text sits on. The rail and header are translucent over the page. */
  const grounds: Record<string, Rgba> = {
    background,
    surface,
    'surface-sunken': color('surface-sunken'),
    'nav-active': color('nav-active'),
    rail: over(color('rail'), background),
    header: over(color('header'), background),
  }
  const accents = ['ion', 'ion-hover', 'astronaut', 'astronaut-text', 'good', 'warning', 'critical']
  const tones = ['ion', 'astronaut', 'good', 'warning', 'critical']

  const failures: string[] = []
  const check = (fg: string, fgColor: Rgba, on: string, bg: Rgba, min: number) => {
    const r = ratio(fgColor, bg)
    if (r < min) failures.push(`${fg} on ${on}: ${r.toFixed(2)}, needs ${min}`)
  }

  for (const text of ['text', 'text-soft', 'text-muted-strong', 'text-muted'])
    for (const [name, bg] of Object.entries(grounds)) check(text, color(text), name, bg, 4.5)
  for (const accent of accents) for (const name of ['background', 'surface', 'surface-sunken']) check(accent, color(accent), name, grounds[name], 4.5)
  // State text on its own tint: chips (/10), pressed counters and placeholders (/20), over a card or the header.
  for (const tone of tones)
    for (const percent of [10, 20])
      for (const name of ['surface', 'header']) check(tone, color(tone), `${tone}/${percent} over ${name}`, tint(color(tone), percent, grounds[name]), 4.5)
  // Icons on a hovered /25 tint
  for (const tone of tones) check(tone, color(tone), `${tone}/25 over surface`, tint(color(tone), 25, surface), 3)
  check('astronaut-text', color('astronaut-text'), 'astronaut/10 over surface', tint(color('astronaut'), 10, surface), 4.5)
  check('on-ion', color('on-ion'), 'ion', color('ion'), 4.5)
  check('on-ion', color('on-ion'), 'ion-hover', color('ion-hover'), 4.5)
  // The focus ring, and the border of a focused card
  for (const name of ['background', 'surface']) check('ion', color('ion'), name, grounds[name], 3)

  expect(failures).toEqual([])
})
