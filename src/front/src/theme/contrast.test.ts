import { expect, it } from 'vitest'
import { themes } from './themes'

// WCAG contrast for every theme, on the pairs the components use. Text needs
// 4.5:1; control boundaries, focus rings and the signal border need 3:1.
// Translucent colours are composited over what sits behind them, as the
// browser does.

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
/** A Tailwind opacity modifier, e.g. bg-signal/10, over an opaque bg */
const tint = (c: Rgba, percent: number, bg: Rgba) => over([c[0], c[1], c[2], percent / 100], bg)

function luminance([r, g, b]: Rgba) {
  const channel = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function ratio(a: Rgba, b: Rgba) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

it.each(themes.map((t) => t.id))('%s meets WCAG AA on every pair the components use', (id) => {
  const vars = Object.fromEntries([...sheetById[id].matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]))
  const color = (name: string) => {
    const c = parse(vars[name] ?? '')
    if (!c) throw new Error(`--${name} in ${id}.css is not a hex or rgb() color`)
    return c
  }
  /** Every opaque ground text sits on. Stage tiles step from first to last, so both ends count. */
  const grounds: Record<string, Rgba> = Object.fromEntries(
    ['ground', 'rail', 'block', 'block-hover', 'tile-first', 'tile-last', 'paper'].map((n) => [n, color(n)]),
  )

  const failures: string[] = []
  const check = (fg: string, fgColor: Rgba, on: string, bg: Rgba, min: number) => {
    const r = ratio(fgColor, bg)
    if (r < min) failures.push(`${fg} on ${on}: ${r.toFixed(2)}, needs ${min}`)
  }

  for (const text of ['ink', 'ink-2']) for (const [name, bg] of Object.entries(grounds)) check(text, color(text), name, bg, 4.5)
  // Text on the ink fills: stage tabs, the primary action, the active nav block
  for (const fill of ['ink', 'ink-hover']) for (const text of ['on-ink', 'on-ink-2']) check(text, color(text), fill, color(fill), 4.5)
  check('on-accent', color('on-accent'), 'accent', color('accent'), 4.5)
  check('on-signal', color('on-signal'), 'signal', color('signal'), 4.5)
  check('on-approve', color('on-approve'), 'approve', color('approve'), 4.5)
  // Coloured words: links and live marks, what needs a person, what awaits approval, success
  for (const text of ['accent', 'signal-ink', 'approve-ink', 'good'])
    for (const name of ['ground', 'tile-first', 'tile-last', 'paper']) check(text, color(text), name, grounds[name], 4.5)
  check('signal-ink', color('signal-ink'), 'signal-wash', color('signal-wash'), 4.5)
  check('ink', color('ink'), 'signal-wash', color('signal-wash'), 4.5)
  check('ink', color('ink'), 'approve/20 over paper', tint(color('approve'), 20, color('paper')), 4.5)
  // Control boundaries and card edges, the focus ring, and the signal border, against what they sit on
  for (const edge of ['line', 'accent', 'signal'])
    for (const name of ['ground', 'tile-first', 'tile-last', 'paper']) check(edge, color(edge), name, grounds[name], 3)

  expect(failures).toEqual([])
})
