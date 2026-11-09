import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useState, type FormEvent } from 'react'
import { flushSync } from 'react-dom'
import { useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { AgentEditor, NoProviders } from '../../components/AgentEditor'
import { Button } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { PageTitle } from '../../components/PageTitle'
import { InkTab, Tile } from '../../components/Tile'
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
import { Card, Confirm } from '../settings/Section'
import { Workflows } from './Workflows'

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
    <div className="flex flex-col gap-6 px-4 pt-5 pb-12 md:px-7">
      <header className="flex flex-col gap-2.5">
        <PageTitle>{labels.automation}</PageTitle>
        <p className="m-0 text-[15px] text-ink-2">
          <span className="font-semibold text-ink">
            {project.owner}/{project.name}
          </span>{' '}
          · Test runs check each {labels.task.toLowerCase()}'s linked pull request, then report back.
        </p>
      </header>
      <div className="flex max-w-[1160px] flex-col gap-6">
        {!automation.data && automation.error instanceof NotFoundError ? (
          <Notice
            title="Automation isn't available for this project"
            body="The server may have no test runner, or the project wasn't found."
          />
        ) : error ? (
          <Notice tone="signal" title="Something went wrong" body={error.message} />
        ) : !automation.data || !providers.data || !me.data ? (
          <p role="status" className="m-0 text-sm text-ink-2">
            Loading {labels.automation.toLowerCase()}…
          </p>
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
        {automation.data && <Workflows projectId={projectId} readOnly={project.permission === 'read'} />}
      </div>
    </div>
  )
}

/** In place of the form when there's none to show: a square mark, a heading and what to do. */
function Notice({ title, body, tone = 'neutral' }: { title: string; body: string; tone?: 'neutral' | 'signal' }) {
  return (
    <Card>
      <div className="flex items-start gap-3.5">
      <span
        aria-hidden="true"
        className={`grid size-10 flex-none place-items-center rounded-[2px] ${tone === 'signal' ? 'bg-signal text-on-signal' : 'border-[1.5px] border-ink text-ink'}`}
      >
        <Icon name={tone === 'signal' ? 'alert' : 'automation'} size={20} />
      </span>
      <div role="status" className="flex min-w-0 flex-col gap-1">
        <h2 className="m-0 text-base font-bold">{title}</h2>
        <p className="m-0 max-w-[60ch] text-[13px] leading-normal text-ink-2">{body}</p>
      </div>
      </div>
    </Card>
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

  // On, paused (it needs the owner, or someone taking over, to save again) or off: a mark with an icon and the words.
  const state = !automation.enabled ? 'off' : automation.pausedReason ? 'paused' : 'on'
  const mark = {
    on: { look: 'bg-ink text-on-ink', icon: 'automation' },
    paused: { look: 'bg-signal text-on-signal', icon: 'pause' },
    off: { look: 'border-[1.5px] border-ink-2 text-ink-2', icon: 'automation' },
  } as const

  return (
    <form onSubmit={submit} noValidate aria-label={labels.automation} className="flex flex-col gap-6">
      <Card gap="gap-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
          <span aria-hidden="true" className={`grid size-11 flex-none place-items-center rounded-[2px] ${mark[state].look}`}>
            <Icon name={mark[state].icon} size={22} />
          </span>
          <div className="flex min-w-[min(100%,16rem)] flex-1 flex-col gap-1">
            <p id={`${id}-state`} tabIndex={-1} className="m-0 text-base font-bold">
              {automation.enabled ? (automation.pausedReason ? 'Automation is on, but paused.' : 'Automation is on.') : 'Automation is off.'}
            </p>
            <p className="m-0 max-w-[80ch] text-[13px] leading-normal text-ink-2">
              {automation.enabled
                ? `Owner: ${automation.owner.login ?? 'none, they unlinked GitHub'}. Test runs and merges use the owner's GitHub account and providers.`
                : 'Choose both agents’ models to turn it on. You become its owner: test runs and merges use your GitHub account and providers.'}
            </p>
            {automation.enabled && (
              // The editors below show only the caller's own providers, so this is what runs for everyone else.
              <p className="m-0 text-[13px] leading-normal text-ink-2">
                {llmAgentLabels.decision.name}: {automation.decisionModel}. {llmAgentLabels.chat.name}: {automation.chatModel}.
              </p>
            )}
          </div>
          {automation.enabled && !confirmOff && !readOnly && (
            <Button id={`${id}-off-open`} onClick={() => openOff(true)} disabled={busy}>
              Turn off automation
            </Button>
          )}
        </div>
        {automation.enabled && automation.pausedReason && (
          <p className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] font-semibold text-signal-ink">
            <Icon name="alert" size={16} className="mt-px flex-none" />
            <span>Paused: {automation.pausedReason} Saving unpauses it.</span>
          </p>
        )}
        {confirmOff && (
          <Confirm id={`${id}-off`} question="Turn off automation? Queued and running test runs are cancelled.">
            <Button id={`${id}-off-cancel`} onClick={() => openOff(false)} disabled={busy}>
              {actionLabels.cancel}
            </Button>
            <Button variant="danger" onClick={off} disabled={busy} aria-busy={turnOff.isPending || undefined}>
              Turn off
            </Button>
          </Confirm>
        )}
      </Card>

      <Tile
        as="section"
        aria-labelledby={`${id}-agents`}
        frameClassName="gap-3"
        tab={
          <InkTab id={`${id}-agents`} count={agentKeys.length} countLabel="agents">
            Test agents
          </InkTab>
        }
      >
        <p className="m-0 max-w-[70ch] px-0.5 text-[13px] leading-[1.45] text-ink-2">
          Every test run uses both, on the owner's providers. Choose each one's provider and model.
        </p>
        {providers.length === 0 && (
          <Card>
            <NoProviders />
          </Card>
        )}
        <fieldset disabled={readOnly} className="m-0 grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,400px),1fr))] gap-2 border-0 p-0">
          {agentKeys.map((a) => (
            // The editor draws a rule above itself for stacking in one panel; as its own card it needs none.
            <Card key={a} frameClassName="[&>div]:border-t-0 [&>div]:pt-0">
              <AgentEditor
                id={`${id}-${a}`}
                {...llmAgentLabels[a]}
                required
                providers={providers}
                value={agents[a]}
                errors={errors[a]}
                onChange={(v) => update({ ...agents, [a]: v })}
              />
            </Card>
          ))}
        </fieldset>

        {takesOver && (
          <p id={`${id}-takeover`} className="m-0 flex items-start gap-2 rounded-[2px] bg-approve/25 px-3 py-2.5 text-[13px] text-ink">
            <Icon name="person" size={16} className="mt-px flex-none" />
            <span>
              Saving makes you the owner{automation.enabled && automation.owner.login ? ` instead of ${automation.owner.login}` : ''}. Test runs and merges then use
              your GitHub account and providers.
            </span>
          </p>
        )}

        <UnsavedChangesGuard when={dirty && !busy} />

        <div className="flex flex-wrap items-center justify-end gap-2.5 border-t border-tile-keyline pt-3">
          <div role="status" className="mr-auto px-0.5 text-[13px] font-semibold">
            {saved && (
              <span className="inline-flex items-center gap-1.5 text-ink">
                <Icon name="check" size={16} className="text-good" />
                Automation saved
              </span>
            )}
            {readOnly ? (
              <span id={`${id}-readonly`} tabIndex={-1} className="inline-flex items-center gap-1.5 text-ink-2">
                <Icon name="lock" size={16} className="flex-none" />
                You have read access only, so you can’t change automation here.
              </span>
            ) : (
              formError && (
                <span className="inline-flex items-center gap-1.5 text-signal-ink">
                  <Icon name="alert" size={16} className="flex-none" />
                  {formError}
                </span>
              )
            )}
            {!saved && !formError && dirty && <span className="text-ink-2">Unsaved changes</span>}
          </div>
          {providers.length > 0 && !readOnly && (
            <Button
              id={`${id}-save`}
              type="submit"
              variant="primary"
              disabled={busy}
              aria-busy={save.isPending || undefined}
              aria-describedby={takesOver ? `${id}-takeover` : undefined}
            >
              {automation.enabled ? actionLabels.save : 'Turn on automation'}
            </Button>
          )}
        </div>
      </Tile>
    </form>
  )
}
