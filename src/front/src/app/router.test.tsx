import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake } from '../test/fakeApi'
import { renderApp } from '../test/render'

async function signIn(email: string, password: string) {
  await userEvent.type(await screen.findByLabelText('Email'), email)
  await userEvent.type(screen.getByLabelText('Password'), password)
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))
}

describe('sign in', () => {
  it('sends signed-out visitors to /login, then back where they were going', async () => {
    const { router } = renderApp('/projects/1', { signedIn: false })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    await signIn('ada@example.com', 'correct horse')

    expect(await screen.findByRole('heading', { name: 'gantt' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/1')
    expect(fake.signedInId).toBe(1)
  })

  it('says so when the password is wrong', async () => {
    renderApp('/login', { signedIn: false })
    await signIn('ada@example.com', 'nope')
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong email or password.')
  })

  it('goes back to login when the session ends mid-use', async () => {
    const { router } = renderApp('/projects/1')
    await screen.findByRole('link', { name: /^#12:/ })
    fake.signedInId = null // expired, or signed out in another tab

    await userEvent.click(screen.getByRole('link', { name: /^#12:/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })
})

describe('sign out', () => {
  it('ends the session from the account menu', async () => {
    const { router } = renderApp('/')
    await userEvent.click(await screen.findByRole('button', { name: /Account menu, signed in as ada@example.com/ }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Sign out' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(fake.signedInId).toBeNull()
  })
})

describe('sign up', () => {
  it('creates an account and lands on adding a project, asking to link GitHub', async () => {
    const { router } = renderApp('/login', { signedIn: false })
    await userEvent.click(await screen.findByRole('link', { name: 'Request access' }))
    expect(router.state.location.pathname).toBe('/signup')

    await userEvent.type(screen.getByLabelText('Email'), 'grace@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'long enough')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByRole('heading', { name: 'Link GitHub first' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/')
    expect(fake.accounts.map((a) => a.email)).toContain('grace@example.com')
  })

  it('shows the API refusal on its field', async () => {
    renderApp('/signup', { signedIn: false })
    await userEvent.type(await screen.findByLabelText('Email'), 'ada@example.com')
    await userEvent.type(screen.getByLabelText('Password'), 'long enough')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByText('An account with this email already exists.')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('project routes', () => {
  it('opens the payload drawer from a card', async () => {
    const { router } = renderApp('/projects/1')
    await userEvent.click(await screen.findByRole('link', { name: /^#12:/ }))

    expect(router.state.location.pathname).toBe('/projects/1/tasks/7')
    expect(await screen.findByRole('dialog', { name: 'Payload' })).toBeInTheDocument()
  })

  it('asks to link GitHub when no account is linked', async () => {
    fake.github = null
    renderApp('/projects/1')
    expect(await screen.findByRole('heading', { name: 'Link GitHub first' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Link GitHub' })).toHaveAttribute('href', '/settings#github')
  })

  it('says a project the user cannot see is not found', async () => {
    renderApp('/projects/2')
    expect(await screen.findByRole('heading', { name: 'Mission not found' })).toBeInTheDocument()
  })

  it('does not route the screens still on mock data', async () => {
    for (const path of ['agents', 'activity', 'new/feature']) {
      const { unmount } = renderApp(`/projects/1/${path}`)
      expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
      unmount()
    }
  })

  it('offers the board, the Hangar, planning and automation in the rail', async () => {
    renderApp('/projects/1')
    const rail = await screen.findByRole('navigation', { name: 'Primary' })
    // The project's pages show once the missions list answered, so they don't jump into it.
    await within(rail).findByRole('link', { name: 'Automation' })
    expect(Array.from(rail.querySelectorAll('a')).map((a) => a.textContent)).toEqual(['GanttStopMe', 'Add a mission', 'All missions', 'Mission control', 'Hangar', 'Plan a feature', 'Automation', 'Settings'])
  })
})
