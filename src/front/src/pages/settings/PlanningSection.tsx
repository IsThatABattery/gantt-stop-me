import { useId, useState, type FormEvent } from 'react'
import { AgentEditor, NoProviders } from '../../components/AgentEditor'
import { Button } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { Toggle } from '../../components/Toggle'
import { UnsavedChangesGuard } from '../../components/UnsavedChangesGuard'
import type { PlanningSettings, Provider } from '../../data/api/types'
import { agentProblems, firstError, usePlaceApiError, usePlanning, useSavePlanning, type AgentErrors } from '../../data/views/agentSettings'
import { useProviders } from '../../data/views/settings'
import { llmAgentLabels, settingsLabels } from '../../labels/labels'
import { Section } from './Section'
import { sectionIds } from './sections'

type PlanningAgent = 'interviewer' | 'taskAgent'
const agents: PlanningAgent[] = ['interviewer', 'taskAgent']

const same = (a: PlanningSettings, b: PlanningSettings) => JSON.stringify(a) === JSON.stringify(b)

/** The user's planning agents, from /api/me/planning. It saves on its own and guards its own unsaved changes. */
export function PlanningSection() {
  const planning = usePlanning()
  const providers = useProviders()
  const error = planning.error ?? providers.error

  return (
    <Section id={sectionIds.agents} title={settingsLabels.agents} intro={settingsLabels.agentsHelp}>
      {error ? (
        <p role="status" className="m-0 text-critical">
          {error.message}
        </p>
      ) : !planning.data || !providers.data ? (
        <p role="status" className="m-0 text-text-muted">
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
        setFormError(at ? 'Fix the marked field and save again.' : err.message)
        focus(at)
      },
    })
  }

  return (
    <form onSubmit={submit} noValidate aria-label={settingsLabels.agents} className="flex flex-col gap-5">
      <div className="flex min-h-14 items-center gap-4">
        <span className="flex flex-1 flex-col gap-0.5">
          <span className="font-medium">{settingsLabels.review}</span>
          <span className="text-[13px] text-text-muted-strong">{settingsLabels.reviewHelp}</span>
        </span>
        <Toggle label={settingsLabels.review} checked={form.review} onChange={(review) => update({ ...form, review })} />
      </div>
      {providers.length === 0 ? (
        <NoProviders />
      ) : (
        agents.map((a) => (
          <AgentEditor
            key={a}
            id={`${id}-${a}`}
            {...llmAgentLabels[a]}
            providers={providers}
            value={form[a]}
            errors={errors[a]}
            onChange={(v) => update({ ...form, [a]: v })}
          />
        ))
      )}
      <UnsavedChangesGuard when={dirty && !save.isPending} />
      <div className="flex flex-wrap items-center justify-end gap-2.5">
        <div role="status" className="mr-auto text-[13px]">
          {saved && (
            <span className="inline-flex items-center gap-1.5 text-good">
              <Icon name="check" size={16} />
              Agent settings saved
            </span>
          )}
          {formError && <span className="text-critical">{formError}</span>}
          {!saved && !formError && dirty && <span className="text-text-muted">Unsaved changes</span>}
        </div>
        <Button type="submit" variant="primary" disabled={save.isPending}>
          {settingsLabels.saveAgents}
        </Button>
      </div>
    </form>
  )
}
