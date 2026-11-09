import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { delay, http } from 'msw'
import { describe, expect, it } from 'vitest'
import { fake } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'

describe('assignees', () => {
  it('stays open and focused while a pick saves, so several people can be picked in a row', async () => {
    // Slow the fake's own answer down.
    server.use(http.patch('/api/projects/1/tasks/7', () => delay(200)))
    renderApp('/projects/1/tasks/7')
    const box = await screen.findByRole('combobox', { name: 'Assignees' })
    await userEvent.click(box)
    await userEvent.click(await screen.findByRole('option', { name: 'ada' }))
    expect(box).toBeEnabled()
    expect(box).toHaveFocus()
    expect(screen.getByRole('option', { name: 'ada' })).toHaveAttribute('aria-selected', 'true')
    await userEvent.click(screen.getByRole('option', { name: 'bob' }))
    await waitFor(() => expect(fake.tasks.find((t) => t.id === 7)?.assignees.map((a) => a.login)).toEqual(['ada', 'bob']))
  })

  it('is read-only with read access to the project', async () => {
    fake.project.permission = 'read'
    renderApp('/projects/1/tasks/7')
    expect(await screen.findByRole('combobox', { name: 'Assignees' })).toBeDisabled()
  })
})
