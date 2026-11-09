import { useEffect, useLayoutEffect, useRef } from 'react'

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Open modals, newest last. Only the newest takes Esc and Tab: one over a drawer must not close the drawer too. */
const open: object[] = []

/**
 * Shared behavior for Drawer and Dialog: Esc closes the topmost, focus moves inside on
 * open and is kept there with Tab, focus returns on close, and the page
 * behind stops scrolling.
 */
export function useModal<T extends HTMLElement>(onClose: () => void) {
  const ref = useRef<T>(null)
  const close = useRef(onClose)
  useLayoutEffect(() => {
    close.current = onClose
  })

  useEffect(() => {
    const panel = ref.current
    const previous = document.activeElement as HTMLElement | null
    const first = panel?.querySelector<HTMLElement>(FOCUSABLE)
    ;(first ?? panel)?.focus()
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const self = {}
    open.push(self)

    const onKey = (e: KeyboardEvent) => {
      if (open[open.length - 1] !== self) return
      if (e.key === 'Escape') {
        e.stopPropagation()
        close.current()
        return
      }
      if (e.key !== 'Tab' || !panel) return
      const items = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)]
      if (items.length === 0) return
      const [head, tail] = [items[0], items[items.length - 1]]
      if (e.shiftKey && document.activeElement === head) {
        e.preventDefault()
        tail.focus()
      } else if (!e.shiftKey && document.activeElement === tail) {
        e.preventDefault()
        head.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      open.splice(open.indexOf(self), 1)
      document.body.style.overflow = overflow
      previous?.focus?.()
    }
  }, [])

  return ref
}
