import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { clearFavoritesCache } from '../../data/views/projects'
import { renderApp } from '../../test/render'

/** Project names on the grid, in order. */
async function projectNames() {
  const grid = await screen.findByRole('region', { name: 'Projects' })
  return within(grid)
    .queryAllByRole('link')
    .map((a) => a.querySelector('.')?.textContent)
}

const filter = (name: string) => userEvent.click(screen.getByRole('button', { name }))

beforeEach(() => {
  localStorage.clear()
  clearFavoritesCache()
})

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('project select', () => {
  it('shows every project, most recent first', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    expect(await projectNames()).toEqual(['Orbital-Nav', 'Lunar Relay'])
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('filters to projects the signed-in person is on', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Mine')

    expect(screen.getByRole('button', { name: 'Mine' })).toHaveAttribute('aria-pressed', 'true')
    expect(await projectNames()).toEqual(['Orbital-Nav', 'Lunar Relay'])
  })

  it('stars a project and shows it under Favorites', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Favorites')
    expect(screen.getByText(/No favorite projects yet/)).toBeInTheDocument()

    await filter('All')
    const star = screen.getByRole('button', { name: 'Favorite Lunar Relay' })
    await userEvent.click(star)
    expect(star).toHaveAttribute('aria-pressed', 'true')
    await filter('Favorites')

    expect(await projectNames()).toEqual(['Lunar Relay'])
    expect(JSON.parse(localStorage.getItem('gsm.favorites.u-tv')!)).toEqual(['lunar-relay'])
  })

  it('combines filters with search', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Mine')
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search projects' }), 'relay')
    expect(await projectNames()).toEqual(['Lunar Relay'])

    await userEvent.type(screen.getByRole('searchbox', { name: 'Search projects' }), ' nope')
    expect(await projectNames()).toEqual([])
    expect(screen.getByText('No projects match “relay nope”.')).toBeInTheDocument()
  })

  it('sorts by name and by sprint end', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    const sort = screen.getByRole('combobox', { name: 'Sort by' })

    await userEvent.selectOptions(sort, 'Name')
    expect(await projectNames()).toEqual(['Lunar Relay', 'Orbital-Nav'])

    await userEvent.selectOptions(sort, 'Sprint end')
    expect(await projectNames()).toEqual(['Orbital-Nav', 'Lunar Relay'])
  })

  it('shows archived projects only under Archived', async () => {
    db.projects.find((p) => p.id === 'lunar-relay')!.archived = true
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    expect(await projectNames()).not.toContain('Lunar Relay')

    await filter('Archived')
    expect(await projectNames()).toEqual(['Lunar Relay'])
  })

  it('Review now opens the project that needs attention most', async () => {
    const { router } = renderApp('/')
    await userEvent.click(await screen.findByRole('link', { name: /Review now/ }))
    expect(router.state.location.pathname).toBe('/projects/orbital-nav')
  })

  it('hides Needs you when nothing needs attention', async () => {
    Object.values(db.projectStats).forEach((s) => Object.assign(s, { awaitingApproval: 0, hails: 0, blocked: 0 }))
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    expect(screen.queryByRole('region', { name: 'Needs your attention' })).not.toBeInTheDocument()
  })

  it('creates a project and opens its empty board', async () => {
    const { router } = renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await userEvent.click(screen.getAllByRole('button', { name: /New project/ })[0])
    const dialog = screen.getByRole('dialog', { name: 'New project' })

    await userEvent.click(within(dialog).getByRole('button', { name: 'Create project' }))
    expect(within(dialog).getByText('Give the project a name.')).toBeInTheDocument()

    await userEvent.type(within(dialog).getByLabelText('Name'), 'Test Mission')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create project' }))

    expect(await screen.findByRole('heading', { name: 'Test Mission' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/test-mission')
    expect(db.projects.find((p) => p.id === 'test-mission')).toMatchObject({ memberIds: ['u-tv'], activeSprintId: null })

    await router.navigate('/')
    expect(await screen.findByRole('link', { name: /^Test Mission/ })).toBeInTheDocument()
  })
})
