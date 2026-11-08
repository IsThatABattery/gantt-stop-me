import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { renderApp } from '../../test/render'

/** The create dialog, once its form has loaded. */
async function openDialog(name: string) {
  const dialog = await screen.findByRole('dialog', { name })
  await within(dialog).findByRole('button', { name: /^Create/ })
  return dialog
}

describe('create dialog', () => {
  it('creates a payload, opens its edit page, and puts it in the Hangar', async () => {
    const { router } = renderApp('/projects/orbital-nav/new/task')
    const dialog = await openDialog('New payload')

    await userEvent.type(within(dialog).getByLabelText('Title'), 'Retry cap for reconnects')
    await userEvent.selectOptions(within(dialog).getByLabelText('Objective'), 'Realtime sync')
    await userEvent.selectOptions(within(dialog).getByLabelText('Assignee'), 'Astronaut-1 · Astronaut')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create payload' }))

    await screen.findByRole('heading', { name: 'Edit payload' })
    const task = db.tasks.find((t) => t.title === 'Retry cap for reconnects')!
    expect(task).toMatchObject({ key: 'PAY-63', column: 'backlog', status: 'queued', featureId: 'f-realtime', epicId: 'e-comms' })
    expect(task.assignee).toEqual({ kind: 'agent', id: 'a-1' })
    expect(db.activityEvents.some((e) => e.verb === 'created' && e.taskId === task.id)).toBe(true)
    expect(router.state.location.pathname).toBe(`/projects/orbital-nav/tasks/${task.id}/edit`)
    expect(screen.getByText(/^PAYLOAD · PAY-63/)).toBeInTheDocument()

    await router.navigate('/projects/orbital-nav')
    expect(await screen.findByRole('link', { name: /PAY-63/ })).toBeInTheDocument()
  })

  it('blocks an empty title', async () => {
    renderApp('/projects/orbital-nav/new/task')
    const dialog = await openDialog('New payload')
    const count = db.tasks.length

    await userEvent.click(within(dialog).getByRole('button', { name: 'Create payload' }))

    expect(within(dialog).getByText('Give the payload a title.')).toBeInTheDocument()
    expect(within(dialog).getByLabelText('Title')).toHaveAttribute('aria-invalid', 'true')
    expect(db.tasks.length).toBe(count)
  })

  it('creates an objective with the next key', async () => {
    renderApp('/projects/orbital-nav/new/feature')
    const dialog = await openDialog('New objective')
    await userEvent.type(within(dialog).getByLabelText('Name'), 'Offline mode')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create objective' }))

    await screen.findByRole('heading', { name: 'Edit objective' })
    expect(db.features.find((f) => f.name === 'Offline mode')?.key).toBe('OBJ-9')
  })

  it('links the objective dialog to the planner', async () => {
    const { router } = renderApp('/projects/orbital-nav/new/feature')
    const dialog = await openDialog('New objective')
    await userEvent.click(within(dialog).getByRole('link', { name: 'Plan with an astronaut' }))

    expect(router.state.location.pathname).toBe('/projects/orbital-nav/plan/realtime-briefing/briefing')
  })

  it('cancel closes back to the board', async () => {
    const { router } = renderApp('/projects/orbital-nav/new/feature')
    const dialog = await openDialog('New objective')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    expect(router.state.location.pathname).toBe('/projects/orbital-nav')
  })
})

describe('edit page', () => {
  it('adds a success criterion, marks it met, and saves', async () => {
    renderApp('/projects/orbital-nav/features/f-backlog/edit')
    await screen.findByRole('heading', { name: 'Edit objective' })
    expect(screen.getByText('OBJECTIVE · OBJ-7 · MISSION PHASE: CORE')).toBeInTheDocument()
    expect(screen.getByText('2 / 3 met')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Add criterion' }))
    await userEvent.type(screen.getByRole('textbox', { name: 'Criterion 4' }), 'Works with 10k payloads')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Met: Works with 10k payloads' }))
    expect(screen.getByText('3 / 4 met')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await screen.findByText('Saved')
    const f = db.features.find((x) => x.id === 'f-backlog')!
    expect(f.successCriteria).toHaveLength(4)
    expect(f.successCriteria[3]).toMatchObject({ text: 'Works with 10k payloads', met: true })
  })

  it('shows a payload with its criteria and modules, and bumps the version on save', async () => {
    renderApp('/projects/orbital-nav/tasks/t-41/edit')
    await screen.findByRole('heading', { name: 'Edit payload' })

    expect(screen.getByText('2 / 3 met')).toBeInTheDocument()
    const modules = screen.getByRole('complementary', { name: 'Modules' })
    expect(within(modules).getAllByRole('listitem')).toHaveLength(4)
    expect(within(modules).getByRole('progressbar', { name: 'Done' })).toHaveAttribute('aria-valuenow', '3')

    const before = db.tasks.find((t) => t.id === 't-41')!.version
    await userEvent.click(screen.getByRole('radio', { name: /Free flight/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await screen.findByText('Saved')
    const t = db.tasks.find((x) => x.id === 't-41')!
    expect(t.version).toBe(before + 1)
    expect(t.autonomyOverride).toBe(2)
  })

  it('archives an objective', async () => {
    renderApp('/projects/orbital-nav/features/f-backlog/edit')
    await userEvent.click(await screen.findByRole('button', { name: 'Archive' }))

    await waitFor(() => expect(db.features.find((f) => f.id === 'f-backlog')?.status).toBe('archived'))
    expect(await screen.findByText('Archived')).toBeInTheDocument()
  })

  it('shows not found for a missing objective or payload', async () => {
    renderApp('/projects/orbital-nav/features/nope/edit')
    expect(await screen.findByRole('heading', { name: 'Objective not found' })).toBeInTheDocument()
  })

  it('shows not found for a missing payload', async () => {
    renderApp('/projects/orbital-nav/tasks/nope/edit')
    expect(await screen.findByRole('heading', { name: 'Payload not found' })).toBeInTheDocument()
  })
})
