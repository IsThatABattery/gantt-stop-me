import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Run, RunDetail, RunResult } from '../../data/views/runs'
import { fake } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'

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
