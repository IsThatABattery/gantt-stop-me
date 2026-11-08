import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { renderApp } from '../../test/render'

// A tile's name starts with its avatar label, e.g. "Astronaut Astronaut-3, Guided…"
const tile = (name: string) => screen.getByRole('button', { name: new RegExp(`^Astronaut ${name}`) })
const panel = () => screen.getByRole('region', { name: /^Selected astronaut/ })
const agent = (id: string) => db.agents.find((a) => a.id === id)!
const task = (id: string) => db.tasks.find((t) => t.id === id)!

async function openConsole(project = 'orbital-nav') {
  const utils = renderApp(`/projects/${project}/agents`)
  await screen.findByRole('heading', { name: 'Astronaut console' })
  return utils
}

async function loaded() {
  await openConsole()
  await screen.findByRole('group', { name: 'Astronauts roster' })
}

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('astronaut console', () => {
  it('shows four tiles, with Astronaut-3 amber and selected for its hail', async () => {
    await loaded()
    const tiles = within(screen.getByRole('group', { name: 'Astronauts roster' })).getAllByRole('button')
    expect(tiles).toHaveLength(4)
    expect(screen.getByText(/Astronauts · 4 active · 1 needs you/i)).toBeInTheDocument()
    expect(tile('Astronaut-3')).toHaveClass('border-warning')
    expect(tile('Astronaut-3')).toHaveAttribute('aria-pressed', 'true')
    expect(within(panel()).getByText('Hail · Waiting on you')).toBeInTheDocument()
    expect(within(panel()).getAllByRole('listitem')).toHaveLength(4)
  })

  it('answers the hail and puts PAY-45 back to work on the board', async () => {
    const { router } = await openConsole()
    const reply = await screen.findByLabelText('Reply to Astronaut-3')
    await userEvent.click(within(panel()).getByRole('button', { name: 'Send' }))
    expect(within(panel()).getByRole('alert')).toHaveTextContent('Type an answer first.')

    await userEvent.type(reply, '3')
    await userEvent.click(within(panel()).getByRole('button', { name: 'Send' }))

    expect(await within(panel()).findByText(/Answer sent. Astronaut-3 is back in orbit./)).toBeInTheDocument()
    expect(screen.queryByLabelText('Reply to Astronaut-3')).not.toBeInTheDocument()
    expect(tile('Astronaut-3')).toHaveAttribute('aria-pressed', 'true')
    expect(db.hails[0]).toMatchObject({ answer: '3', answeredBy: 'u-tv' })
    expect(agent('a-3').state).toBe('working')
    expect(task('t-45').status).toBe('working')
    expect(db.activityEvents.find((e) => e.id === 'ev-2')!.needsAction).toBe(false)
    expect(db.activityEvents.at(-1)).toMatchObject({ verb: 'answered', taskId: 't-45', detail: '3' })

    await router.navigate('/projects/orbital-nav')
    expect(await screen.findByRole('link', { name: /^PAY-45: .*Working$/ })).toBeInTheDocument()
  })

  it('lets the lead set Free flight', async () => {
    await loaded()
    await userEvent.click(tile('Astronaut-2'))
    await userEvent.click(within(panel()).getByRole('radio', { name: /Free flight/ }))
    await waitFor(() => expect(agent('a-2').autonomy).toBe(2))
    expect(await within(panel()).findByText('Astronaut-2 is now Free flight.')).toBeInTheDocument()
  })

  it('locks Free flight for operators when only the lead may grant it', async () => {
    db.users.find((u) => u.id === 'u-tv')!.role = 'member'
    await loaded()
    await waitFor(() => expect(within(panel()).getByRole('radio', { name: /Free flight/ })).toBeDisabled())
    expect(within(panel()).getByText('Only the flight director can grant this.')).toBeInTheDocument()
  })

  it('recalls Astronaut-4 after a confirm step, sending PAY-43 to the Hangar', async () => {
    await loaded()
    await userEvent.click(tile('Astronaut-4'))
    await userEvent.click(within(panel()).getByRole('button', { name: 'Recall to Hangar' }))
    const confirm = within(panel()).getByRole('alertdialog')
    expect(confirm).toHaveTextContent('Recall Astronaut-4? PAY-43 goes back to the Hangar')
    expect(task('t-43').column).toBe('testing')

    await userEvent.click(within(confirm).getByRole('button', { name: 'Recall' }))
    expect(await within(panel()).findByText('PAY-43 is back in the Hangar.')).toBeInTheDocument()
    expect(task('t-43')).toMatchObject({ column: 'backlog', status: 'queued' })
    expect(agent('a-4')).toMatchObject({ state: 'idle', currentTaskId: null })
    expect(db.activityEvents.at(-1)).toMatchObject({ verb: 'recalled', taskId: 't-43' })
    expect(db.activityEvents.find((e) => e.id === 'ev-3')!.needsAction).toBe(false)
    expect(within(tile('Astronaut-4')).getByText('No current payload')).toBeInTheDocument()
    expect(within(panel()).getByRole('button', { name: 'Recall to Hangar' })).toBeDisabled()
  })

  it('pauses and resumes', async () => {
    await loaded()
    await userEvent.click(within(panel()).getByRole('button', { name: 'Pause' }))
    await waitFor(() => expect(agent('a-3').paused).toBe(true))
    expect(await within(panel()).findByRole('button', { name: 'Resume' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(tile('Astronaut-3')).getByText('Paused')).toBeInTheDocument()
    await userEvent.click(within(panel()).getByRole('button', { name: 'Resume' }))
    await waitFor(() => expect(agent('a-3').paused).toBe(false))
  })

  it('logs a redirect, which needs an instruction', async () => {
    await loaded()
    await userEvent.click(within(panel()).getByRole('button', { name: 'Redirect' }))
    const input = within(panel()).getByLabelText('New instruction for Astronaut-3')
    const send = within(input.closest('form')!).getByRole('button', { name: 'Send' })
    await userEvent.click(send)
    expect(within(panel()).getByText('Type an instruction first.')).toBeInTheDocument()

    await userEvent.type(input, 'Use a cap of 5 in tests')
    await userEvent.click(send)
    expect(await within(panel()).findByText('Instruction logged for Astronaut-3.')).toBeInTheDocument()
    expect(db.activityEvents.at(-1)).toMatchObject({ verb: 'commented', taskId: 't-45', detail: 'Redirected Astronaut-3: Use a cap of 5 in tests' })
  })

  it('shows an empty state for a mission with no astronauts', async () => {
    await openConsole('deep-field')
    expect(await screen.findByRole('heading', { name: 'No astronauts on this mission yet' })).toBeInTheDocument()
  })
})
