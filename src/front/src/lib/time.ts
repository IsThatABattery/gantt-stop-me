const MIN = 60 * 1000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

const pad = (n: number) => String(n).padStart(2, '0')

/** "3d 06h", or "05h 12m" inside the last day. Never negative. */
export function countdown(endIso: string, now = Date.now()): string {
  const ms = Math.max(0, new Date(endIso).getTime() - now)
  const d = Math.floor(ms / DAY)
  const h = Math.floor((ms % DAY) / HOUR)
  const m = Math.floor((ms % HOUR) / MIN)
  return d > 0 ? `${d}d ${pad(h)}h` : `${pad(h)}h ${pad(m)}m`
}

/** True in the last 24 hours, when the countdown turns amber. */
export const isFinalDay = (endIso: string, now = Date.now()) => new Date(endIso).getTime() - now < DAY

/** "12m ago", "3h ago", "2d ago" */
export function timeAgo(iso: string, now = Date.now()): string {
  const ms = Math.max(0, now - new Date(iso).getTime())
  if (ms < HOUR) return `${Math.max(1, Math.floor(ms / MIN))}m ago`
  if (ms < DAY) return `${Math.floor(ms / HOUR)}h ago`
  return `${Math.floor(ms / DAY)}d ago`
}

/** "Nov 16" */
export const shortDate = (iso: string) =>
  new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
