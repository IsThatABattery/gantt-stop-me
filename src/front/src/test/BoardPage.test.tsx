import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../data/mock/store'
import { forceMoveConflict } from '../data/views/board'
import { renderApp } from './render'

const column = (name: RegExp) => screen.getByRole('region', { name })
const cardsIn = (name: RegExp) =>
  within(column(name))
    .queryAllByRole('link')
    .map((l) => l.getAttribute('aria-label')!.split(':')[0])

async function openBoard() {
  const utils = renderApp('/projects/orbital-nav')
  await screen.findByRole('link', { name: /PAY-49/ })
  return utils
}

describe('board keyboard', () => {
  it('moves focus between cards with the arrow keys', async () => {
    await openBoard()
    screen.getByRole('link', { name: /PAY-51/ }).focus()

    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('link', { name: /PAY-53/ })).toHaveFocus()
    await userEvent.keyboard('{ArrowRight}')
    expect(screen.getByRole('link', { name: /PAY-45/ })).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}')
    expect(screen.getByRole('link', { name: /PAY-44/ })).toHaveFocus()
    await userEvent.keyboard('{ArrowLeft}')
    expect(screen.getByRole('link', { name: /PAY-51/ })).toHaveFocus()
  })

  it('moves a card one column with ] and [, keeping focus on it', async () => {
    await openBoard()
    screen.getByRole('link', { name: /PAY-49/ }).focus()

    await userEvent.keyboard(']')
    expect(cardsIn(/^In orbit/)).toContain('PAY-49')
    expect(cardsIn(/^Hangar/)).not.toContain('PAY-49')
    await waitFor(() => expect(screen.getByRole('link', { name: /PAY-49/ })).toHaveFocus())
    await waitFor(() => expect(db.tasks.find((t) => t.id === 't-49')!.column).toBe('in-progress'))
    expect(screen.getByText('PAY-49 moved to In orbit.')).toBeInTheDocument()

    await userEvent.keyboard('[[')
    await waitFor(() => expect(db.tasks.find((t) => t.id === 't-49')!.column).toBe('backlog'))
  })

  it('opens the drawer with Enter', async () => {
    const { router } = await openBoard()
    screen.getByRole('link', { name: /PAY-49/ }).focus()
    await userEvent.keyboard('{Enter}')
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/tasks/t-49')
  })
})

describe('board moves', () => {
  it('snaps a rejected move back and says why', async () => {
    await openBoard()
    forceMoveConflict()
    screen.getByRole('link', { name: /PAY-49/ }).focus()

    await userEvent.keyboard(']')
    expect(cardsIn(/^In orbit/)).toContain('PAY-49')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      "PAY-49 couldn't move: someone else changed it first. It's back in Hangar.",
    )
    expect(cardsIn(/^Hangar/)).toContain('PAY-49')
    expect(cardsIn(/^In orbit/)).not.toContain('PAY-49')
    expect(db.tasks.find((t) => t.id === 't-49')!.column).toBe('backlog')
  })
})

describe('board filters', () => {
  it('narrows to one objective', async () => {
    await openBoard()
    await userEvent.selectOptions(screen.getByLabelText('Objective'), 'Realtime sync')

    expect(cardsIn(/^Hangar/)).toEqual(['PAY-60', 'PAY-61', 'PAY-62'])
    expect(cardsIn(/^In orbit/)).toEqual(['PAY-44', 'PAY-45'])
    expect(cardsIn(/^Docking/)).toEqual([])
  })

  it('filters by a counter, and clears it on a second click', async () => {
    await openBoard()
    const blocked = screen.getByRole('button', { name: /Comms down/ })
    expect(blocked).toHaveAttribute('aria-pressed', 'false')

    await userEvent.click(blocked)
    expect(blocked).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByRole('link', { name: /^PAY-/ }).map((l) => l.getAttribute('aria-label')!.split(':')[0])).toEqual([
      'PAY-43',
    ])

    await userEvent.click(blocked)
    expect(blocked).toHaveAttribute('aria-pressed', 'false')
    expect(cardsIn(/^Hangar/)).toContain('PAY-49')
  })

  it('shows only agent-owned, not landed cards for Astronauts active', async () => {
    await openBoard()
    await userEvent.click(screen.getByRole('button', { name: /Astronauts active/ }))
    const shown = screen.getAllByRole('link', { name: /^PAY-/ })
    expect(shown.length).toBeGreaterThan(0)
    for (const link of shown) {
      const key = link.getAttribute('aria-label')!.split(':')[0]
      const t = db.tasks.find((x) => x.key === key)!
      expect(t.assignee?.kind).toBe('agent')
      expect(t.status).not.toBe('landed')
    }
  })
})
