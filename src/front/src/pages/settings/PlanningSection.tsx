import { useId, useState, type FormEvent } from 'react'
import { AgentEditor, NoProviders } from '../../components/AgentEditor'
import { Button } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { Toggle } from '../../components/Toggle'
import { UnsavedChangesGuard } from '../../components/UnsavedChangesGuard'
import type { PlanningSettings, Provider } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { agentProblems, firstError, usePlaceApiError, usePlanning, useSavePlanning, type AgentErrors } from '../../data/views/agentSettings'
import { useProviders } from '../../data/views/settings'
import { labels, llmAgentLabels, settingsLabels } from '../../labels/labels'
import { Card, Section } from './Section'
import { sectionIds } from './sections'

type PlanningAgent = 'interviewer' | 'taskAgent'
const agents: PlanningAgent[] = ['interviewer', 'taskAgent']

const same = (a: PlanningSettings, b: PlanningSettings) => JSON.stringify(a) === JSON.stringify(b)

/** The user's planning agents, from /api/me/planning. It saves on its own and guards its own unsaved changes. */
export function PlanningSection() {
  const planning = usePlanning()
  const providers = useProviders()
  // A failed refetch keeps the data, and the form with its edits.
  const error = (!planning.data && planning.error) || (!providers.data && providers.error)

  return (
    <Section id={sectionIds.agents} title={settingsLabels.agents} intro={settingsLabels.agentsHelp}>
      {error ? (
        <p role="status" className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] font-semibold text-signal-ink">
          <Icon name="alert" size={16} className="mt-px flex-none" />
          <span>Couldn't load the agents. {error.message}</span>
        </p>
      ) : !planning.data || !providers.data ? (
        <p role="status" className="m-0 px-0.5 text-[13px] text-ink-2">
          Loading agents…
        </p>
      ) : (
        <PlanningForm initial={planning.data} providers={providers.data} />
      )}
    </Section>
  )
}

function PlanningForm({ initial, providers }: { initial: PlanningSettings; providers: Provider[] }) {
  const [form, setForm] = useState(initial)
  const [errors, setErrors] = useState<Partial<Record<PlanningAgent, AgentErrors>>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const save = useSavePlanning()
  const placeApiError = usePlaceApiError()
  const id = useId()
  // initial is refetched after a save, so it's what the server has.
  const dirty = !same(form, initial)

  const focus = (at: { agent: PlanningAgent; field: string } | null) => at && document.getElementById(`${id}-${at.agent}-${at.field}`)?.focus()

  const update = (next: PlanningSettings) => {
    setForm(next)
    setErrors({})
    setFormError(null)
    setSaved(false)
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const problems = { interviewer: agentProblems(form.interviewer), taskAgent: agentProblems(form.taskAgent) }
    if (Object.values(problems).some((p) => Object.keys(p).length)) {
      setErrors(problems)
      setFormError('Fix the marked field and save again.')
      focus(firstError(agents, problems))
      return
    }
    save.mutate(form, {
      onSuccess: (s) => {
        setForm(s)
        setSaved(true)
      },
      onError: async (err) => {
        const at = await placeApiError(err, { interviewer: form.interviewer, taskAgent: form.taskAgent })
        if (at) setErrors({ [at.agent]: { [at.field]: err.message } })
        // A 401 has the session dialog, and the edits stay to save once signed in again.
        if (errorCode(err) !== 'unauthorized') setFormError(at ? 'Fix the marked field and save again.' : err.message)
        focus(at)
      },
    })
  }

  return (
    <form onSubmit={submit} noValidate aria-label={settingsLabels.agents} className="flex flex-col gap-3">
      <Card frameClassName="py-3">
        <div className="flex items-center gap-4">
          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="font-bold">{settingsLabels.review}</span>
            <span className="text-[13px] text-ink-2">{settingsLabels.reviewHelp}</span>
          </span>
          <Toggle label={settingsLabels.review} checked={form.review} onChange={(review) => update({ ...form, review })} />
        </div>
      </Card>
      {providers.length === 0 && (
        <Card>
          <NoProviders />
        </Card>
      )}
      {agents.map((a) => (
        // The editor draws a rule above itself for stacking in one panel; as its own card it needs none.
        <Card key={a} frameClassName="[&>div]:border-t-0 [&>div]:pt-0">
          <AgentEditor
            id={`${id}-${a}`}
            {...llmAgentLabels[a]}
            deep
            providers={providers}
            value={form[a]}
            errors={errors[a]}
            onChange={(v) => update({ ...form, [a]: v })}
          />
        </Card>
      ))}
      <p className="m-0 px-0.5 text-[13px] leading-normal text-ink-2">
        The test agents, {llmAgentLabels.decision.name.toLowerCase()} and {llmAgentLabels.chat.name.toLowerCase()}, are set per {labels.project.toLowerCase()} on its {labels.automation} page.
      </p>
      <UnsavedChangesGuard when={dirty && !save.isPending} />
      <div className="flex flex-wrap items-center justify-end gap-2.5 border-t border-rule pt-3">
        <div role="status" className="mr-auto px-0.5 text-[13px] font-semibold">
          {saved && (
            <span className="inline-flex items-center gap-1.5 text-ink">
              <Icon name="check" size={16} className="text-good" />
              Agent settings saved
            </span>
          )}
          {formError && (
            <span className="inline-flex items-center gap-1.5 text-signal-ink">
              <Icon name="alert" size={16} />
              {formError}
            </span>
          )}
          {!saved && !formError && dirty && <span className="text-ink-2">Unsaved changes</span>}
        </div>
        <Button type="submit" variant="primary" busy={save.isPending}>
          {settingsLabels.saveAgents}
        </Button>
      </div>
    </form>
  )
}
