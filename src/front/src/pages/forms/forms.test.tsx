import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'

describe('new task', () => {
  it('creates an issue in the Backlog and opens it', async () => {
    const { router } = renderApp('/projects/1/new/task')
    await userEvent.type(await screen.findByLabelText('Title'), '  Print the chart  ')
    await userEvent.type(screen.getByLabelText('Description (optional)'), 'A4, landscape.')
    await userEvent.click(screen.getByRole('button', { name: 'Create task' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1/tasks/13'))
    expect(fake.requests.find((r) => r.method === 'POST' && r.path === '/api/projects/1/tasks')?.body).toEqual({
      title: 'Print the chart',
      body: 'A4, landscape.',
    })
    expect(fake.tasks.find((t) => t.id === 13)).toMatchObject({ title: 'Print the chart', columnId: 1 })
    expect(await screen.findByRole('link', { name: /^#18: Print the chart/ })).toBeInTheDocument()
  })

  it('blocks an empty title without asking the API', async () => {
    renderApp('/projects/1/new/task')
    await userEvent.click(await screen.findByRole('button', { name: 'Create task' }))
    expect(screen.getByText('Give it a title.')).toBeInTheDocument()
    expect(screen.getByLabelText('Title')).toHaveAttribute('aria-invalid', 'true')
    expect(fake.requests.some((r) => r.method === 'POST')).toBe(false)
  })

  it('shows an API refusal on its field', async () => {
    failNext('POST /api/projects/1/tasks', { status: 400, code: 'invalid_body', message: 'Keep the description to 65536 characters.' })
    renderApp('/projects/1/new/task')
    await userEvent.type(await screen.findByLabelText('Title'), 'Too long')
    await userEvent.click(screen.getByRole('button', { name: 'Create task' }))
    expect(await screen.findByText('Keep the description to 65536 characters.')).toBeInTheDocument()
    expect(screen.getByLabelText('Description (optional)')).toHaveAttribute('aria-invalid', 'true')
  })

  it('cancel closes back to the board', async () => {
    const { router } = renderApp('/projects/1/new/task')
    await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))
    expect(router.state.location.pathname).toBe('/projects/1')
  })

  it('opens with the title focused', async () => {
    renderApp('/projects/1/new/task')
    expect(await screen.findByLabelText('Title')).toHaveFocus()
  })

  it('tells read-only members they cannot add one', async () => {
    fake.project.permission = 'read'
    renderApp('/projects/1/new/task')
    expect(await screen.findByRole('heading', { name: 'You have read access only' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Title')).not.toBeInTheDocument()
  })
})

describe('edit task', () => {
  it('saves only what changed and writes it to the issue', async () => {
    renderApp('/projects/1/tasks/7/edit')
    const title = await screen.findByLabelText('Title')
    expect(screen.getByLabelText('Description (optional)')).toHaveValue('One bar per task, by week.\n\nSecond paragraph.')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()

    await userEvent.clear(title)
    await userEvent.type(title, 'Draw the Gantt timeline')
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(await screen.findByText('Saved')).toBeInTheDocument()
    expect(fake.requests.find((r) => r.method === 'PATCH')?.body).toEqual({ title: 'Draw the Gantt timeline' })
    expect(fake.tasks.find((t) => t.id === 7)?.title).toBe('Draw the Gantt timeline')
  })

  it('links to the issue on GitHub', async () => {
    renderApp('/projects/1/tasks/7/edit')
    expect(await screen.findByRole('link', { name: 'View on GitHub' })).toHaveAttribute('href', 'https://github.com/ada/gantt/issues/12')
  })

  it('asks before leaving with unsaved edits', async () => {
    const { router } = renderApp('/projects/1/tasks/7/edit')
    await userEvent.type(await screen.findByLabelText('Description (optional)'), ' More.')
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(await screen.findByRole('alertdialog')).toHaveTextContent('Leave without saving?')
    expect(router.state.location.pathname).toBe('/projects/1/tasks/7/edit')
  })

  it('shows not found for a missing task', async () => {
    renderApp('/projects/1/tasks/999/edit')
    expect(await screen.findByRole('heading', { name: 'Task not found' })).toBeInTheDocument()
  })
})
