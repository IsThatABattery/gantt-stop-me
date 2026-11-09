import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Run, RunDetail, RunResult } from '../../data/views/runs'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'

const passedRun = { id: 1, prNumber: 32, status: 'passed', reason: null, approvedBy: null, approvedAt: null }

/** Task 11, in User Check, with a passed run waiting for the user's check. */
function waitingForCheck() {
  server.use(
    http.get('/api/projects/1/tasks/11/runs', () => HttpResponse.json({ runs: [passedRun] })),
    http.get('/api/projects/1/runs/1', () => HttpResponse.json({ ...passedRun, executions: [], results: [] })),
  )
}

/** The approval band, once the latest run has loaded and Approve and merge is open. */
async function openBand() {
  const band = await screen.findByRole('region', { name: 'Check and approve' })
  await waitFor(() => expect(within(band).getByRole('button', { name: 'Approve and merge' })).toBeEnabled())
  return within(band)
}

async function openDrawer(id: number) {
  const utils = renderApp(`/projects/1/tasks/${id}`)
  const drawer = await screen.findByRole('dialog', { name: 'Task' })
  await within(drawer).findByRole('heading', { level: 1 })
  return { ...utils, drawer }
}

describe('task drawer', () => {
  it('shows the issue: number, column, title, labels, body and a GitHub link', async () => {
    const { drawer } = await openDrawer(7)
    const d = within(drawer)

    expect(d.getByRole('heading', { level: 1, name: 'Draw the timeline' })).toBeInTheDocument()
    expect(d.getByText('#12')).toBeInTheDocument()
    // The column's chip; a themed label set would show its word with the plain one beside it.
    expect(await d.findByText(/^Backlog( · Backlog)?$/)).toBeInTheDocument()
    expect(d.getByRole('link', { name: 'Issue #12 on GitHub' })).toHaveAttribute('href', 'https://github.com/ada/gantt/issues/12')
    // board:* labels repeat the column, so only the others show.
    expect(d.getByText('enhancement')).toBeInTheDocument()
    expect(d.queryByText('board:backlog')).not.toBeInTheDocument()
    expect(d.getByText(/One bar per task, by week\.\s+Second paragraph\./)).toBeInTheDocument()
    expect(d.getByRole('link', { name: 'Edit task' })).toHaveAttribute('href', '/projects/1/tasks/7/edit')
  })

  it('shows the test status and why a task needs attention', async () => {
    const { drawer } = await openDrawer(10)
    expect(within(drawer).getByText('Runner error')).toBeInTheDocument()
    expect(within(drawer).getByText('The runner could not start the container.')).toBeInTheDocument()
  })

  it('opens on Tests for a task that has been tested, and on Details for one that has not', async () => {
    const { drawer, unmount } = await openDrawer(10)
    expect(within(drawer).getByRole('tab', { name: /^Tests/ })).toHaveAttribute('aria-selected', 'true')
    unmount()
    const second = await openDrawer(7)
    expect(within(second.drawer).getByRole('tab', { name: 'Details' })).toHaveAttribute('aria-selected', 'true')
  })

  it('says when there is no description', async () => {
    const { drawer } = await openDrawer(8)
    expect(within(drawer).getByText('No description yet.')).toBeInTheDocument()
  })

  it('shows a message for a task that does not exist', async () => {
    renderApp('/projects/1/tasks/999')
    expect(await screen.findByRole('heading', { name: 'Task not found' })).toBeInTheDocument()
  })

  it('Esc closes back to the board', async () => {
    const { router } = await openDrawer(7)
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1'))
  })

  it('offers to try again when the task fails to load', async () => {
    failNext('GET /api/projects/1/tasks/8', { status: 502, code: 'github_error', message: 'GitHub did not answer in time.' })
    renderApp('/projects/1/tasks/8')
    await userEvent.click(await screen.findByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Zoom the timeline' })).toBeInTheDocument()
  })

  it('goes back to the Backlog, not the board, when opened from the Backlog', async () => {
    const { router } = renderApp('/projects/1/backlog/tasks/999')
    expect(await screen.findByRole('link', { name: 'Back to Backlog' })).toHaveAttribute('href', '/projects/1/backlog')

    await router.navigate('/projects/1/backlog/tasks/7')
    await userEvent.click(await screen.findByRole('link', { name: 'Edit task' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))
    expect(router.state.location.pathname).toBe('/projects/1/backlog/tasks/7')
  })

  it('shows read-only members no write controls', async () => {
    fake.project.permission = 'read'
    waitingForCheck()
    const { drawer } = await openDrawer(11)
    expect(await within(drawer).findByRole('heading', { name: /Latest test run/ })).toBeInTheDocument()
    expect(within(drawer).queryByRole('link', { name: 'Edit task' })).not.toBeInTheDocument()
    expect(within(drawer).queryByRole('button', { name: 'Retest' })).not.toBeInTheDocument()
    // The approval band is the redesign's "Your check".
    expect(screen.queryByRole('region', { name: 'Check and approve' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Approve and merge' })).not.toBeInTheDocument()
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
    renderApp('/projects/1/tasks/7')
    await userEvent.click(await screen.findByRole('tab', { name: /^Tests/ }))
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

  it('waits for the runs before offering a test run', async () => {
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
    // A tested task opens on its Tests tab.
    expect(await screen.findByRole('tabpanel', { name: /^Tests/ })).toBeInTheDocument()
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
    const { drawer } = await openDrawer(7)
    await userEvent.click(within(drawer).getByRole('tab', { name: /^Tests/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t load the test runs. Database is restarting.')
    fails = false
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('shows the error of the last action only, under its own button', async () => {
    waitingForCheck()
    server.use(
      http.post('/api/projects/1/tasks/11/approve', () => HttpResponse.json({ error: { code: 'not_mergeable', message: 'Approve failed.' } }, { status: 409 })),
      http.post('/api/projects/1/tasks/11/request-changes', () =>
        HttpResponse.json({ error: { code: 'github_error', message: 'Request changes failed.' } }, { status: 502 }),
      ),
    )
    await openDrawer(11)
    const band = await openBand()
    await userEvent.click(band.getByRole('button', { name: 'Approve and merge' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Approve failed.')

    // Request changes opens the note; sending it is the action.
    await userEvent.click(band.getByRole('button', { name: 'Request changes' }))
    await userEvent.type(band.getByRole('textbox', { name: 'What should change?' }), 'Fix the export')
    await userEvent.click(band.getByRole('button', { name: 'Send back for changes' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Request changes failed.'))
    expect(screen.getAllByRole('alert')).toHaveLength(1)
  })

  it('keeps a failed approve’s reason when the task leaves User Check', async () => {
    waitingForCheck()
    server.use(
      http.post('/api/projects/1/tasks/11/approve', () => {
        fake.tasks.find((t) => t.id === 11)!.columnId = 3 // the branch moved: back to Testing
        return HttpResponse.json({ error: { code: 'head_moved', message: 'The branch moved; testing again.' } }, { status: 409 })
      }),
    )
    await openDrawer(11)
    const band = await openBand()
    await userEvent.click(band.getByRole('button', { name: 'Approve and merge' }))
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Check and approve' })).not.toBeInTheDocument())
    expect(await screen.findByRole('alert')).toHaveTextContent('The branch moved; testing again.')
    expect(screen.getAllByRole('alert')).toHaveLength(1)
  })

  it('asks before Esc throws away the feedback', async () => {
    waitingForCheck()
    const { router } = await openDrawer(11)
    const band = within(await screen.findByRole('region', { name: 'Check and approve' }))
    await userEvent.click(band.getByRole('button', { name: 'Request changes' }))
    await userEvent.type(band.getByRole('textbox', { name: 'What should change?' }), 'Fix the export')
    await userEvent.keyboard('{Escape}')
    expect(await screen.findByRole('alertdialog')).toHaveTextContent('Leave without saving?')
    expect(router.state.location.pathname).toBe('/projects/1/tasks/11')
  })
})

describe('approval in User Check', () => {
  const run: Run = { id: 4, prNumber: 21, status: 'passed', reason: null, approvedBy: null, approvedAt: null }
  const result = (id: number, status: RunResult['status'], text: string): RunResult => ({
    id,
    position: id,
    text,
    kind: 'llm',
    workflow: 'frontend',
    probabilities: {},
    status,
    detail: '',
    hasScreenshot: false,
    hasVideo: false,
  })
  const detail: RunDetail = { ...run, executions: [], results: [result(21, 'passed', '`npm test`'), result(22, 'manual', 'The PNG opens in a viewer')] }

  function serveRun() {
    const sent: { path: string; body: unknown }[] = []
    server.use(
      http.get('/api/projects/1/tasks/11/runs', () => HttpResponse.json({ runs: [run] })),
      http.get('/api/projects/1/runs/4', () => HttpResponse.json(detail)),
      http.post('/api/projects/1/tasks/11/approve', async ({ request }) => {
        sent.push({ path: 'approve', body: await request.json() })
        return HttpResponse.json({ ...run, approvedBy: 'ada', approvedAt: '2026-11-09T10:00:00Z' })
      }),
      http.post('/api/projects/1/tasks/11/request-changes', async ({ request }) => {
        sent.push({ path: 'request-changes', body: await request.json() })
        return HttpResponse.json(fake.tasks.find((t) => t.id === 11))
      }),
    )
    return sent
  }

  it('approves and merges once the manual tests are ticked', async () => {
    const sent = serveRun()
    renderApp('/projects/1/tasks/11')
    const band = await screen.findByRole('region', { name: 'Check and approve' })
    const approve = within(band).getByRole('button', { name: 'Approve and merge' })
    await waitFor(() => expect(within(band).getByRole('checkbox', { name: 'The PNG opens in a viewer' })).toBeInTheDocument())
    expect(approve).toBeDisabled()

    await userEvent.click(within(band).getByRole('checkbox', { name: 'The PNG opens in a viewer' }))
    await userEvent.click(approve)
    await waitFor(() => expect(sent).toEqual([{ path: 'approve', body: { checked: [22] } }]))
    expect(await screen.findByText('Approved and merged. #16 is closed.')).toBeInTheDocument()
  })

  it('says why Approve is closed while the latest run has not passed', async () => {
    serveRun()
    server.use(
      http.get('/api/projects/1/tasks/11/runs', () => HttpResponse.json({ runs: [{ ...run, status: 'failed' }] })),
      http.get('/api/projects/1/runs/4', () => HttpResponse.json({ ...detail, status: 'failed' })),
    )
    renderApp('/projects/1/tasks/11')
    const band = await screen.findByRole('region', { name: 'Check and approve' })
    expect(await within(band).findByText(/The latest test run failed/)).toBeInTheDocument()
    expect(within(band).getByRole('button', { name: 'Approve and merge' })).toBeDisabled()
  })

  it('requests changes with a note for the pull request', async () => {
    const sent = serveRun()
    renderApp('/projects/1/tasks/11')
    const band = await screen.findByRole('region', { name: 'Check and approve' })
    await userEvent.click(within(band).getByRole('button', { name: 'Request changes' }))
    const note = within(band).getByLabelText('What should change?')
    expect(note).toHaveFocus()

    await userEvent.click(within(band).getByRole('button', { name: 'Send back for changes' }))
    expect(within(band).getByRole('alert')).toHaveTextContent('Say what should change')
    expect(sent).toEqual([])

    await userEvent.type(note, 'The PNG is blurry at 2x.')
    await userEvent.click(within(band).getByRole('button', { name: 'Send back for changes' }))
    await waitFor(() => expect(sent).toEqual([{ path: 'request-changes', body: { feedback: 'The PNG is blurry at 2x.' } }]))
  })

  it('shows no band outside User Check', async () => {
    await openDrawer(7)
    expect(screen.queryByRole('region', { name: 'Check and approve' })).not.toBeInTheDocument()
  })
})
