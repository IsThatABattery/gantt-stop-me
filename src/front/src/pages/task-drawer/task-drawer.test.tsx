import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderApp } from '../../test/render'

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
})
