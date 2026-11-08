import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useState, type FormEvent } from 'react'
import { flushSync } from 'react-dom'
import { useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { AgentEditor, NoProviders } from '../../components/AgentEditor'
import { Button } from '../../components/controls'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { UnsavedChangesGuard } from '../../components/UnsavedChangesGuard'
import type { AgentSettings, Automation, Me, Permission, Provider } from '../../data/api/types'
import { errorCode, NotFoundError } from '../../data/errors'
import { keys } from '../../data/keys'
import { useMe } from '../../data/session'
import {
  agentProblems,
  firstError,
  noAgent,
  useAutomation,
  usePlaceApiError,
  useSaveAutomation,
  useTurnOffAutomation,
  type AgentErrors,
} from '../../data/views/agentSettings'
import { useProviders } from '../../data/views/settings'
import { actionLabels, labels, llmAgentLabels } from '../../labels/labels'

type AutomationAgent = 'decision' | 'chat'
type Agents = Record<AutomationAgent, AgentSettings>
const agentKeys: AutomationAgent[] = ['decision', 'chat']

/** A project's automation agents, from /api/projects/{id}/automation. */
export function AutomationPage() {
  const project = useProjectContext()
  const { projectId = '' } = useParams()
  const automation = useAutomation(projectId)
  const providers = useProviders()
  const me = useMe()
  // A failed refetch keeps the data, and the form with its edits.
  const error = [automation, providers, me].find((q) => !q.data && q.error)?.error

  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <Eyebrow>
          {labels.project} · {project.name}
        </Eyebrow>
        <h1 className="mt-1.5 mb-0 font-heading text-[26px] font-semibold">{labels.automation}</h1>
      </header>
      {!automation.data && automation.error instanceof NotFoundError ? (
        <StatusMessage
          title="Automation isn't available for this project"
          body="The server may have no test runner, or the project wasn't found."
        />
      ) : error ? (
        <StatusMessage title="Something went wrong" body={error.message} />
      ) : !automation.data || !providers.data || !me.data ? (
        <StatusMessage title={`Loading ${labels.automation.toLowerCase()}…`} />
      ) : (
        <AutomationForm
          key={projectId}
          projectId={projectId}
          permission={project.permission}
          automation={automation.data}
          providers={providers.data}
          me={me.data}
        />
      )}
    </div>
  )
}

/** The saved agents, but only on the caller's own providers: saving takes over with theirs. */
function fromData(a: Automation, providers: Provider[]): Agents {
  if (!a.enabled) return { decision: noAgent, chat: noAgent }
  const agent = (providerId: number, model: string, instructions: string) =>
    providers.some((p) => p.id === providerId) ? { providerId, model, instructions } : { ...noAgent, instructions }
  return {
    decision: agent(a.decisionProviderId, a.decisionModel, a.decisionInstructions),
    chat: agent(a.chatProviderId, a.chatModel, a.chatInstructions),
  }
}

const same = (a: Agents, b: Agents) => JSON.stringify(a) === JSON.stringify(b)

function AutomationForm({
  projectId,
  permission,
  automation,
  providers,
  me,
}: {
  projectId: string
  permission: Permission
  automation: Automation
  providers: Provider[]
  me: Me
}) {
  const [agents, setAgents] = useState(() => fromData(automation, providers))
  const [errors, setErrors] = useState<Partial<Record<AutomationAgent, AgentErrors>>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [confirmOff, setConfirmOff] = useState(false)
  // The permission comes from GitHub when the project loads; a 403 on save means it has dropped since.
  const [forbidden, setForbidden] = useState(false)
  const readOnly = forbidden || permission === 'read'
  const save = useSaveAutomation(projectId)
  const turnOff = useTurnOffAutomation(projectId)
  const placeApiError = usePlaceApiError()
  const client = useQueryClient()
  const id = useId()

  const takesOver = !readOnly && automation.enabled && automation.owner.id !== me.id
  // automation is refetched after a save or turn-off, so it's what the server has.
  const dirty = !readOnly && !same(agents, fromData(automation, providers))
  const busy = save.isPending || turnOff.isPending
  // The Turn off button is gone once it succeeds, so focus moves to the one that turns automation back on, or the status without one.
  useEffect(() => {
    if (turnOff.isSuccess) (document.getElementById(`${id}-save`) ?? document.getElementById(`${id}-state`))?.focus()
  }, [turnOff.isSuccess, id])

  const focus = (at: { agent: AutomationAgent; field: string } | null) => at && document.getElementById(`${id}-${at.agent}-${at.field}`)?.focus()

  const showError = (err: Error) => {
    if (errorCode(err) === 'forbidden') {
      // Read-only shows what's saved, not the edits; the project's permission is refetched.
      flushSync(() => {
        setForbidden(true)
        setConfirmOff(false)
        setAgents(fromData(automation, providers))
        setErrors({})
        setFormError(null)
      })
      document.getElementById(`${id}-readonly`)?.focus()
      client.invalidateQueries({ queryKey: keys.board(projectId) })
    } else setFormError(err.message)
  }

  const openOff = (open: boolean) => {
    flushSync(() => setConfirmOff(open))
    document.getElementById(`${id}-${open ? 'off-cancel' : 'off-open'}`)?.focus()
  }

  const update = (next: Agents) => {
    setAgents(next)
    setErrors({})
    setFormError(null)
    setSaved(false)
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setSaved(false)
    const problems = { decision: agentProblems(agents.decision, true), chat: agentProblems(agents.chat, true) }
    if (Object.values(problems).some((p) => Object.keys(p).length)) {
      setErrors(problems)
      setFormError('Fix the marked fields and save again.')
      focus(firstError(agentKeys, problems))
      return
    }
    const { decision: d, chat: c } = agents
    save.mutate(
      {
        decisionProviderId: d.providerId!,
        decisionModel: d.model!,
        decisionInstructions: d.instructions,
        chatProviderId: c.providerId!,
        chatModel: c.model!,
        chatInstructions: c.instructions,
      },
      {
        onSuccess: (a) => {
          setAgents(fromData(a, providers))
          setSaved(true)
        },
        onError: async (err) => {
          const at = await placeApiError(err, agents)
          if (at) {
            setErrors({ [at.agent]: { [at.field]: err.message } })
            setFormError('Fix the marked field and save again.')
            focus(at)
          } else showError(err)
        },
      },
    )
  }

  const off = () => {
    setSaved(false)
    setFormError(null)
    turnOff.mutate(undefined, {
      onSuccess: () => {
        setAgents({ decision: noAgent, chat: noAgent })
        setErrors({})
        setConfirmOff(false)
      },
      onError: showError,
    })
  }

  return (
    <form onSubmit={submit} noValidate aria-label={labels.automation} className="flex max-w-[820px] flex-col gap-5 rounded-[14px] border border-line bg-surface p-6">
      <div className="flex flex-col gap-1.5">
        <p id={`${id}-state`} tabIndex={-1} className="m-0 font-medium">
          {automation.enabled ? (automation.pausedReason ? 'Automation is on, but paused.' : 'Automation is on.') : 'Automation is off.'}
        </p>
        <p className="m-0 text-[13px] leading-normal text-text-muted-strong">
          {automation.enabled
            ? `Owner: ${automation.owner.login ?? 'none, they unlinked GitHub'}. Test runs and merges use the owner's GitHub account and providers.`
            : 'Choose both agents’ models to turn it on. You become its owner: test runs and merges use your GitHub account and providers.'}
        </p>
        {automation.enabled && (
          // The editors below show only the caller's own providers, so this is what runs for everyone else.
          <p className="m-0 text-[13px] leading-normal text-text-muted-strong">
            {llmAgentLabels.decision.name}: {automation.decisionModel}. {llmAgentLabels.chat.name}: {automation.chatModel}.
          </p>
        )}
        {automation.enabled && automation.pausedReason && (
          <p className="m-0 flex items-start gap-2 rounded-control border border-warning bg-warning/6 px-3 py-2 text-[13px]">
            <Icon name="alert" size={16} className="mt-0.5 flex-none text-warning" />
            <span>Paused: {automation.pausedReason} Saving unpauses it.</span>
          </p>
        )}
      </div>

      {providers.length === 0 && <NoProviders />}
      <fieldset disabled={readOnly} className="m-0 flex min-w-0 flex-col gap-5 border-0 p-0">
        {agentKeys.map((a) => (
          <AgentEditor
            key={a}
            id={`${id}-${a}`}
            {...llmAgentLabels[a]}
            required
            providers={providers}
            value={agents[a]}
            errors={errors[a]}
            onChange={(v) => update({ ...agents, [a]: v })}
          />
        ))}
      </fieldset>

      {takesOver && (
        <p id={`${id}-takeover`} className="m-0 flex items-start gap-2 rounded-control border border-warning bg-warning/6 px-3 py-2 text-[13px]">
          <Icon name="alert" size={16} className="mt-0.5 flex-none text-warning" />
          <span>
            Saving makes you the owner{automation.enabled && automation.owner.login ? ` instead of ${automation.owner.login}` : ''}. Test runs and merges then use
            your GitHub account and providers.
          </span>
        </p>
      )}

      {confirmOff && (
        <div role="alertdialog" aria-labelledby={`${id}-off`} className="flex flex-wrap items-center gap-3 rounded-card border border-warning bg-warning/6 p-4">
          <p id={`${id}-off`} className="m-0 flex-1 text-[13px]">
            Turn off automation? Queued and running test runs are cancelled.
          </p>
          <Button id={`${id}-off-cancel`} onClick={() => openOff(false)} disabled={busy}>
            {actionLabels.cancel}
          </Button>
          <Button onClick={off} disabled={busy} className="border-warning text-warning hover:border-warning hover:text-warning">
            Turn off
          </Button>
        </div>
      )}

      <UnsavedChangesGuard when={dirty && !busy} />

      <div className="flex flex-wrap items-center justify-end gap-2.5">
        <div role="status" className="mr-auto text-[13px]">
          {saved && (
            <span className="inline-flex items-center gap-1.5 text-good">
              <Icon name="check" size={16} />
              Automation saved
            </span>
          )}
          {readOnly ? (
            <span id={`${id}-readonly`} tabIndex={-1} className="text-text-muted-strong">
              You have read access only, so you can’t change automation here.
            </span>
          ) : (
            formError && <span className="text-critical">{formError}</span>
          )}
          {!saved && !formError && dirty && <span className="text-text-muted">Unsaved changes</span>}
        </div>
        {automation.enabled && !confirmOff && !readOnly && (
          <Button id={`${id}-off-open`} onClick={() => openOff(true)} disabled={busy}>
            Turn off automation
          </Button>
        )}
        {providers.length > 0 && !readOnly && (
          <Button id={`${id}-save`} type="submit" variant="primary" disabled={busy} aria-describedby={takesOver ? `${id}-takeover` : undefined}>
            {automation.enabled ? actionLabels.save : 'Turn on automation'}
          </Button>
        )}
      </div>
    </form>
  )
}
