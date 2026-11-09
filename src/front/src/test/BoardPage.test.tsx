import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { fake, failNext } from './fakeApi'
import { renderApp } from './render'
import { server } from './server'

const column = (name: RegExp) => screen.getByRole('region', { name })
const cardsIn = (name: RegExp) =>
  within(column(name))
    .queryAllByRole('link', { name: /^#/ })
    .map((l) => l.getAttribute('aria-label')!.split(':')[0])
const card = (n: number) => screen.getByRole('link', { name: new RegExp(`^#${n}:`) })
const task = (id: number) => fake.tasks.find((t) => t.id === id)!
const moves = () => fake.requests.filter((r) => r.path.endsWith('/move'))

async function openBoard() {
  const utils = renderApp('/projects/1')
  await screen.findByRole('link', { name: /^#12:/ })
  return utils
}

describe('board', () => {
  it('offers Plan a feature from the board', async () => {
    await openBoard()
    // The rail has a link of the same name; this is the board's own.
    const plan = within(screen.getByRole('region', { name: 'Board' })).getByRole('link', { name: 'Plan a feature' })
    expect(plan).toHaveAttribute('href', '/projects/1/plans')
  })

  it('shows the repository and each issue in its column', async () => {
    await openBoard()
    expect(screen.getByRole('heading', { name: 'gantt' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'ada/gantt' })).toHaveAttribute('href', 'https://github.com/ada/gantt')
    expect(cardsIn(/^Backlog/)).toEqual(['#12', '#13'])
    expect(cardsIn(/^In Progress/)).toEqual(['#14'])
    expect(cardsIn(/^Closed/)).toEqual(['#17'])
    // A runner error flags the task: the card says Needs attention, keeps the run in its name, and reads the reason.
    expect(card(15)).toHaveAccessibleName('#15: Flaky login test. Needs attention, Runner error')
    expect(within(card(15)).getByText('The runner could not start the container.')).toBeInTheDocument()
    expect(card(15)).toHaveAccessibleDescription(/^The runner could not start the container\./)
  })

  it('marks the columns holding work for a person', async () => {
    await openBoard()
    expect(column(/^Testing/)).toHaveAccessibleName('Testing, 1 task, 1 needs attention')
    expect(within(column(/^Testing/)).getByRole('img', { name: '1 needs attention' })).toBeInTheDocument()
    expect(column(/^User Check/)).toHaveAccessibleName('User Check, 1 task, 1 awaiting approval')
    expect(within(column(/^User Check/)).getByText('A person checks and approves')).toBeInTheDocument()
    expect(column(/^Backlog/)).toHaveAccessibleName('Backlog, 2 tasks')
  })

  it("lays out the API's columns in its order, however many there are", async () => {
    // A column the label set doesn't know yet: it takes the API's name and joins the keyboard moves.
    fake.columns.push({ id: 6, key: 'review' as never, name: 'Review', position: 1.5, closesIssue: false })
    task(9).columnId = 6
    await openBoard()
    const names = within(screen.getByRole('region', { name: 'Board' }))
      .getAllByRole('heading', { level: 2 })
      .map((h) => h.textContent)
    expect(names).toEqual(['Backlog', 'In Progress', 'Review', 'Testing', 'User Check', 'Closed'])
    expect(cardsIn(/^Review/)).toEqual(['#14'])

    card(14).focus()
    await userEvent.keyboard(']')
    // Same height in the next column: above #15
    expect(cardsIn(/^Testing/)).toEqual(['#14', '#15'])
    await waitFor(() => expect(moves()[0].body).toMatchObject({ columnId: 3, beforeTaskId: 10 }))
  })

  it('shows a still mark for queued tests and a moving one for running tests', async () => {
    Object.assign(task(8), { runStatus: 'queued' })
    Object.assign(task(9), { runStatus: 'running' })
    await openBoard()
    expect(within(card(13)).getByText('Tests queued').querySelector('svg')).not.toHaveClass('motion-safe:animate-live')
    expect(within(card(14)).getByText('Tests running').querySelector('svg')).toHaveClass('motion-safe:animate-live')
  })

  it('gives read-only members no create links and no move keys', async () => {
    fake.project.permission = 'read'
    await openBoard()
    const board = within(screen.getByRole('region', { name: 'Board' }))
    expect(board.queryByRole('link', { name: 'Plan a feature' })).not.toBeInTheDocument()
    expect(board.queryByRole('link', { name: 'New task' })).not.toBeInTheDocument()
    expect(card(12)).toHaveAccessibleDescription('Arrow keys move between tasks. Enter opens it.')
    card(12).focus()
    await userEvent.keyboard(']')
    expect(cardsIn(/^Backlog/)).toContain('#12')
    expect(moves()).toHaveLength(0)
  })

  it('shows only Closed on a closed card, keeping the run in its name', async () => {
    await openBoard()
    expect(within(card(17)).getByText('Closed')).toBeInTheDocument()
    expect(within(card(17)).queryByText('Tests passed')).not.toBeInTheDocument()
    expect(card(17)).toHaveAccessibleName(/Tests passed/)
  })
})

describe('board keyboard', () => {
  it('moves focus between cards with the arrow keys', async () => {
    await openBoard()
    card(12).focus()

    await userEvent.keyboard('{ArrowDown}')
    expect(card(13)).toHaveFocus()
    await userEvent.keyboard('{ArrowRight}')
    expect(card(14)).toHaveFocus()
    await userEvent.keyboard('{ArrowLeft}')
    expect(card(12)).toHaveFocus()
  })

  it('moves a card one column with ] and [, keeping focus on it', async () => {
    await openBoard()
    card(12).focus()

    await userEvent.keyboard(']')
    expect(cardsIn(/^In Progress/)).toEqual(['#12', '#14'])
    expect(cardsIn(/^Backlog/)).toEqual(['#13'])
    await waitFor(() => expect(task(7).columnId).toBe(2))
    expect(moves()[0].body).toEqual({ columnId: 2, beforeTaskId: 9, version: 1 })
    await waitFor(() => expect(card(12)).toHaveFocus())
    expect(screen.getByText('#12 moved to In Progress.')).toBeInTheDocument()

    // The next move sends the version the API answered with.
    await waitFor(() => expect(within(column(/^In Progress/)).getAllByRole('link')).toHaveLength(2))
    await userEvent.keyboard('[[') // "[[" types one [
    await waitFor(() => expect(task(7).columnId).toBe(1))
    expect(moves()[1].body).toMatchObject({ columnId: 1, version: 2 })
  })

  it('opens the drawer with Enter', async () => {
    const { router } = await openBoard()
    card(12).focus()
    await userEvent.keyboard('{Enter}')
    expect(router.state.location.pathname).toBe('/projects/1/tasks/7')
  })
})

describe('board moves', () => {
  it('snaps a stale move back and says why', async () => {
    await openBoard()
    task(7).version = 2 // someone else moved it since the board loaded
    card(12).focus()

    await userEvent.keyboard(']')

    expect(await screen.findByRole('alert')).toHaveTextContent("#12 couldn't move: someone else changed it first. It's in Backlog.")
    expect(cardsIn(/^Backlog/)).toContain('#12')
    expect(cardsIn(/^In Progress/)).not.toContain('#12')
    expect(task(7).columnId).toBe(1)
  })

  it('says where a conflicting card really is', async () => {
    await openBoard()
    Object.assign(task(7), { columnId: 3, position: 5, version: 2 }) // someone moved #12 to Testing
    card(12).focus()
    await userEvent.keyboard(']')

    expect(await screen.findByRole('alert')).toHaveTextContent("#12 couldn't move: someone else changed it first. It's in Testing.")
    expect(cardsIn(/^Testing/)).toContain('#12')
  })

  it('keeps focus on a card whose keyboard move is refused', async () => {
    let refuse!: () => void
    const refused = new Promise<void>((resolve) => (refuse = resolve))
    server.use(
      http.post('/api/projects/1/tasks/7/move', async () => {
        await refused
        return HttpResponse.json({ error: { code: 'bad_request', message: 'No.' } }, { status: 400 })
      }),
    )
    await openBoard()
    card(12).focus()
    await userEvent.keyboard(']')
    // Refused only once the card has moved, so the rollback remounts it.
    await waitFor(() => expect(cardsIn(/^In Progress/)).toContain('#12'))
    await waitFor(() => expect(card(12)).toHaveFocus())
    refuse()

    await screen.findByRole('alert')
    expect(cardsIn(/^Backlog/)).toContain('#12')
    await waitFor(() => expect(card(12)).toHaveFocus())
  })

  it("shows automation's reason for refusing a move", async () => {
    await openBoard()
    failNext('POST /api/projects/1/tasks/7/move', {
      status: 409,
      code: 'not_tested',
      message: 'Only a task whose tests passed on its latest commit can move to User Check.',
    })
    card(12).focus()
    await userEvent.keyboard(']')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      "#12 couldn't move: Only a task whose tests passed on its latest commit can move to User Check. It's back in Backlog.",
    )
  })
})

describe('board filters', () => {
  it('filters by a counter, and clears it on a second click', async () => {
    await openBoard()
    const attention = screen.getByRole('button', { name: /Needs attention/ })
    expect(attention).toHaveAttribute('aria-pressed', 'false')
    expect(attention).toHaveTextContent('1')

    await userEvent.click(attention)
    expect(attention).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#15'])

    await userEvent.click(attention)
    expect(cardsIn(/^Backlog/)).toEqual(['#12', '#13'])
  })

  it('shows the tasks waiting for approval and the ones being tested', async () => {
    await openBoard()
    await userEvent.click(screen.getByRole('button', { name: /Awaiting approval/ }))
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#16'])

    await userEvent.click(screen.getByRole('button', { name: /Clear filters/ }))
    await userEvent.click(screen.getByRole('button', { name: /Tests running/ }))
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#14'])
  })
})
