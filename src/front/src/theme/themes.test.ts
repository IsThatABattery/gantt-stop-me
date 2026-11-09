import { describe, expect, it } from 'vitest'
import indexHtml from '../../index.html?raw'
import indexCss from '../styles/index.css?raw'
import { storageKey } from './theme'
import { defaultTheme, themes } from './themes'

// The theme contract, checked from the stylesheets themselves, so a new theme
// that misses a variable, or a variable only some themes set, fails here.

const sheets = import.meta.glob<string>('../styles/themes/*.css', { query: '?raw', import: 'default', eager: true })
const sheetById = Object.fromEntries(Object.entries(sheets).map(([path, css]) => [path.match(/([^/]+)\.css$/)![1], css]))
const source = import.meta.glob<string>(['../**/*.{ts,tsx}', '!../**/*.test.{ts,tsx}'], { query: '?raw', import: 'default', eager: true })

/** Custom properties a stylesheet sets */
const declared = (css: string) => [...new Set([...css.matchAll(/(?:^|[{;\s])--([a-z0-9-]+)\s*:/g)].map((m) => m[1]))].sort()
/** Custom properties a file reads with var() */
const read = (text: string) => [...new Set([...text.matchAll(/var\(--([a-z0-9-]+)/g)].map((m) => m[1]))]
/** Tailwind colors mapped onto theme variables. Code reads the theme variable instead: when Tailwind emits one of these
 *  at all, it's on :root, where var() resolves once, so a data-theme lower down never reaches it. */
const inlineVars = declared(indexCss.match(/@theme inline\s*\{([^}]*)\}/)![1])
/** Tailwind's own theme variables, from the other @theme block in index.css */
const tailwindVars = declared(indexCss).filter((v) => !inlineVars.includes(v))

const variables = declared(sheetById[defaultTheme])
const ids = themes.map((t) => t.id)

describe('theme stylesheets', () => {
  it('has one per theme, each imported by index.css', () => {
    expect(Object.keys(sheetById).sort()).toEqual([...ids].sort())
    const imported = [...indexCss.matchAll(/@import '\.\/themes\/([a-z0-9-]+)\.css'/g)].map((m) => m[1])
    expect([...imported].sort()).toEqual([...ids].sort())
  })

  it.each(ids)('%s sets every theme variable and its color scheme, scoped to its data-theme', (id) => {
    const css = sheetById[id]
    expect(declared(css)).toEqual(variables)
    expect(css).toMatch(/color-scheme:\s*(dark|light);/)
    // Any element, not just the root, so data-theme themes a subtree (the previews in Settings).
    // The default also matches a bare root, at no specificity, so it shows before any script runs.
    const selector = css.replace(/\/\*[\s\S]*?\*\//g, '').split('{')[0].replace(/\s+/g, ' ').trim()
    expect(selector).toBe(id === defaultTheme ? `:where(:root), [data-theme='${id}']` : `[data-theme='${id}']`)
  })

  it('gives every theme variable a use, and reads none that no theme sets', () => {
    // --tile-step is set on each stage tile, by its position, not by a theme
    const readByCss = read(indexCss).filter((v) => !tailwindVars.includes(v) && v !== 'tile-step')
    expect([...readByCss].sort()).toEqual(variables)
  })
})

describe('code', () => {
  it('reads only theme variables, or the ones index.css gives Tailwind', () => {
    const known = new Set([...variables, ...tailwindVars])
    const unknown = Object.entries(source).flatMap(([path, text]) => read(text).filter((v) => !known.has(v)).map((v) => `var(--${v}) in ${path}`))
    expect(Object.keys(source).length).toBeGreaterThan(50)
    expect(unknown).toEqual([])
  })

  it('colours classes only from the contract', () => {
    // The tokens the process-flow overhaul replaced, and Tailwind's own palette, which no theme repaints
    const retired = ['background', 'surface', 'surface-sunken', 'header', 'nav-active', 'line-soft', 'line-strong', 'text', 'text-soft', 'text-muted', 'text-muted-strong', 'ion', 'ion-hover', 'on-ion', 'astronaut', 'astronaut-text', 'warning', 'critical']
    const palette = '(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\\d+'
    const utility = '(?:bg|text|border(?:-[trblxy])?|ring|outline|accent|fill|stroke|shadow|divide|placeholder|caret|from|to|via|decoration)'
    const pattern = new RegExp(`(?<![\\w-])${utility}-(?:${retired.join('|')}|${palette})(?:/\\d+)?(?![\\w-])`, 'g')
    const found = Object.entries(source).flatMap(([path, text]) => [...text.matchAll(pattern)].map((m) => `${m[0]} in ${path}`))
    expect(found).toEqual([])
  })

  it('applies the saved theme in index.html before first paint, from the same key', () => {
    expect(indexHtml).toContain(`localStorage.getItem('${storageKey}')`)
  })
})
