import type { CSSProperties } from 'react'
import { Icon } from '../../components/Icon'
import { RadioCard } from '../../components/RadioCard'
import { tileStep } from '../../components/tileStep'
import { settingsLabels } from '../../labels/labels'
import { useTheme } from '../../theme/theme'
import { themes, type ThemeId } from '../../theme/themes'
import { Section } from './Section'
import { sectionIds } from './sections'

/** The color theme for this browser. Picking one applies it at once; there's nothing to save. */
export function AppearanceSection() {
  const [theme, setTheme] = useTheme()
  return (
    <Section id={sectionIds.appearance} title={settingsLabels.appearance} intro={settingsLabels.appearanceHelp}>
      <fieldset className="m-0 min-w-0 border-0 p-0">
        <legend className="mb-2 px-0.5 text-[13px] font-semibold text-ink-2">{settingsLabels.theme}</legend>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-2">
          {themes.map((t) => (
            <RadioCard
              key={t.id}
              name="theme"
              value={t.id}
              checked={theme === t.id}
              onChange={() => setTheme(t.id)}
              label={
                <>
                  {t.name}
                  {/* The radio already says it's checked; this is for the eye, beside the accent edge. */}
                  {theme === t.id && (
                    <span
                      aria-hidden="true"
                      className="ml-auto inline-flex h-5 flex-none items-center gap-1 rounded-[2px] bg-accent px-1.5 text-[11px] font-bold text-on-accent"
                    >
                      <Icon name="check" size={12} strokeWidth={3} />
                      In use
                    </span>
                  )}
                </>
              }
              description={t.description}
              preview={<ThemePreview id={t.id} />}
            />
          ))}
        </div>
      </fieldset>
    </Section>
  )
}

/** The three stages the preview draws, each with its cards: a plain one, then what lights a stage */
const previewStages: { cards: ('plain' | 'live' | 'signal' | 'approve')[]; mark?: 'signal' | 'approve' }[] = [
  { cards: ['plain', 'plain', 'plain'] },
  { cards: ['live', 'signal'], mark: 'signal' },
  { cards: ['approve'], mark: 'approve' },
]

const cardEdge = {
  plain: 'border border-line',
  live: 'border border-line',
  signal: 'border border-signal',
  approve: 'border-2 border-approve',
}

/**
 * A miniature of the board in one theme: the rail with the open project in ink, the header's counters and New task,
 * then stepped stage tiles under ink tabs holding white cards. data-theme scopes every colour to it, so each card
 * shows its own theme whichever one is in use.
 */
function ThemePreview({ id }: { id: ThemeId }) {
  return (
    <span data-theme={id} aria-hidden="true" className="stars mb-1.5 flex h-[112px] overflow-hidden rounded-[3px] border border-rule bg-ground">
      <span className="flex w-[24%] max-w-11 flex-none flex-col gap-[3px] border-r border-rule bg-rail p-[5px]">
        <span className="mb-1 flex h-2.5 w-3.5 items-center gap-px rounded-[1px] bg-ink px-[2px]">
          <span className="h-1 flex-1 bg-on-ink" />
          <span className="h-1 flex-1 bg-approve" />
        </span>
        <span className="h-3.5 rounded-[2px] bg-ink" />
        <span className="-mt-[3px] flex h-5 flex-col justify-center gap-[3px] rounded-b-[2px] bg-paper px-[3px]">
          <span className="h-[3px] rounded-[1px] bg-block" />
          <span className="h-[3px] w-3/4 rounded-[1px] bg-ink-2" />
        </span>
        <span className="h-3.5 rounded-[2px] bg-block" />
        <span className="h-3.5 rounded-[2px] bg-block" />
      </span>

      <span className="flex min-w-0 flex-1 flex-col gap-[5px] p-[5px]">
        <span className="flex h-3 flex-none items-center gap-[3px]">
          <span className="h-full w-[38%] rounded-[1px] bg-ink" />
          <span className="ml-auto size-2.5 rounded-[1px] border border-ink bg-approve" />
          <span className="size-2.5 rounded-[1px] border border-ink bg-signal" />
          <span className="flex h-full w-5 items-center rounded-[1px] bg-ink pl-[2px]">
            <span className="size-2 rounded-[1px] bg-paper" />
          </span>
        </span>

        <span className="flex min-h-0 flex-1 gap-[3px]">
          {previewStages.map((stage, i) => (
            <span
              key={i}
              className="tile flex min-w-0 flex-1 flex-col gap-[3px] rounded-[2px] p-[2px]"
              style={{ '--tile-step': tileStep(i, previewStages.length) } as CSSProperties}
            >
              <span className="flex h-2.5 flex-none items-center justify-end rounded-[1px] bg-ink px-[2px]">
                {stage.mark && <span className={`size-1.5 rounded-[1px] ${stage.mark === 'signal' ? 'bg-signal' : 'bg-approve'}`} />}
              </span>
              <span className="flex flex-col gap-[3px] rounded-[1px] border border-tile-keyline p-[2px]">
                {stage.cards.map((card, j) => (
                  <span key={j} className={`flex h-[18px] flex-none flex-col justify-center gap-[3px] rounded-[1px] bg-paper px-[3px] ${cardEdge[card]}`}>
                    <span className="h-[2px] w-4/5 bg-ink-2" />
                    {card === 'live' && <span className="h-[3px] w-1/2 bg-accent" />}
                    {card === 'signal' && <span className="h-[3px] w-1/2 bg-signal" />}
                    {(card === 'plain' || card === 'approve') && <span className="h-[2px] w-1/2 bg-keyline" />}
                  </span>
                ))}
              </span>
            </span>
          ))}
        </span>
      </span>
    </span>
  )
}
