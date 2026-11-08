import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { renderApp } from '../../test/render'

/** Payload keys in the Hangar list, in order. */
async function rowKeys() {
  const table = await screen.findByRole('table', { name: 'Hangar payloads' })
  return within(table)
    .getAllByRole('row')
    .slice(1)
    .map((r) => within(r).getByRole('link').textContent!.match(/PAY-\d+/)![0])
}

describe('hangar', () => {
  it('lists every payload in the Hangar column, in board order', async () => {
    renderApp('/projects/orbital-nav/backlog')
    expect(await screen.findByRole('heading', { name: 'Hangar' })).toBeInTheDocument()
    expect(await rowKeys()).toEqual(['PAY-51', 'PAY-53', 'PAY-47', 'PAY-49', 'PAY-60', 'PAY-61', 'PAY-62'])
    expect(screen.getByText('7 payloads')).toBeInTheDocument()
  })

  it('shows owner, launch window and status on each row', async () => {
    db.tasks.find((t) => t.key === 'PAY-53')!.sprintId = null
    renderApp('/projects/orbital-nav/backlog')
    await rowKeys()
    const row = (key: string) => screen.getByRole('link', { name: new RegExp(key) }).closest('tr')!

    expect(within(row('PAY-60')).getByText('Unassigned')).toBeInTheDocument()
    expect(within(row('PAY-60')).getByText('Upcoming')).toBeInTheDocument()
    expect(within(row('PAY-51')).getByText('Launch window 14')).toBeInTheDocument()
    expect(within(row('PAY-53')).getByText('Not scheduled')).toBeInTheDocument()
    expect(within(row('PAY-49')).getByText('Ready')).toBeInTheDocument()
  })

  it('opens the payload drawer over the list and closes back to it', async () => {
    const { router } = renderApp('/projects/orbital-nav/backlog')
    await rowKeys()
    await userEvent.click(screen.getByRole('link', { name: /PAY-49/ }))

    expect(router.state.location.pathname).toMatch(/^\/projects\/orbital-nav\/backlog\/tasks\//)
    const drawer = await screen.findByRole('dialog', { name: 'Payload' })
    expect(await within(drawer).findByText(/Keyboard shortcuts for board navigation/)).toBeInTheDocument()

    await userEvent.keyboard('{Escape}')
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/backlog')
    expect(screen.getByRole('table', { name: 'Hangar payloads' })).toBeInTheDocument()
  })

  it('says so when the Hangar is empty', async () => {
    db.tasks = db.tasks.filter((t) => t.projectId !== 'orbital-nav' || t.column !== 'backlog')
    renderApp('/projects/orbital-nav/backlog')
    expect(await screen.findByText(/The Hangar is empty/)).toBeInTheDocument()
  })
})
