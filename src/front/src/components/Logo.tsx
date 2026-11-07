import { labels } from '../labels/labels'

export function LogoMark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="var(--color-ion)" strokeWidth="1.6" aria-hidden="true">
      <g transform="rotate(-25 12 12)">
        <path d="M1.5 12A10.5 3.4 0 0 1 22.5 12" />
        <circle cx="12" cy="12" r="6" fill="#0D1830" />
        <path d="M1.5 12A10.5 3.4 0 0 0 22.5 12" />
      </g>
    </svg>
  )
}

export function Logo({ size = 28, textSize = 'text-lg' }: { size?: number; textSize?: string }) {
  return (
    <span className="flex items-center gap-2.5">
      <LogoMark size={size} />
      <span className={`font-heading font-semibold tracking-[.01em] text-text ${textSize}`}>{labels.appName}</span>
    </span>
  )
}
