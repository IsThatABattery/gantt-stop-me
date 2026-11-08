import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { keys } from '../data/keys'
import { fake, failNext } from '../test/fakeApi'
import { renderApp } from '../test/render'
import { server } from '../test/server'

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

})

describe('session ends mid-use', () => {
  it('asks to sign in again over the page, which keeps what was typed', async () => {
    const { router } = renderApp('/projects/1/tasks/7/edit')
    await userEvent.type(await screen.findByLabelText('Title'), ' v2')
    fake.signedInId = null // expired, or signed out in another tab
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    const dialog = await screen.findByRole('dialog', { name: 'Session ended' })
    await userEvent.type(within(dialog).getByLabelText('Password'), 'correct horse')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Session ended' })).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/projects/1/tasks/7/edit')
    expect(screen.getByLabelText('Title')).toHaveValue(`${fake.tasks[0].title} v2`)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument() // the 401 was the dialog's to explain
  })

  it('stays up on Esc, and leaves a page with unsaved edits for another account at once', async () => {
    const { router } = renderApp('/projects/1/tasks/7/edit')
    await userEvent.type(await screen.findByLabelText('Title'), ' v2')
    fake.signedInId = null
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    const dialog = await screen.findByRole('dialog', { name: 'Session ended' })

    await userEvent.keyboard('{Escape}')
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Title')).toHaveValue(`${fake.tasks[0].title} v2`)

    await userEvent.click(within(dialog).getByRole('button', { name: 'Use another account' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })

  it('keeps the drawer under it on Esc', async () => {
    const { router, client } = renderApp('/projects/1/tasks/7')
    await screen.findByRole('dialog', { name: 'Payload' })
    fake.signedInId = null
    await client.invalidateQueries({ queryKey: ['task'] })
    await screen.findByRole('dialog', { name: 'Session ended' })

    await userEvent.keyboard('{Escape}')
    expect(screen.getByRole('dialog', { name: 'Payload' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/1/tasks/7')
  })

  it('forgets the old session when another account signs in instead', async () => {
    const { router, client } = renderApp('/projects/1')
    await screen.findByRole('link', { name: /^#12:/ })
    fake.signedInId = null
    await client.invalidateQueries({ queryKey: keys.board('1') })

    await userEvent.click(await screen.findByRole('button', { name: 'Use another account' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(client.getQueryData(keys.board('1'))).toBeUndefined()
  })

  it('keeps the page when a background /api/me refetch fails', async () => {
    const { client } = renderApp('/projects')
    await screen.findByRole('heading', { name: 'All missions' })
    server.use(http.get('/api/me', () => HttpResponse.json({ error: { code: 'bad_gateway', message: 'Bad gateway' } }, { status: 502 }), { once: true }))
    await client.refetchQueries({ queryKey: keys.me })
    await new Promise((r) => setTimeout(r, 50))

    expect(screen.getByRole('heading', { name: 'All missions' })).toBeInTheDocument()
    expect(screen.queryByText('Bad gateway')).not.toBeInTheDocument()
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

  it('leaves a page with unsaved edits, as a choice to leave', async () => {
    const { router } = renderApp('/projects/1/tasks/7/edit')
    await userEvent.type(await screen.findByLabelText('Title'), ' v2')
    await userEvent.click(screen.getByRole('button', { name: /Account menu, signed in as ada@example.com/ }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Sign out' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(screen.queryByRole('dialog', { name: 'Session ended' })).not.toBeInTheDocument()
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

    expect(await screen.findByRole('alert')).toHaveTextContent('An account with this email already exists.')
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true')

    await userEvent.type(screen.getByLabelText('Email'), 'u')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'false')
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

  it('says a project the user cannot see is not found, with no pages for it in the rail', async () => {
    const { client } = renderApp('/projects/2')
    expect(await screen.findByRole('heading', { name: 'Mission not found' })).toBeInTheDocument()
    expect(within(screen.getByRole('main')).getByRole('link', { name: 'Add a mission' })).toHaveAttribute('href', '/projects/new')
    await waitFor(() => expect(client.getQueryState(keys.projects)?.status).toBe('success'))
    expect(screen.queryByRole('link', { name: 'Automation' })).not.toBeInTheDocument()
  })

  it('keeps the page when a refetch of the project fails, and offers Try again when the first load does', async () => {
    failNext('GET /api/projects/1/board', { status: 500, code: 'internal', message: 'Database is restarting.' })
    const { client } = renderApp('/projects/1/automation')
    expect(await screen.findByText('Database is restarting.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByRole('heading', { name: 'Automation' })).toBeInTheDocument()

    failNext('GET /api/projects/1/board', { status: 500, code: 'internal', message: 'Database is restarting.' })
    await client.invalidateQueries({ queryKey: keys.board('1') })
    await waitFor(() => expect(client.getQueryState(keys.board('1'))?.status).toBe('error'))
    expect(screen.getByRole('heading', { name: 'Automation' })).toBeInTheDocument()
    expect(screen.queryByText('Database is restarting.')).not.toBeInTheDocument()
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
