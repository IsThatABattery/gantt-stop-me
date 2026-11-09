import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake } from '../../test/fakeApi'
import { renderApp } from '../../test/render'

const list = async () => screen.findByRole('list', { name: 'Backlog tasks' })

describe('backlog', () => {
  it('lists the Backlog column in board order', async () => {
    renderApp('/projects/1/backlog')
    const backlog = fake.tasks.filter((t) => t.columnId === 1).sort((a, b) => a.position - b.position)
    const rows = within(await list()).getAllByRole('link').map((l) => l.textContent)
    expect(rows.map((r) => r?.match(/^#\d+/)?.[0])).toEqual(backlog.map((t) => `#${t.issueNumber}`))
  })

  it('opens the drawer over the list and closes back to it', async () => {
    const { router } = renderApp('/projects/1/backlog')
    await userEvent.click(within(await list()).getAllByRole('link')[0])
    expect(router.state.location.pathname).toMatch(/^\/projects\/1\/backlog\/tasks\/\d+$/)
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Close' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1/backlog'))
  })

  it('says so when the Backlog is empty', async () => {
    fake.tasks = fake.tasks.filter((t) => t.columnId !== 1)
    renderApp('/projects/1/backlog')
    expect(await screen.findByText(/The Backlog is empty/)).toBeInTheDocument()
  })
})
