import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../../data/mock/store'
import { renderApp } from '../../../test/render'

const REVIEW = '/projects/orbital-nav/plan/realtime/tasks'

const queue = () => screen.getByRole('navigation', { name: 'Payload proposals' })
const selected = () => screen.getByRole('article', { name: 'Selected payload' })
const pick = (n: number) => userEvent.click(within(queue()).getByRole('button', { name: new RegExp(`Proposal ${n}:`) }))

describe('payload review', () => {
  it('opens on proposal 4 with every field', async () => {
    renderApp(REVIEW)
    const detail = await screen.findByRole('article', { name: 'Selected payload' })
    expect(within(detail).getByRole('heading', { name: 'Resolve concurrent moves on one card' })).toBeInTheDocument()
    expect(within(detail).getByText('Proposal 4 of 8 · proposed by the planning astronaut')).toBeInTheDocument()
    expect(within(detail).getAllByRole('listitem')).toHaveLength(3)
    expect(within(detail).getByText('Core')).toBeInTheDocument()
    expect(within(detail).getByText('5 pt')).toBeInTheDocument()
    expect(within(detail).getByText('High')).toBeInTheDocument()
    expect(await within(detail).findByText('Astronaut-2 · Tethered')).toBeInTheDocument()
    expect(within(detail).getByRole('button', { name: 'Broadcast card moves to operators' })).toBeInTheDocument()
    // The source section's text is quoted from the doc.
    expect(await within(detail).findByText('A conflicting move shows both positions and lets the second mover choose.')).toBeInTheDocument()
    expect(within(queue()).getByRole('button', { name: /Proposal 4:/ })).toHaveAttribute('aria-current', 'true')
    expect(screen.getByText('4 of 8 reviewed · 3 approved · 1 sent back')).toBeInTheDocument()
  })

  it('approves into the Hangar as PAY-63', async () => {
    const { router } = renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve and add to Hangar' }))

    expect(await screen.findByText('5 of 8 reviewed · 4 approved · 1 sent back')).toBeInTheDocument()
    const task = db.tasks.find((t) => t.key === 'PAY-63')!
    expect(task).toMatchObject({ title: 'Resolve concurrent moves on one card', column: 'backlog', featureId: 'f-realtime', sprintId: 's-on-15' })
    expect(task.acceptanceCriteria).toHaveLength(3)
    expect(db.taskProposals.find((p) => p.id === 'pr-4')).toMatchObject({ status: 'approved', taskId: task.id })
    expect(await within(selected()).findByRole('link', { name: 'PAY-63' })).toBeInTheDocument()

    await router.navigate('/projects/orbital-nav')
    const board = await screen.findByRole('region', { name: 'Mission control' })
    expect(await within(board).findByRole('link', { name: /PAY-63/ })).toBeInTheDocument()
  })

  it('blocks a drop without feedback, then drops', async () => {
    renderApp(REVIEW)
    await screen.findByRole('article', { name: 'Selected payload' })
    await pick(5)
    await userEvent.click(within(selected()).getByRole('button', { name: 'Deny with feedback' }))
    await userEvent.click(screen.getByRole('radio', { name: /Drop this payload/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))
    expect(screen.getByText(/Write what should change/)).toBeInTheDocument()
    expect(db.feedback).toHaveLength(0)

    await userEvent.type(screen.getByLabelText(/Feedback/), 'Presence can wait for v2.')
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))

    expect(await within(selected()).findByText('Dropped')).toBeInTheDocument()
    expect(db.feedback[0]).toMatchObject({ targetType: 'proposal', targetId: 'pr-5', outcome: 'discard' })
  })

  it('edits the estimate before approving, which marks it edited', async () => {
    renderApp(REVIEW)
    await screen.findByRole('article', { name: 'Selected payload' })
    await pick(6)
    await userEvent.click(within(selected()).getByRole('button', { name: 'Edit before approving' }))
    const estimate = within(selected()).getByLabelText('Estimate (points)')
    await userEvent.clear(estimate)
    await userEvent.type(estimate, '3')
    await userEvent.click(within(selected()).getByRole('button', { name: 'Approve and add to Hangar' }))

    expect(await within(selected()).findByText('Approved · edited')).toBeInTheDocument()
    const p = db.taskProposals.find((x) => x.id === 'pr-6')!
    expect(p.status).toBe('approved-edited')
    expect(db.tasks.find((t) => t.id === p.taskId)?.estimate).toBe(3)
  })

  it('a revise comes back from the astronaut as Revised', async () => {
    renderApp(REVIEW)
    await screen.findByRole('article', { name: 'Selected payload' })
    await pick(8)
    await userEvent.click(within(selected()).getByRole('button', { name: 'Deny with feedback' }))
    await userEvent.type(screen.getByLabelText(/Feedback/), 'Show it in the board header instead.')
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))

    expect(await within(selected()).findByText('Sent back')).toBeInTheDocument()
    expect(await within(selected()).findByText('Revised', {}, { timeout: 3000 })).toBeInTheDocument()
    expect(within(selected()).getByText(/Revised after your feedback/)).toBeInTheDocument()
    // Back in the queue for review.
    expect(within(selected()).getByRole('button', { name: 'Approve and add to Hangar' })).toBeInTheDocument()
  })

  it('Finish review lists the approved payloads on the Done page', async () => {
    const { router } = renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve and add to Hangar' }))
    await screen.findByText('5 of 8 reviewed · 4 approved · 1 sent back')
    await userEvent.click(screen.getByRole('link', { name: 'Finish review (3 left)' }))

    expect(await screen.findByRole('heading', { name: 'Realtime sync is ready' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/plan/realtime/done')
    const added = screen.getByRole('region', { name: 'Added to the board' })
    expect(within(added).getAllByRole('link').map((a) => a.textContent)).toEqual([
      'PAY-60Open a websocket channel per mission',
      'PAY-61Reconnect with a capped backoff',
      'PAY-62Broadcast card moves to operators',
      'PAY-63Resolve concurrent moves on one card',
    ])
    expect(within(added).getByRole('link', { name: /PAY-63/ })).toHaveAttribute('href', expect.stringMatching(/^\/projects\/orbital-nav\/tasks\/t-/))
    expect(screen.getByRole('link', { name: 'View the objective' })).toHaveAttribute('href', '/projects/orbital-nav/features/f-realtime/edit')
  })

  it('shows an empty state when nothing has been proposed', async () => {
    renderApp('/projects/orbital-nav/plan/realtime-review/tasks')
    expect(await screen.findByRole('heading', { name: 'No payloads proposed yet' })).toBeInTheDocument()
  })
})
