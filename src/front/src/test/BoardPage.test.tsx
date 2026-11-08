import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake, failNext } from './fakeApi'
import { renderApp } from './render'

const column = (name: RegExp) => screen.getByRole('region', { name })
const cardsIn = (name: RegExp) =>
  within(column(name))
    .queryAllByRole('link')
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
  it('shows the repository and each issue in its column', async () => {
    await openBoard()
    expect(screen.getByRole('heading', { name: 'gantt' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'ada/gantt' })).toHaveAttribute('href', 'https://github.com/ada/gantt')
    expect(cardsIn(/^Hangar/)).toEqual(['#12', '#13'])
    expect(cardsIn(/^In orbit/)).toEqual(['#14'])
    expect(cardsIn(/^Landed/)).toEqual(['#17'])
    expect(card(15)).toHaveAccessibleName('#15: Flaky login test. Test error')
    expect(within(card(15)).getByText('The runner could not start the container.')).toBeInTheDocument()
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
    expect(cardsIn(/^In orbit/)).toEqual(['#12', '#14'])
    expect(cardsIn(/^Hangar/)).toEqual(['#13'])
    await waitFor(() => expect(task(7).columnId).toBe(2))
    expect(moves()[0].body).toEqual({ columnId: 2, beforeTaskId: 9, version: 1 })
    await waitFor(() => expect(card(12)).toHaveFocus())
    expect(screen.getByText('#12 moved to In orbit.')).toBeInTheDocument()

    // The next move sends the version the API answered with.
    await waitFor(() => expect(within(column(/^In orbit/)).getAllByRole('link')).toHaveLength(2))
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

    expect(await screen.findByRole('alert')).toHaveTextContent("#12 couldn't move: someone else changed it first. It's back in Hangar.")
    expect(cardsIn(/^Hangar/)).toContain('#12')
    expect(cardsIn(/^In orbit/)).not.toContain('#12')
    expect(task(7).columnId).toBe(1)
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
      "#12 couldn't move: Only a task whose tests passed on its latest commit can move to User Check. It's back in Hangar.",
    )
  })
})

describe('board filters', () => {
  it('filters by a counter, and clears it on a second click', async () => {
    await openBoard()
    const attention = screen.getByRole('button', { name: /Comms down/ })
    expect(attention).toHaveAttribute('aria-pressed', 'false')
    expect(attention).toHaveTextContent('1')

    await userEvent.click(attention)
    expect(attention).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#15'])

    await userEvent.click(attention)
    expect(cardsIn(/^Hangar/)).toEqual(['#12', '#13'])
  })

  it('shows the tasks waiting for approval and the ones being tested', async () => {
    await openBoard()
    await userEvent.click(screen.getByRole('button', { name: /Awaiting approval/ }))
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#16'])

    await userEvent.click(screen.getByRole('button', { name: /Clear filters/ }))
    await userEvent.click(screen.getByRole('button', { name: /Static fires running/ }))
    expect(screen.getAllByRole('link', { name: /^#/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual(['#14'])
  })
})
