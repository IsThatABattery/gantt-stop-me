import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderApp } from '../test/render'

describe('sign-in', () => {
  it('sends signed-out visitors to /login, then back where they were going', async () => {
    const { router } = renderApp('/projects/orbital-nav', { signedIn: false })
    expect(router.state.location.pathname).toBe('/login')

    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('heading', { name: 'Ship the astronaut approval flow end to end' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/orbital-nav')
  })
})

describe('board', () => {
  it('opens the payload drawer from a card', async () => {
    const { router } = renderApp('/projects/orbital-nav')
    await userEvent.click(await screen.findByRole('link', { name: /PAY-41/ }))

    expect(router.state.location.pathname).toBe('/projects/orbital-nav/tasks/t-41')
    expect(await screen.findByRole('dialog', { name: 'Payload' })).toBeInTheDocument()
  })
})
