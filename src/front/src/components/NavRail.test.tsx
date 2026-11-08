import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fake } from '../test/fakeApi'
import { renderApp } from '../test/render'

const ada = { login: 'ada', avatarUrl: 'https://avatars.example/ada' }

describe('NavRail', () => {
  it('lists opened missions in visit order, with the open one’s pages under it', async () => {
    fake.opened = [{ id: 2, owner: 'ada', name: 'other', permission: 'read' }, fake.project]
    renderApp('/projects/1')
    const missions = within(await screen.findByRole('region', { name: 'Missions' }))

    const links = missions.getAllByRole('link').map((l) => l.textContent)
    expect(links).toEqual(['other', 'gantt', 'Mission control', 'Hangar', 'Plan a feature', 'Automation'])
    expect(missions.getByRole('link', { name: 'gantt' })).toHaveAttribute('aria-current', 'true')
  })

  it('lists eight missions, plus the open one when it is further down', async () => {
    const others = Array.from({ length: 10 }, (_, i) => ({ id: i + 2, owner: 'ada', name: `m${i + 2}`, permission: 'read' as const }))
    fake.opened = [...others, fake.project]
    renderApp('/projects/1')
    const missions = within(await screen.findByRole('region', { name: 'Missions' }))
    const names = missions.getAllByRole('listitem').map((li) => li.querySelector('a')!.textContent)
    expect(names).toEqual(['m2', 'm3', 'm4', 'm5', 'm6', 'm7', 'm8', 'm9', 'gantt'])
  })

  it('still shows the open project’s pages before the list includes it', async () => {
    renderApp('/projects/1')
    const rail = within(await screen.findByRole('navigation', { name: 'Primary' }))
    expect(await rail.findByRole('link', { name: 'Automation' })).toHaveAttribute('href', '/projects/1/automation')
    expect(rail.queryByRole('region', { name: 'Missions' })).not.toBeInTheDocument()
  })

  it('shows assigned tasks by mission with their stage, and the rocket while testing', async () => {
    fake.opened = [fake.project]
    Object.assign(fake.tasks[0], { assignees: [ada], columnId: 3, runStatus: 'running' })
    renderApp('/projects/1')
    const assigned = within(await screen.findByRole('region', { name: 'Assigned to you' }))

    expect(assigned.getByRole('link', { name: 'gantt' })).toHaveAttribute('href', '/projects/1')
    const task = assigned.getByRole('link', { name: `#12: ${fake.tasks[0].title}. Testing` })
    expect(task).toHaveAttribute('href', `/projects/1/tasks/${fake.tasks[0].id}`)
    expect(task.querySelector('svg')).toHaveClass('motion-safe:animate-liftoff')
  })

  it('says a task in User Check awaits review, and hides the section with nothing assigned', async () => {
    fake.opened = [fake.project]
    Object.assign(fake.tasks[0], { assignees: [ada], columnId: 4 })
    const { unmount } = renderApp('/projects/1')
    expect(await screen.findByRole('link', { name: /^#12: .*\. Awaiting review$/ })).toBeInTheDocument()
    unmount()

    fake.tasks[0].assignees = []
    renderApp('/projects/1')
    await screen.findByRole('region', { name: 'Missions' })
    expect(screen.queryByRole('region', { name: 'Assigned to you' })).not.toBeInTheDocument()
  })
})
