import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'

const automationOn = () => {
  fake.automation = {
    enabled: true,
    owner: { id: 1, login: 'ada' },
    decisionProviderId: 1,
    decisionModel: 'llama3',
    decisionInstructions: '',
    chatProviderId: 1,
    chatModel: 'llama3',
    chatInstructions: '',
    pausedReason: null,
    payloadUrl: 'https://gantt.example.com/api/github/webhook/1',
    webhookSecretSet: false,
  }
}

const passedRun = { id: 1, prNumber: 32, status: 'passed', reason: null, approvedBy: null, approvedAt: null }

/** Task 11, in User Check, with a passed run waiting for the user's check. */
function waitingForCheck() {
  server.use(
    http.get('/api/projects/1/tasks/11/runs', () => HttpResponse.json({ runs: [passedRun] })),
    http.get('/api/projects/1/runs/1', () => HttpResponse.json({ ...passedRun, executions: [], results: [] })),
  )
}

async function openDrawer(id: number) {
  const utils = renderApp(`/projects/1/tasks/${id}`)
  const drawer = await screen.findByRole('dialog', { name: 'Payload' })
  await within(drawer).findByRole('heading', { level: 1 })
  return { ...utils, drawer }
}

describe('payload drawer', () => {
  it('shows the issue: number, column, title, labels, body and a GitHub link', async () => {
    const { drawer } = await openDrawer(7)
    const d = within(drawer)

    expect(d.getByRole('heading', { level: 1, name: 'Draw the timeline' })).toBeInTheDocument()
    expect(d.getByText('#12')).toBeInTheDocument()
    expect(await d.findByText('Hangar · Backlog')).toBeInTheDocument()
    expect(d.getByRole('link', { name: 'Issue #12 on GitHub' })).toHaveAttribute('href', 'https://github.com/ada/gantt/issues/12')
    // board:* labels repeat the column, so only the others show.
    expect(d.getByText('enhancement')).toBeInTheDocument()
    expect(d.queryByText('board:backlog')).not.toBeInTheDocument()
    expect(d.getByText(/One bar per task, by week\.\s+Second paragraph\./)).toBeInTheDocument()
    expect(d.getByRole('link', { name: 'Edit payload' })).toHaveAttribute('href', '/projects/1/tasks/7/edit')
  })

  it('shows the test status and why a task needs attention', async () => {
    const { drawer } = await openDrawer(10)
    expect(within(drawer).getByText('Test error')).toBeInTheDocument()
    expect(within(drawer).getByText('The runner could not start the container.')).toBeInTheDocument()
  })

  it('says when there is no description', async () => {
    const { drawer } = await openDrawer(8)
    expect(within(drawer).getByText('No description yet.')).toBeInTheDocument()
  })

  it('shows a message for a payload that does not exist', async () => {
    renderApp('/projects/1/tasks/999')
    expect(await screen.findByRole('heading', { name: 'Payload not found' })).toBeInTheDocument()
  })

  it('Esc closes back to the board', async () => {
    const { router } = await openDrawer(7)
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1'))
  })

  it('offers to try again when the payload fails to load', async () => {
    failNext('GET /api/projects/1/tasks/8', { status: 502, code: 'github_error', message: 'GitHub did not answer in time.' })
    renderApp('/projects/1/tasks/8')
    await userEvent.click(await screen.findByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Zoom the timeline' })).toBeInTheDocument()
  })

  it('goes back to the Hangar, not the board, when opened from the Hangar', async () => {
    const { router } = renderApp('/projects/1/backlog/tasks/999')
    expect(await screen.findByRole('link', { name: 'Back to Hangar' })).toHaveAttribute('href', '/projects/1/backlog')

    await router.navigate('/projects/1/backlog/tasks/7')
    await userEvent.click(await screen.findByRole('link', { name: 'Edit payload' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))
    expect(router.state.location.pathname).toBe('/projects/1/backlog/tasks/7')
  })

  it('shows read-only members no write controls', async () => {
    fake.project.permission = 'read'
    automationOn()
    waitingForCheck()
    const { drawer } = await openDrawer(11)
    expect(await within(drawer).findByRole('heading', { name: /Latest test run/ })).toBeInTheDocument()
    expect(within(drawer).queryByRole('link', { name: 'Edit payload' })).not.toBeInTheDocument()
    expect(within(drawer).queryByRole('button', { name: 'Retest' })).not.toBeInTheDocument()
    expect(within(drawer).queryByText('Your check')).not.toBeInTheDocument()
  })
})

describe('assignees', () => {
  it('assigns people to the task on GitHub', async () => {
    renderApp('/projects/1/tasks/7')
    await userEvent.click(await screen.findByLabelText('Assignees'))
    await userEvent.click(screen.getByRole('option', { name: 'bob' }))
    await waitFor(() => expect(fake.requests.find((r) => r.method === 'PATCH')?.body).toEqual({ assignees: ['bob'] }))
    expect(fake.tasks.find((t) => t.id === 7)?.assignees.map((a) => a.login)).toEqual(['bob'])
  })
})

describe('test runs', () => {
  it('starts a test run by hand', async () => {
    automationOn()
    renderApp('/projects/1/tasks/7')
    await userEvent.click(await screen.findByRole('button', { name: 'Run tests' }))
    await waitFor(() => expect(fake.requests.some((r) => r.method === 'POST' && r.path === '/api/projects/1/tasks/7/retest')).toBe(true))
  })

  it('links the branch made for the task', async () => {
    fake.tasks.find((t) => t.id === 7)!.branch = 'feature/12-draw-the-timeline'
    renderApp('/projects/1/tasks/7')
    expect(await screen.findByRole('link', { name: 'feature/12-draw-the-timeline' })).toHaveAttribute(
      'href',
      'https://github.com/ada/gantt/tree/feature/12-draw-the-timeline',
    )
  })

  it('offers no test run while automation is off', async () => {
    await openDrawer(7)
    await waitFor(() => expect(fake.requests.some((r) => r.path === '/api/projects/1/automation')).toBe(true))
    expect(screen.queryByRole('button', { name: 'Run tests' })).not.toBeInTheDocument()
  })

  it('waits for the runs before offering a test run', async () => {
    automationOn()
    let answer = () => {}
    const answered = new Promise<void>((resolve) => (answer = resolve))
    server.use(
      http.get('/api/projects/1/tasks/9/runs', async () => {
        await answered
        return HttpResponse.json({ runs: [{ ...passedRun, status: 'running' }] })
      }),
      http.get('/api/projects/1/runs/1', () => HttpResponse.json({ ...passedRun, status: 'running', executions: [], results: [] })),
    )
    await openDrawer(9)
    await waitFor(() => expect(fake.requests.some((r) => r.path === '/api/projects/1/automation')).toBe(true))
    expect(screen.queryByRole('button', { name: 'Run tests' })).not.toBeInTheDocument()
    answer()
    expect(await screen.findByRole('button', { name: 'Restart test run' })).toBeInTheDocument()
  })

  it('says when the runs fail to load, and tries again', async () => {
    let fails = true
    server.use(
      http.get('/api/projects/1/tasks/7/runs', () =>
        fails ? HttpResponse.json({ error: { code: 'internal', message: 'Database is restarting.' } }, { status: 500 }) : undefined,
      ),
    )
    await openDrawer(7)
    expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t load the test runs. Database is restarting.')
    fails = false
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('shows the error of the last action only, under its own button', async () => {
    automationOn()
    waitingForCheck()
    server.use(
      http.post('/api/projects/1/tasks/11/approve', () => HttpResponse.json({ error: { code: 'not_mergeable', message: 'Approve failed.' } }, { status: 409 })),
      http.post('/api/projects/1/tasks/11/request-changes', () =>
        HttpResponse.json({ error: { code: 'github_error', message: 'Request changes failed.' } }, { status: 502 }),
      ),
    )
    await openDrawer(11)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve, merge and close' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Approve failed.')

    await userEvent.type(screen.getByRole('textbox', { name: 'What to change' }), 'Fix the export')
    await userEvent.click(screen.getByRole('button', { name: 'Request changes' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Request changes failed.'))
    expect(screen.getAllByRole('alert')).toHaveLength(1)
  })

  it('asks before Esc throws away the feedback', async () => {
    automationOn()
    waitingForCheck()
    const { router } = await openDrawer(11)
    await userEvent.type(await screen.findByRole('textbox', { name: 'What to change' }), 'Fix the export')
    await userEvent.keyboard('{Escape}')
    expect(await screen.findByRole('alertdialog')).toHaveTextContent('Leave without saving?')
    expect(router.state.location.pathname).toBe('/projects/1/tasks/11')
  })
})
