import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import type { ActivityEvent } from '../../data/types'
import { renderApp, renderWithProviders } from '../../test/render'
import { ActivityFeed } from './ActivityFeed'
import { dayLabel } from './feed'

const entries = () => within(screen.getByRole('list')).getAllByRole('listitem')
const filter = (name: RegExp) => screen.getByRole('button', { name })

async function openLog(project = 'orbital-nav') {
  const utils = renderApp(`/projects/${project}/activity`)
  await screen.findByRole('group', { name: /Filter the mission log/ })
  return utils
}

describe('mission log', () => {
  it('lists every event, newest first, with actor, verb and payload', async () => {
    await openLog()
    const items = entries()
    expect(items).toHaveLength(8)
    expect(items[0]).toHaveTextContent('Astronaut-1 opened a PR for PAY-41')
    expect(items[0]).toHaveTextContent('Opened PR #212 and docked for review')
    expect(items[7]).toHaveTextContent('TV created PAY-60')
    expect(filter(/^All/)).toHaveAttribute('aria-pressed', 'true')
  })

  it('filters to astronauts only', async () => {
    await openLog()
    await userEvent.click(filter(/Astronauts only/))
    expect(filter(/Astronauts only/)).toHaveAttribute('aria-pressed', 'true')
    expect(filter(/^All/)).toHaveAttribute('aria-pressed', 'false')
    const items = entries()
    expect(items).toHaveLength(4)
    items.forEach((li) => expect(li).toHaveTextContent(/Astronaut-\d/))
  })

  it('filters to operators only', async () => {
    await openLog()
    await userEvent.click(filter(/Operators only/))
    expect(entries()).toHaveLength(4)
    entries().forEach((li) => expect(li).not.toHaveTextContent(/Astronaut-\d/))
  })

  it('filters to the three entries that need action, marked in words', async () => {
    await openLog()
    await userEvent.click(filter(/Needs action/))
    const items = entries()
    expect(items).toHaveLength(3)
    expect(items.map((li) => li.textContent)).toEqual([
      expect.stringContaining('PR #212'),
      expect.stringContaining('retry cap'),
      expect.stringContaining('Comms down'),
    ])
    items.forEach((li) => expect(within(li).getByText('Needs action')).toBeInTheDocument())
  })

  it('opens the payload drawer from an entry', async () => {
    const { router } = await openLog()
    await userEvent.click(within(entries()[0]).getByRole('link', { name: 'PAY-41' }))
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/tasks/t-41')
    expect(await screen.findByRole('dialog', { name: 'Payload' })).toBeInTheDocument()
  })

  it('shows an empty state for a mission with no activity', async () => {
    renderApp('/projects/deep-field/activity')
    expect(await screen.findByText('No activity on this mission yet.')).toBeInTheDocument()
  })

  it('shows an empty state per filter', async () => {
    db.activityEvents = db.activityEvents.filter((e) => e.actor.kind === 'user')
    await openLog()
    await userEvent.click(filter(/Astronauts only/))
    expect(screen.getByText('No astronaut activity yet.')).toBeInTheDocument()
  })
})

describe('ActivityFeed', () => {
  const now = new Date('2026-11-07T15:00:00').getTime()
  const event = (id: string, at: string): ActivityEvent => ({
    id,
    projectId: 'orbital-nav',
    at: new Date(at).toISOString(),
    actor: { kind: 'user', id: 'u-tv' },
    verb: 'moved',
    taskId: null,
    detail: id,
    needsAction: false,
  })

  it('groups by day once there are older events', async () => {
    renderWithProviders(
      <ActivityFeed
        now={now}
        events={[event('a', '2026-11-07T10:00:00'), event('b', '2026-11-06T18:00:00'), event('c', '2026-11-02T09:00:00')]}
        users={db.users}
        agents={[]}
        tasks={[]}
      />,
    )
    expect(await screen.findByRole('heading', { name: 'Today' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Yesterday' })).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Mon, Nov 2' })).getByText('c')).toBeInTheDocument()
  })

  it('has no day headings when everything is from one day', async () => {
    renderWithProviders(<ActivityFeed now={now} events={[event('a', '2026-11-07T10:00:00')]} users={db.users} agents={[]} tasks={[]} />)
    expect(await screen.findByText('a')).toBeInTheDocument()
    expect(screen.queryByRole('heading')).not.toBeInTheDocument()
  })

  it('labels days', () => {
    expect(dayLabel('2026-11-07T01:00:00', now)).toBe('Today')
    expect(dayLabel('2026-11-06T23:00:00', now)).toBe('Yesterday')
    expect(dayLabel('2026-10-30T12:00:00', now)).toBe('Fri, Oct 30')
  })
})
