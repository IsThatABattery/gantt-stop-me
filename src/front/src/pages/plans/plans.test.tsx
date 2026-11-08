import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Plan, Proposal } from '../../data/views/plans'
import { server } from '../../test/server'
import { renderApp } from '../../test/render'

const proposal = (id: number, title: string): Proposal => ({
  id,
  position: id,
  status: 'needs_review',
  title,
  description: `${title}, in detail.`,
  acceptanceCriteria: [],
  tests: [{ name: 'npm test', kind: 'script', command: 'npm test', details: '', workflow: null }],
  dependsOn: [],
  agentNote: '',
  taskId: null,
})

const plan = (status: Plan['status'], title = 'Board export'): Plan => ({
  id: 1,
  title,
  description: 'Export the board.',
  status,
  pending: null,
  error: null,
  doc: [],
  messages: [],
  proposals: [proposal(1, 'Export button'), proposal(2, 'CSV writer')],
})

describe('plans', () => {
  it('lists plans in progress and done on their own tabs', async () => {
    server.use(http.get('/api/projects/1/plans', () => HttpResponse.json({ plans: [plan('briefing'), { ...plan('done', 'Old export'), id: 2 }] })))
    renderApp('/projects/1/plans?tab=active')
    expect(await screen.findByRole('link', { name: /Board export/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Old export/ })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Done' }))
    expect(await screen.findByRole('link', { name: /Old export/ })).toBeInTheDocument()
  })

  it('rejects a proposed task with feedback, for the agent to revise', async () => {
    let denied: unknown
    server.use(
      http.get('/api/projects/1/plans/1', () => HttpResponse.json(plan('approved'))),
      http.post('/api/projects/1/plans/1/proposals/2/deny', async ({ request }) => {
        denied = await request.json()
        return HttpResponse.json(plan('approved'))
      }),
    )
    renderApp('/projects/1/plans/1')
    const decide = within(await screen.findByRole('group', { name: 'Decide on CSV writer' }))
    await userEvent.click(decide.getByRole('button', { name: 'Reject CSV writer' }))
    const revise = screen.getByRole('button', { name: 'Revise' })
    expect(revise).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Why reject CSV writer'), 'Stream it, do not buffer.')
    await userEvent.click(revise)
    await waitFor(() => expect(denied).toEqual({ feedback: 'Stream it, do not buffer.', outcome: 'revise' }))
  })
})
