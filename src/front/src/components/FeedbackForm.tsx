import { useId, useState, type FormEvent } from 'react'
import type { FeedbackOutcome } from '../data/types'
import { Eyebrow } from './Eyebrow'
import { RadioCard } from './RadioCard'
import { Button, Checkbox, Select, Textarea } from './controls'

export interface FeedbackValue {
  outcome: FeedbackOutcome
  reasons: string[]
  text: string
  /** Section the feedback points at; null means the whole item */
  sectionKey: string | null
}

interface FeedbackFormProps {
  heading: string
  intro: string
  /** The two outcomes, e.g. "Send back for revision" / "Discard this objective" */
  outcomes: Record<FeedbackOutcome, { label: string; description: string }>
  reasons: string[]
  /** Offer an "Applies to" picker over these sections (doc denials only) */
  sections?: { key: string; title: string }[]
  /** Label for the whole-item choice in the section picker */
  wholeLabel?: string
  placeholder?: string
  onCancel: () => void
  onSubmit: (value: FeedbackValue) => void
  submitting?: boolean
}

/**
 * Deny-with-feedback form shared by doc and payload denials (AGENT_BRIEF §8,
 * PlanDocDeny and PlanPayloadsDeny mockups). Free text is required.
 */
export function FeedbackForm({
  heading,
  intro,
  outcomes,
  reasons,
  sections,
  wholeLabel = 'Whole doc',
  placeholder,
  onCancel,
  onSubmit,
  submitting,
}: FeedbackFormProps) {
  const id = useId()
  const [outcome, setOutcome] = useState<FeedbackOutcome>('revise')
  const [picked, setPicked] = useState<string[]>([])
  const [sectionKey, setSectionKey] = useState('')
  const [text, setText] = useState('')
  const [error, setError] = useState<string | null>(null)

  const toggle = (r: string) => setPicked((p) => (p.includes(r) ? p.filter((x) => x !== r) : [...p, r]))

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!text.trim()) {
      setError('Write what should change. The astronaut needs it to revise.')
      return
    }
    setError(null)
    onSubmit({ outcome, reasons: picked, text: text.trim(), sectionKey: sectionKey || null })
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5">
      <div>
        <Eyebrow>Deny with feedback</Eyebrow>
        <h2 className="mt-2 mb-0 font-heading text-lg font-semibold">{heading}</h2>
        <p className="mt-2 mb-0 leading-normal text-text-muted-strong">{intro}</p>
      </div>

      <fieldset className="m-0 flex flex-col gap-3 border-0 p-0">
        <legend className="mb-3 p-0 text-[13px] text-text-soft">What should happen next?</legend>
        {(Object.keys(outcomes) as FeedbackOutcome[]).map((o) => (
          <RadioCard
            key={o}
            name={`${id}-outcome`}
            value={o}
            checked={outcome === o}
            onChange={() => setOutcome(o)}
            label={outcomes[o].label}
            description={outcomes[o].description}
          />
        ))}
      </fieldset>

      <fieldset className="m-0 border-0 p-0">
        <legend className="mb-1 p-0 text-[13px] text-text-soft">
          What is wrong? <span className="text-text-muted">(pick any)</span>
        </legend>
        <div className="flex flex-wrap gap-x-5">
          {reasons.map((r) => (
            <Checkbox key={r} label={r} checked={picked.includes(r)} onChange={() => toggle(r)} />
          ))}
        </div>
      </fieldset>

      {sections && (
        <div className="flex flex-col gap-2">
          <label htmlFor={`${id}-section`} className="text-[13px] text-text-soft">
            Applies to
          </label>
          <Select id={`${id}-section`} value={sectionKey} onChange={(e) => setSectionKey(e.target.value)}>
            <option value="">{wholeLabel}</option>
            {sections.map((s) => (
              <option key={s.key} value={s.key}>
                {s.title}
              </option>
            ))}
          </Select>
        </div>
      )}

      <div className="flex flex-col gap-2">
        <label htmlFor={`${id}-text`} className="text-[13px] text-text-soft">
          Feedback <span className="text-text-muted">(required)</span>
        </label>
        <Textarea
          id={`${id}-text`}
          rows={5}
          required
          aria-invalid={!!error}
          aria-describedby={`${id}-help`}
          placeholder={placeholder}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <span id={`${id}-help`} className={`text-[13px] ${error ? 'text-critical' : 'text-text-muted'}`}>
          {error ?? 'Be specific. The astronaut reads this word for word and shows what it changed.'}
        </span>
      </div>

      <div className="flex flex-wrap justify-end gap-3">
        <Button onClick={onCancel}>Cancel</Button>
        <Button type="submit" variant="primary" disabled={submitting}>
          Send feedback
        </Button>
      </div>
    </form>
  )
}
