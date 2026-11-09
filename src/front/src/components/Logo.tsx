import { labels } from '../labels/labels'

/**
 * The GanttStopMe mark: an ink square holding three stage blocks, the last one in approve yellow, where the work
 * stops for a person. It follows the theme: light blocks on dark ink in Light, the reverse in Dark.
 */
export function LogoMark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 28 28" aria-hidden="true" focusable="false" className="flex-none">
      <rect width="28" height="28" rx="3" fill="var(--ink)" />
      <rect x="4" y="11" width="6" height="6" fill="var(--on-ink)" />
      <rect x="11" y="11" width="6" height="6" fill="var(--on-ink)" />
      <rect x="18" y="11" width="6" height="6" fill="var(--approve)" />
    </svg>
  )
}

export function Logo({ size = 28, textSize = 'text-base' }: { size?: number; textSize?: string }) {
  return (
    <span className="flex items-center gap-2.5">
      <LogoMark size={size} />
      <span className={`font-bold tracking-[-0.01em] text-ink ${textSize}`}>{labels.appName}</span>
    </span>
  )
}
