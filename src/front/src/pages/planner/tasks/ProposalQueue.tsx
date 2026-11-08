import { Chip } from '../../../components/Chip'
import type { ID, TaskProposal } from '../../../data/types'
import { labels, proposalStatusLabels } from '../../../labels/labels'
import { statusTone } from './proposalText'

interface ProposalQueueProps {
  proposals: TaskProposal[]
  selectedId: ID | undefined
  onSelect: (id: ID) => void
}

/** Every proposal in order. Each is a button, so the queue works by keyboard. */
export function ProposalQueue({ proposals, selectedId, onSelect }: ProposalQueueProps) {
  return (
    <nav aria-label={`${labels.task} proposals`} className="flex min-w-0 max-w-[400px] flex-[1_1_320px] flex-col">
      <ol className="m-0 flex list-none flex-col gap-2.5 p-0">
        {proposals.map((p) => {
          const selected = p.id === selectedId
          return (
            <li key={p.id}>
              <button
                type="button"
                aria-current={selected ? 'true' : undefined}
                onClick={() => onSelect(p.id)}
                className={`flex w-full cursor-pointer flex-col gap-2 rounded-card border p-3.5 text-left font-[inherit] text-text ${
                  selected ? 'border-ion bg-ion/8' : 'border-line bg-surface hover:border-line-strong'
                }`}
              >
                <span className="flex items-center justify-between gap-2">
                  <Chip tone={statusTone[p.status]}>{proposalStatusLabels[p.status]}</Chip>
                  <span className="font-mono text-xs text-text-muted">{p.estimate} pt</span>
                </span>
                <span className={`leading-[1.4] ${selected ? 'font-medium' : ''}`}>
                  <span className="sr-only">Proposal {p.order}: </span>
                  {p.title}
                </span>
              </button>
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
