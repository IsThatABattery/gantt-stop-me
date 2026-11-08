import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { clearFavoritesCache } from '../../data/views/projects'
import { renderApp } from '../../test/render'

/** Mission names on the grid, in order. */
async function missionNames() {
  const grid = await screen.findByRole('region', { name: 'Missions' })
  return within(grid)
    .queryAllByRole('link')
    .map((a) => a.querySelector('.font-heading')?.textContent)
}

const filter = (name: string) => userEvent.click(screen.getByRole('button', { name }))

beforeEach(() => {
  localStorage.clear()
  clearFavoritesCache()
})

describe('mission select', () => {
  it('shows every mission, most recent first', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    expect(await missionNames()).toEqual(['Orbital-Nav', 'Lunar Relay', 'Ground Station', 'Deep Field'])
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('filters to missions the signed-in operator is on', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Mine')

    expect(screen.getByRole('button', { name: 'Mine' })).toHaveAttribute('aria-pressed', 'true')
    expect(await missionNames()).toEqual(['Orbital-Nav', 'Lunar Relay', 'Deep Field'])
  })

  it('stars a mission and shows it under Favorites', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Favorites')
    expect(screen.getByText(/No favorite missions yet/)).toBeInTheDocument()

    await filter('All')
    const star = screen.getByRole('button', { name: 'Favorite Lunar Relay' })
    await userEvent.click(star)
    expect(star).toHaveAttribute('aria-pressed', 'true')
    await filter('Favorites')

    expect(await missionNames()).toEqual(['Lunar Relay'])
    expect(JSON.parse(localStorage.getItem('gsm.favorites.u-tv')!)).toEqual(['lunar-relay'])
  })

  it('combines filters with search', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await filter('Mine')
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search missions' }), 'station')

    expect(await missionNames()).toEqual([])
    expect(screen.getByText('No missions match “station”.')).toBeInTheDocument()
  })

  it('sorts by name and by launch window end', async () => {
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    const sort = screen.getByRole('combobox', { name: 'Sort by' })

    await userEvent.selectOptions(sort, 'Name')
    expect(await missionNames()).toEqual(['Deep Field', 'Ground Station', 'Lunar Relay', 'Orbital-Nav'])

    await userEvent.selectOptions(sort, 'Launch window end')
    expect(await missionNames()).toEqual(['Ground Station', 'Orbital-Nav', 'Lunar Relay', 'Deep Field'])
  })

  it('shows archived missions only under Archived', async () => {
    db.projects.find((p) => p.id === 'ground-station')!.archived = true
    renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    expect(await missionNames()).not.toContain('Ground Station')

    await filter('Archived')
    expect(await missionNames()).toEqual(['Ground Station'])
  })

  it('Review now opens the mission that needs attention most', async () => {
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

  it('creates a mission and opens its empty board', async () => {
    const { router } = renderApp('/')
    await screen.findByRole('link', { name: /^Orbital-Nav/ })
    await userEvent.click(screen.getAllByRole('button', { name: /New mission/ })[0])
    const dialog = screen.getByRole('dialog', { name: 'New mission' })

    await userEvent.click(within(dialog).getByRole('button', { name: 'Create mission' }))
    expect(within(dialog).getByText('Give the mission a name.')).toBeInTheDocument()

    await userEvent.type(within(dialog).getByLabelText('Name'), 'Test Mission')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create mission' }))

    expect(await screen.findByRole('heading', { name: 'Test Mission' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/test-mission')
    expect(db.projects.find((p) => p.id === 'test-mission')).toMatchObject({ memberIds: ['u-tv'], activeSprintId: null })

    await router.navigate('/')
    expect(await screen.findByRole('link', { name: /^Test Mission/ })).toBeInTheDocument()
  })
})
