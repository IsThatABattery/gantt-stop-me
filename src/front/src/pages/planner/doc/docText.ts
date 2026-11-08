import type { DocSection } from '../../../data/types'

/** Anchor id for a doc section, so comments can link to it. */
export const sectionAnchor = (key: string) => `doc-${key}`

/** A section body as editable text: lists become one item per line. */
export const toText = (s: DocSection) => (Array.isArray(s.body) ? s.body.join('\n') : s.body)

/** Edited text back into a section body, keeping lists as lists. */
export function fromText(s: DocSection, text: string): DocSection {
  const body = Array.isArray(s.body)
    ? text
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
    : text.trim()
  return { ...s, body }
}
