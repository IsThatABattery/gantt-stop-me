import { labels } from '../labels/labels'

/** A person: their initials on a 26px square ink-2 block with a 2px radius. */
export function UserAvatar({ initials }: { initials: string }) {
  return (
    <span
      role="img"
      aria-label={`${labels.member} ${initials}`}
      className="inline-flex size-[26px] flex-none items-center justify-center rounded-[2px] bg-ink-2 text-[11px] leading-none font-bold text-on-ink"
    >
      {initials}
    </span>
  )
}
