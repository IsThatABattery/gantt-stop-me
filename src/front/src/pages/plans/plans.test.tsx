import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { keys } from '../../data/keys'
import type { Plan, Proposal } from '../../data/views/plans'
import { fake } from '../../test/fakeApi'
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
  it('links to Settings only when planning is not set up', async () => {
    server.use(
      http.get('/api/projects/1/plans', () => HttpResponse.json({ plans: [] })),
      http.post('/api/projects/1/plans', () => HttpResponse.json({ error: { code: 'internal', message: 'GitHub is down.' } }, { status: 502 })),
    )
    renderApp('/projects/1/plans')
    await userEvent.type(await screen.findByLabelText('Feature'), 'Export')
    await userEvent.type(screen.getByLabelText('Describe it'), 'As CSV.')
    await userEvent.click(screen.getByRole('button', { name: 'Start interview' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('GitHub is down.')
    expect(within(alert).queryByRole('link')).not.toBeInTheDocument()

    server.use(http.post('/api/projects/1/plans', () => HttpResponse.json({ error: { code: 'planning_not_set_up', message: 'Pick the interviewer first.' } }, { status: 400 })))
    await userEvent.click(screen.getByRole('button', { name: 'Start interview' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Pick the interviewer first.'))
    expect(within(screen.getByRole('alert')).getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/settings')
  })

  it('keeps the plan and the typed answer when refreshing it fails', async () => {
    let gets = 0
    const briefing: Plan = { ...plan('briefing'), proposals: [], messages: [{ id: 1, role: 'agent', text: 'Which formats?', options: ['CSV'], skipped: false }] }
    server.use(
      http.get('/api/projects/1/plans/1', () =>
        ++gets === 1 ? HttpResponse.json(briefing) : HttpResponse.json({ error: { code: 'internal', message: 'GitHub timed out.' } }, { status: 502 }),
      ),
    )
    const { client } = renderApp('/projects/1/plans/1')
    await userEvent.type(await screen.findByLabelText('Your answer'), 'CSV and PNG')
    await client.invalidateQueries({ queryKey: keys.plan('1', '1') })
    await waitFor(() => expect(client.getQueryState(keys.plan('1', '1'))?.status).toBe('error'))
    await new Promise((r) => setTimeout(r, 20)) // let the error render
    expect(screen.getByLabelText('Your answer')).toHaveValue('CSV and PNG')
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument()
  })

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
      http.post('/api/projects/1/plans/1/proposals/1/approve', () => HttpResponse.json(plan('approved'))),
      http.post('/api/projects/1/plans/1/proposals/2/deny', async ({ request }) => {
        denied = await request.json()
        return HttpResponse.json(plan('approved'))
      }),
    )
    renderApp('/projects/1/plans/1')
    const decide = within(await screen.findByRole('group', { name: 'Decide on CSV writer' }))
    await userEvent.click(within(await screen.findByRole('group', { name: 'Decide on Export button' })).getByRole('button', { name: 'Approve Export button' }))
    await userEvent.click(decide.getByRole('button', { name: 'Reject CSV writer' }))
    const confirm = screen.getByRole('button', { name: 'Confirm' })
    expect(confirm).toBeDisabled() // a rejection needs a reason
    await userEvent.type(screen.getByLabelText('Why reject CSV writer'), 'Stream it, do not buffer.')
    await userEvent.selectOptions(screen.getByLabelText('What to do with CSV writer'), 'revise')
    expect(denied).toBeUndefined() // choosing sends nothing
    await userEvent.click(confirm)
    await waitFor(() => expect(denied).toEqual({ feedback: 'Stream it, do not buffer.', outcome: 'revise' }))
  })

  it('rejects every proposed task at once', async () => {
    const denied: number[] = []
    server.use(
      http.get('/api/projects/1/plans/1', () => HttpResponse.json(plan('approved'))),
      http.post('/api/projects/1/plans/1/proposals/:pid/deny', ({ params }) => {
        denied.push(Number(params.pid))
        return HttpResponse.json(plan('approved'))
      }),
    )
    renderApp('/projects/1/plans/1')
    await userEvent.click(await screen.findByRole('button', { name: 'Reject all' }))
    await userEvent.type(screen.getByLabelText('Reason for every rejected task'), 'Out of scope.')
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() => expect(denied).toEqual([1, 2]))
  })

  it('moves a plan to Done once every task is approved and confirmed', async () => {
    let approved = 0
    server.use(
      http.get('/api/projects/1/plans/1', () => HttpResponse.json(plan('approved'))),
      http.post('/api/projects/1/plans/1/proposals/:pid/approve', () => HttpResponse.json(plan(++approved === 2 ? 'done' : 'approved'))),
      http.get('/api/projects/1/plans', () => HttpResponse.json({ plans: [] })),
    )
    const { router } = renderApp('/projects/1/plans/1')
    await userEvent.click(await screen.findByRole('button', { name: 'Approve all' }))
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() => expect(router.state.location.search).toBe('?tab=done'))
    expect(approved).toBe(2)
  })

  it('opens a plan on its first link, not Close', async () => {
    server.use(http.get('/api/projects/1/plans/1', () => HttpResponse.json(plan('needs_review'))))
    renderApp('/projects/1/plans/1')
    await waitFor(() => expect(screen.getByRole('link', { name: '← Plans in progress' })).toHaveFocus())
  })

  it('offers read-only members no new plan, nor actions on one', async () => {
    fake.project.permission = 'read'
    server.use(http.get('/api/projects/1/plans/1', () => HttpResponse.json(plan('needs_review'))))
    const { router } = renderApp('/projects/1/plans')
    expect(await screen.findByRole('heading', { name: 'You have read access only' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start interview' })).not.toBeInTheDocument()

    await router.navigate('/projects/1/plans/1')
    expect(await screen.findByRole('button', { name: 'Approve doc and propose tasks' })).toBeDisabled()
  })
})
