// The short trail a card leaves when it changes column (AGENT_BRIEF §9).
// Drawn imperatively with the Web Animations API: one streak from the old
// spot to the new one that grows and fades in under half a second.

const DURATION_MS = 450

export function prefersReducedMotion() {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

export function drawTrail(from: DOMRect, to: DOMRect) {
  if (prefersReducedMotion()) return
  const el = document.createElement('div')
  if (typeof el.animate !== 'function') return

  const x1 = from.left + from.width / 2
  const y1 = from.top + from.height / 2
  const x2 = to.left + to.width / 2
  const y2 = to.top + to.height / 2
  const length = Math.hypot(x2 - x1, y2 - y1)
  if (length < 1) return
  const angle = Math.atan2(y2 - y1, x2 - x1)

  el.setAttribute('aria-hidden', 'true')
  Object.assign(el.style, {
    position: 'fixed',
    left: `${x1}px`,
    top: `${y1 - 2}px`,
    width: `${length}px`,
    height: '4px',
    borderRadius: '999px',
    pointerEvents: 'none',
    zIndex: '50',
    transformOrigin: '0 50%',
    background: 'linear-gradient(90deg, transparent, var(--ion))',
    boxShadow: '0 0 12px var(--ion)',
  })
  document.body.append(el)

  const rotate = `rotate(${angle}rad)`
  el.animate(
    [
      { transform: `${rotate} scaleX(0)`, opacity: 0.9 },
      { transform: `${rotate} scaleX(1)`, opacity: 0.7, offset: 0.55 },
      { transform: `${rotate} scaleX(1) translateX(8%)`, opacity: 0 },
    ],
    { duration: DURATION_MS, easing: 'ease-out' },
  ).onfinish = () => el.remove()
}
