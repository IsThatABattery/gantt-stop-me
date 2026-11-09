// The short trail a card leaves when it changes column: one flat line in the accent (ink in Light and Dark), with
// square ends like the rest of the sheet, drawn from the old spot to the new one and gone in under half a second.
// Drawn imperatively with the Web Animations API. Nothing is drawn when the user asks for reduced motion.

const DURATION_MS = 420
const WEIGHT_PX = 2

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
    top: `${y1 - WEIGHT_PX / 2}px`,
    width: `${length}px`,
    height: `${WEIGHT_PX}px`,
    pointerEvents: 'none',
    zIndex: '50',
    transformOrigin: '0 50%',
    background: 'var(--accent)',
  })
  document.body.append(el)

  // The line draws out to the new spot, then withdraws into it.
  const rotate = `rotate(${angle}rad)`
  el.animate(
    [
      { transform: `${rotate} translateX(0) scaleX(0)` },
      { transform: `${rotate} translateX(0) scaleX(1)`, offset: 0.5 },
      { transform: `${rotate} translateX(${length}px) scaleX(0)` },
    ],
    { duration: DURATION_MS, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' },
  ).onfinish = () => el.remove()
}
