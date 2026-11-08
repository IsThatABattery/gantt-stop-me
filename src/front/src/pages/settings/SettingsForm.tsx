import { useEffect, useId, useState, type ReactNode } from 'react'
import { useBlocker } from 'react-router'
import { AutonomyRings } from '../../components/Avatar'
import { Button, Field, Select, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { RadioCard } from '../../components/RadioCard'
import { Toggle } from '../../components/Toggle'
import type { Autonomy, User, WorkspaceSettings } from '../../data/types'
import { useSaveProfile, useSaveSettings } from '../../data/views/settings'
import { actionLabels, autonomyDescriptions, autonomyLabels, labels, settingsLabels } from '../../labels/labels'
import { PasswordForm } from './PasswordForm'
import { sectionIds } from './sections'

interface FormState {
  name: string
  email: string
  defaultAutonomy: Autonomy
  freeFlightGrantedBy: WorkspaceSettings['freeFlightGrantedBy']
  /** Kept as typed so a half-typed number can be shown and checked */
  agentTaskCap: string
  rules: Record<string, boolean>
  providerName: string
  apiKey: string
}

const PROVIDERS = [{ name: 'OpenAI', description: 'Default provider.' }]

function fromData(s: WorkspaceSettings, u: User): FormState {
  return {
    name: u.name,
    email: u.email,
    defaultAutonomy: s.defaultAutonomy,
    freeFlightGrantedBy: s.freeFlightGrantedBy,
    agentTaskCap: String(s.agentTaskCap),
    rules: Object.fromEntries(s.approvalRules.map((r) => [r.id, r.enabled])),
    providerName: s.provider.name,
    apiKey: '',
  }
}

const same = (a: FormState, b: FormState) => JSON.stringify(a) === JSON.stringify(b)

function capProblem(cap: string) {
  const n = Number(cap)
  return cap.trim() !== '' && Number.isInteger(n) && n >= 1 ? null : 'Use a whole number, 1 or more.'
}

export function SettingsForm({ settings, user }: { settings: WorkspaceSettings; user: User }) {
  const [baseline, setBaseline] = useState(() => fromData(settings, user))
  const [form, setForm] = useState(baseline)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showCapError, setShowCapError] = useState(false)
  const saveSettings = useSaveSettings()
  const saveProfile = useSaveProfile(user.id)
  const saving = saveSettings.isPending || saveProfile.isPending
  const dirty = !same(form, baseline)
  const id = useId()

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
    setSaved(false)
    setError(null)
  }

  // Free flight is only for the flight director when the workspace says so (AGENT_BRIEF §6).
  const canPickFreeFlight = user.role === 'lead' || baseline.freeFlightGrantedBy === 'any'
  const cap = capProblem(form.agentTaskCap)

  const save = async () => {
    if (cap) {
      setShowCapError(true)
      document.getElementById(`${id}-cap`)?.focus()
      return
    }
    setError(null)
    try {
      if (form.name !== baseline.name || form.email !== baseline.email) {
        await saveProfile.mutateAsync({ userId: user.id, name: form.name, email: form.email })
      }
      await saveSettings.mutateAsync({
        defaultAutonomy: form.defaultAutonomy,
        freeFlightGrantedBy: form.freeFlightGrantedBy,
        agentTaskCap: Number(form.agentTaskCap),
        rules: form.rules,
        providerName: form.providerName,
        apiKey: form.apiKey,
      })
      const next = { ...form, name: form.name.trim(), email: form.email.trim(), apiKey: '' }
      setBaseline(next)
      setForm(next)
      setSaved(true)
      setShowCapError(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Saving failed.')
    }
  }

  const cancel = () => {
    setForm(baseline)
    setSaved(false)
    setError(null)
    setShowCapError(false)
  }

  // Warn before leaving the page with unsaved changes. Section links only change the hash, so they pass.
  const blocker = useBlocker(({ currentLocation, nextLocation }) => dirty && !saving && currentLocation.pathname !== nextLocation.pathname)
  useEffect(() => {
    if (!dirty) return
    const onUnload = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', onUnload)
    return () => window.removeEventListener('beforeunload', onUnload)
  }, [dirty])

  return (
    <div className="flex min-w-0 flex-[999_1_520px] flex-col gap-5 lg:max-w-[820px]">
      <Section id={sectionIds.profile} title={settingsLabels.profile}>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-4">
          <Field id={`${id}-name`} label="Name">
            <TextInput id={`${id}-name`} autoComplete="name" value={form.name} onChange={(e) => set('name', e.target.value)} />
          </Field>
          <Field id={`${id}-email`} label="Email">
            <TextInput id={`${id}-email`} type="email" autoComplete="email" value={form.email} onChange={(e) => set('email', e.target.value)} />
          </Field>
        </div>
        <PasswordForm />
      </Section>

      <Section id={sectionIds.autonomy} title={settingsLabels.defaultAutonomy} intro={settingsLabels.defaultAutonomyHelp}>
        <fieldset className="m-0 grid grid-cols-[repeat(auto-fit,minmax(170px,1fr))] gap-3 border-0 p-0">
          <legend className="sr-only">{settingsLabels.defaultAutonomy}</legend>
          {([0, 1, 2] as Autonomy[]).map((level) => (
            <RadioCard
              key={level}
              tone="astronaut"
              name={`${id}-autonomy`}
              value={level}
              checked={form.defaultAutonomy === level}
              disabled={level === 2 && !canPickFreeFlight}
              onChange={() => set('defaultAutonomy', level)}
              label={
                <>
                  {autonomyLabels[level]}
                  <AutonomyRings level={level} />
                </>
              }
              description={level === 2 && !canPickFreeFlight ? `Only the ${labels.lead.toLowerCase()} can choose this.` : autonomyDescriptions[level]}
            />
          ))}
        </fieldset>
        <div className="flex flex-wrap gap-4">
          <div className="flex-[1_1_240px]">
            <Field id={`${id}-grant`} label={settingsLabels.freeFlightGrantedBy}>
              <Select
                id={`${id}-grant`}
                value={form.freeFlightGrantedBy}
                disabled={user.role !== 'lead'}
                onChange={(e) => set('freeFlightGrantedBy', e.target.value as FormState['freeFlightGrantedBy'])}
              >
                <option value="lead">{settingsLabels.grantLead}</option>
                <option value="any">{settingsLabels.grantAny}</option>
              </Select>
            </Field>
          </div>
          <div className="flex-[1_1_240px]">
            <Field id={`${id}-cap`} label={settingsLabels.agentTaskCap}>
              <TextInput
                id={`${id}-cap`}
                type="number"
                inputMode="numeric"
                min={1}
                step={1}
                value={form.agentTaskCap}
                aria-invalid={showCapError && !!cap}
                aria-describedby={showCapError && cap ? `${id}-cap-error` : undefined}
                onChange={(e) => set('agentTaskCap', e.target.value)}
              />
              {showCapError && cap && (
                <span id={`${id}-cap-error`} className="text-xs text-critical">
                  {cap}
                </span>
              )}
            </Field>
          </div>
        </div>
      </Section>

      <Section title={settingsLabels.approvalRules}>
        <ul className="m-0 list-none p-0">
          {settings.approvalRules.map((r) => (
            <li key={r.id} className="flex min-h-14 items-center gap-4 border-t border-line-soft py-2.5">
              <span className="flex flex-1 flex-col gap-0.5">
                <span className="font-medium">{r.title}</span>
                <span className="text-[13px] text-text-muted-strong">{r.description}</span>
              </span>
              <Toggle label={r.title} checked={form.rules[r.id] ?? false} onChange={(on) => set('rules', { ...form.rules, [r.id]: on })} />
            </li>
          ))}
        </ul>
      </Section>

      <Section id={sectionIds.roles} title={settingsLabels.roles}>
        <NotBuiltYet />
      </Section>

      <Section id={sectionIds.notifications} title={settingsLabels.notifications}>
        <NotBuiltYet />
      </Section>

      <Section id={sectionIds.integrations} title={settingsLabels.integrations} intro={settingsLabels.integrationsHelp}>
        <fieldset className="m-0 grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-3 border-0 p-0">
          <legend className="sr-only">AI provider</legend>
          {PROVIDERS.map((p) => (
            <RadioCard
              key={p.name}
              name={`${id}-provider`}
              value={p.name}
              checked={form.providerName === p.name}
              onChange={() => set('providerName', p.name)}
              label={p.name}
              description={p.description}
            />
          ))}
        </fieldset>
        <Field id={`${id}-key`} label={settingsLabels.apiKey}>
          {settings.provider.keyLast4 && (
            <span id={`${id}-key-saved`} className="inline-flex items-center gap-2 text-[13px] text-text-muted-strong">
              <Icon name="lock" size={14} />
              Saved key: <span className="font-mono text-text">•••• {settings.provider.keyLast4}</span>
            </span>
          )}
          <TextInput
            id={`${id}-key`}
            type="password"
            autoComplete="off"
            spellCheck={false}
            placeholder={settings.provider.keyLast4 ? 'Paste a new key to replace it' : `Paste your ${form.providerName} API key`}
            aria-describedby={settings.provider.keyLast4 ? `${id}-key-saved` : undefined}
            value={form.apiKey}
            onChange={(e) => set('apiKey', e.target.value)}
          />
        </Field>
      </Section>

      {blocker.state === 'blocked' && (
        <div role="alertdialog" aria-labelledby={`${id}-leave`} className="flex flex-wrap items-center gap-3 rounded-card border border-warning bg-warning/6 p-4">
          <p id={`${id}-leave`} className="m-0 flex-1 text-[13px]">
            You have unsaved changes. Leave without saving?
          </p>
          <Button onClick={() => blocker.reset()}>Stay</Button>
          <Button onClick={() => blocker.proceed()} className="border-warning text-warning hover:border-warning hover:text-warning">
            Leave without saving
          </Button>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-end gap-2.5">
        <div role="status" className="mr-auto text-[13px]">
          {saved && (
            <span className="inline-flex items-center gap-1.5 text-good">
              <Icon name="check" size={16} />
              Settings saved
            </span>
          )}
          {error && <span className="text-critical">{error}</span>}
          {!saved && !error && dirty && <span className="text-text-muted">Unsaved changes</span>}
        </div>
        <Button onClick={cancel} disabled={saving}>
          {actionLabels.cancel}
        </Button>
        <Button variant="primary" onClick={save} disabled={saving} className="px-[22px]">
          {actionLabels.save}
        </Button>
      </div>
    </div>
  )
}

function Section({ id, title, intro, children }: { id?: string; title: string; intro?: string; children: ReactNode }) {
  const headingId = useId()
  return (
    <section id={id} aria-labelledby={headingId} className="flex scroll-mt-6 flex-col gap-[18px] rounded-[14px] border border-line bg-surface p-6">
      <div>
        <h2 id={headingId} className="m-0 font-heading text-lg font-semibold">
          {title}
        </h2>
        {intro && <p className="m-0 mt-1.5 leading-normal text-text-muted-strong">{intro}</p>}
      </div>
      {children}
    </section>
  )
}

function NotBuiltYet() {
  return (
    <p className="m-0 rounded-control border border-dashed border-line-strong px-4 py-3 text-[13px] text-text-muted-strong">
      Not built yet. This section needs a design first.
    </p>
  )
}
