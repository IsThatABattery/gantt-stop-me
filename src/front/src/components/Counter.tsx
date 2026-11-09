import { Icon } from './Icon'

/** The count box's fill per tone. approve and signal ask for a person; live is what is running now. */
const fills = {
  approve: 'bg-approve text-on-approve',
  signal: 'bg-signal text-on-signal',
  live: 'bg-accent text-on-accent',
}

/** Older names, kept so call sites compile */
const aliases = { warning: 'approve', critical: 'signal', astronaut: 'live', ion: 'live' } as const

export type CounterTone = keyof typeof fills | keyof typeof aliases

interface CounterProps {
  tone: CounterTone
  count: number
  label: string
  onClick?: () => void
  /** When set, the counter is a toggle (a board filter): aria-pressed, an ink fill and a clear mark while on */
  pressed?: boolean
}

/**
 * A count block plus its plain word, 52px tall with a 1.5px ink edge, as in the board header. A zero count goes
 * hollow so the filled ones stand out. Pressed, the whole counter fills ink.
 */
export function Counter({ tone, count, label, onClick, pressed }: CounterProps) {
  const fill = fills[tone in aliases ? aliases[tone as keyof typeof aliases] : (tone as keyof typeof fills)]
  const empty = count === 0
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={pressed}
      className={`inline-flex min-h-[52px] cursor-pointer items-center gap-2.5 rounded-control border-[1.5px] border-ink py-1.5 pr-3.5 pl-[7px] font-[inherit] text-sm font-semibold whitespace-nowrap transition-colors duration-150 ease-out-quint ${
        pressed ? 'bg-ink text-on-ink hover:bg-ink-hover' : 'bg-paper text-ink hover:bg-ink/8 active:bg-ink/15'
      }`}
    >
      <span
        className={`grid h-9 min-w-9 place-items-center rounded-[2px] border-[1.5px] px-1.5 text-[17px] font-bold ${
          empty ? (pressed ? 'border-on-ink-2 text-on-ink-2' : 'border-ink-2 text-ink-2') : `${fill} ${pressed ? 'border-on-ink' : 'border-ink'}`
        }`}
      >
        {count}
      </span>
      {label}
      {pressed && <Icon name="close" size={14} className="-mr-0.5 ml-0.5" />}
    </button>
  )
}
