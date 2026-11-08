import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { renderApp } from '../../test/render'

const drawer = () => screen.findByRole('dialog', { name: 'Payload' })
const column = (name: RegExp) => screen.getByRole('region', { name })

async function openTask(id: string) {
  const utils = renderApp(`/projects/orbital-nav/tasks/${id}`)
  const d = await drawer()
  await within(d).findByRole('link', { name: 'Edit payload' })
  return { ...utils, d }
}

describe('payload drawer', () => {
  it('shows PAY-41 in full: header, modules, trace, cargo and the approval bar', async () => {
    const { d } = await openTask('t-41')

    expect(within(d).getByRole('heading', { name: 'Pagination for backlog endpoint' })).toBeInTheDocument()
    expect(within(d).getByText('Mission review · Awaiting review')).toBeInTheDocument()
    expect(within(d).getByRole('link', { name: 'Edit payload' })).toHaveAttribute('href', '/projects/orbital-nav/tasks/t-41/edit')
    expect(within(d).getByText(/Astronaut-1 · Guided/)).toBeInTheDocument()
    expect(within(d).getByRole('tab', { name: 'Modules 3/4' })).toBeInTheDocument()

    const trace = within(d).getByRole('region', { name: 'Astronaut trace' })
    expect(within(trace).getAllByRole('listitem')).toHaveLength(5)

    await userEvent.click(within(d).getByRole('tab', { name: 'Cargo' }))
    expect(within(d).getByText('PR #212')).toBeInTheDocument()
    expect(within(d).getByText('all checks passing')).toBeInTheDocument()

    expect(within(d).getByRole('button', { name: 'Approve & land' })).toBeInTheDocument()
    expect(within(d).getByText('A approve · R request changes · T take over')).toBeInTheDocument()
  })

  it('shows acceptance criteria with met and unmet marked in words', async () => {
    const { d } = await openTask('t-41')
    expect(within(d).getByText('2 of 3 met')).toBeInTheDocument()
    expect(within(d).getByText(/Not met:/).parentElement).toHaveTextContent('Tests cover first, middle, and last page')
  })

  it('A approves and lands the payload on the board', async () => {
    const { d } = await openTask('t-41')
    await userEvent.keyboard('a')

    expect(await within(d).findByText(/PAY-41 approved and landed/)).toBeInTheDocument()
    expect(within(d).getByText('Landed · Done')).toBeInTheDocument()
    expect(within(d).queryByRole('button', { name: 'Approve & land' })).not.toBeInTheDocument()
    await waitFor(() => expect(within(column(/^Landed · Closed/)).getByRole('link', { name: /PAY-41/ })).toBeInTheDocument())

    const task = db.tasks.find((t) => t.id === 't-41')!
    expect(task).toMatchObject({ column: 'closed', status: 'landed', version: 2 })
    expect(db.activityEvents.at(-1)).toMatchObject({ verb: 'approved', taskId: 't-41', actor: { kind: 'user', id: 'u-tv' } })
    expect(db.activityEvents.find((e) => e.id === 'ev-1')!.needsAction).toBe(false)
  })

  it('R asks for a required note, then sends the payload back to In orbit', async () => {
    const { d } = await openTask('t-42')
    await userEvent.keyboard('r')

    const note = within(d).getByLabelText(/What should change/)
    expect(note).toHaveFocus()
    await userEvent.click(within(d).getByRole('button', { name: 'Send back for changes' }))
    expect(within(d).getByRole('alert')).toHaveTextContent('Add a note')

    // Typing a, r and t into the note must not trigger the shortcuts.
    await userEvent.type(note, 'Rename the daylight tokens')
    expect(note).toHaveValue('Rename the daylight tokens')
    await userEvent.click(within(d).getByRole('button', { name: 'Send back for changes' }))

    expect(await within(d).findByText(/PAY-42 is back in In orbit/)).toBeInTheDocument()
    await waitFor(() => expect(within(column(/^In orbit · In Progress/)).getByRole('link', { name: /PAY-42/ })).toBeInTheDocument())
    expect(db.tasks.find((t) => t.id === 't-42')).toMatchObject({ column: 'in-progress', status: 'working' })
    expect(db.activityEvents.at(-1)).toMatchObject({ verb: 'requested-changes', detail: 'Rename the daylight tokens' })

    await userEvent.click(within(d).getByRole('tab', { name: 'Flight log' }))
    expect(within(d).getByText('Rename the daylight tokens')).toBeInTheDocument()
  })

  it('T hands the payload to the signed-in operator', async () => {
    const { d } = await openTask('t-41')
    await userEvent.keyboard('t')

    expect(await within(d).findByText('You took over PAY-41.')).toBeInTheDocument()
    expect(db.tasks.find((t) => t.id === 't-41')).toMatchObject({ assignee: { kind: 'user', id: 'u-tv' }, status: 'working' })
    expect(within(d).getByText('Mission review · Working')).toBeInTheDocument()
  })

  it('has no approval bar outside Mission review, and the shortcuts do nothing', async () => {
    const { d } = await openTask('t-51')
    expect(within(d).queryByRole('button', { name: 'Approve & land' })).not.toBeInTheDocument()
    await userEvent.keyboard('a')
    expect(db.tasks.find((t) => t.id === 't-51')!.status).toBe('queued')
  })

  it('ticks modules and keeps the counts in step', async () => {
    const { d } = await openTask('t-41')
    await userEvent.click(within(d).getByRole('tab', { name: 'Modules 3/4' }))
    await userEvent.click(within(d).getByRole('checkbox', { name: 'Document the response shape flag' }))

    expect(await within(d).findByRole('tab', { name: 'Modules 4/4' })).toBeInTheDocument()
    expect(within(d).getByRole('progressbar', { name: 'Done' })).toHaveAttribute('aria-valuenow', '4')
    expect(db.tasks.find((t) => t.id === 't-41')!.subtasks).toEqual({ done: 4, total: 4 })
  })

  it('shows the open hail in the Flight log with a link to the console', async () => {
    const { d } = await openTask('t-45')
    await userEvent.click(within(d).getByRole('tab', { name: 'Flight log' }))
    expect(within(d).getByText(/Should the reconnect retry cap be 3 or 5/)).toBeInTheDocument()
    expect(within(d).getByRole('link', { name: 'Answer in the Astronauts console' })).toHaveAttribute('href', '/projects/orbital-nav/agents')
  })

  it('shows a message for a payload that does not exist', async () => {
    renderApp('/projects/orbital-nav/tasks/t-999')
    const d = await drawer()
    expect(await within(d).findByRole('heading', { name: 'Payload not found' })).toBeInTheDocument()
  })

  it('Esc closes back to the board', async () => {
    const { router } = await openTask('t-41')
    await userEvent.keyboard('{Escape}')
    expect(router.state.location.pathname).toBe('/projects/orbital-nav')
  })
})
