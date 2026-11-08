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

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('objective create dialog', () => {
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

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('objective edit page', () => {
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

  it('archives an objective', async () => {
    renderApp('/projects/orbital-nav/features/f-backlog/edit')
    await userEvent.click(await screen.findByRole('button', { name: 'Archive' }))

    await waitFor(() => expect(db.features.find((f) => f.id === 'f-backlog')?.status).toBe('archived'))
    expect(await screen.findByText('Archived')).toBeInTheDocument()
  })

  it('shows not found for a missing objective', async () => {
    renderApp('/projects/orbital-nav/features/nope/edit')
    expect(await screen.findByRole('heading', { name: 'Objective not found' })).toBeInTheDocument()
  })

})
