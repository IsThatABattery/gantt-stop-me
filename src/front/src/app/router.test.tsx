import { act, screen, waitFor, within } from '@testing-library/react'
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

describe('Continue with GitHub', () => {
  const github = () => screen.queryByRole('link', { name: 'Continue with GitHub' })

  /** Until the OAuth query's answer has rendered */
  async function settled(client: ReturnType<typeof renderApp>['client']) {
    await waitFor(() => expect(client.getQueryState(keys.githubOAuth)?.status).not.toBe('pending'))
    // The query tells its observers on a timer; let that render land.
    await act(() => new Promise((done) => setTimeout(done, 0)))
  }

  it("signs in with GitHub when the server's OAuth app is set up, asking without a session", async () => {
    const { client } = renderApp('/login', { signedIn: false })
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github')
    expect(screen.getByRole('button', { name: 'Continue with SSO' })).toBeInTheDocument()
    expect(client.getQueryState(keys.githubOAuth)?.status).toBe('success')
    expect(client.getQueryData(keys.me)).toBeNull()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it("offers no GitHub sign-in when the server's OAuth app isn't set up", async () => {
    fake.githubOAuth = false
    const { client } = renderApp('/login', { signedIn: false })
    await screen.findByLabelText('Email')
    await settled(client)
    expect(github()).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue with SSO' })).toBeInTheDocument()
    expect(screen.getByText('or with email')).toBeInTheDocument()
  })

  it('holds its space, empty, until the server says whether OAuth is set up', async () => {
    let finish: (() => void) | undefined
    server.use(http.get('/api/github/oauth', () => new Promise<Response>((done) => (finish = () => done(HttpResponse.json({ enabled: true }))))))
    renderApp('/login', { signedIn: false })
    await screen.findByLabelText('Email')
    await waitFor(() => expect(finish).toBeDefined())
    expect(github()).not.toBeInTheDocument()
    const held = screen.getByRole('link', { name: 'Continue with GitHub', hidden: true }).parentElement
    expect(held).toHaveClass('invisible')
    expect(held).toHaveAttribute('inert')
    act(() => finish?.())
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toBeInTheDocument()
    expect(held).not.toHaveClass('invisible')
    expect(held).not.toHaveAttribute('inert')
  })

  it("offers it when it can't tell whether OAuth is set up", async () => {
    failNext('GET /api/github/oauth', { status: 500, code: 'internal', message: 'boom' })
    const { client } = renderApp('/login', { signedIn: false })
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github')
    await settled(client)
    expect(github()).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  for (const [outcome, text] of [
    ['denied', "You cancelled on GitHub. You're not signed in."],
    ['error', "GitHub sign-in didn't work. Try again, or sign in with your email."],
    ['unavailable', "GitHub sign-in isn't set up on this server. Sign in with your email."],
    ['account_exists', 'An account already uses your GitHub email. Sign in with its password, then link GitHub in Settings.'],
    ['no_email', 'Your GitHub account has no verified email. Verify one on GitHub, or sign up with your email.'],
    ['ambiguous', 'More than one account here is linked to that GitHub account. Sign in with your email.'],
  ]) {
    it(`says why when GitHub sends the person back with ?github=${outcome}`, async () => {
      // It sends unavailable only while OAuth isn't set up.
      if (outcome === 'unavailable') fake.githubOAuth = false
      const { client } = renderApp(`/login?github=${outcome}`, { signedIn: false })
      expect(await screen.findByRole('status')).toHaveTextContent(text)
      await settled(client)
      expect(screen.getByRole('status')).toHaveTextContent(text)
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  }

  it('says nothing for an outcome it does not know', async () => {
    for (const outcome of ['toString', 'linked', '']) {
      const { client, unmount } = renderApp(`/login?github=${outcome}`, { signedIn: false })
      await screen.findByRole('link', { name: 'Continue with GitHub' })
      await settled(client)
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
      unmount()
    }
  })

  it("drops the note that OAuth isn't set up once the server says it is", async () => {
    const { client } = renderApp('/login?github=unavailable', { signedIn: false })
    await settled(client)
    expect(github()).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('still signs in with email and password under a note', async () => {
    const { router } = renderApp('/login?github=account_exists', { signedIn: false })
    await signIn('ada@example.com', 'correct horse')
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(fake.signedInId).toBe(1)
  })

  it('refuses a password for an account made with GitHub, as a wrong one', async () => {
    fake.accounts.push({ id: 2, email: 'grace@example.com', password: null })
    renderApp('/login', { signedIn: false })
    await signIn('grace@example.com', 'anything at all')
    expect(await screen.findByRole('alert')).toHaveTextContent('Wrong email or password.')
    expect(fake.signedInId).toBeNull()
  })

  it('offers to sign up with GitHub, which links it at once', async () => {
    renderApp('/signup', { signedIn: false })
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github')
    expect(screen.getByText('Signs you up with your GitHub email, and links GitHub at once.')).toBeInTheDocument()
    expect(screen.getByText('or with email')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create account' })).toBeInTheDocument()
  })

  it("offers sign up with email alone when the server's OAuth app isn't set up", async () => {
    fake.githubOAuth = false
    const { client } = renderApp('/signup', { signedIn: false })
    await screen.findByLabelText('Email')
    await settled(client)
    expect(github()).not.toBeInTheDocument()
    expect(screen.queryByText(/links GitHub at once/)).not.toBeInTheDocument()
    expect(screen.queryByText('or with email')).not.toBeInTheDocument()
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
    // An account made with GitHub has no password: Continue with GitHub must come up without touching the page.
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).not.toHaveAttribute('aria-hidden')
  })

  it('keeps the drawer under it on Esc', async () => {
    const { router, client } = renderApp('/projects/1/tasks/7')
    await screen.findByRole('dialog', { name: 'Task' })
    fake.signedInId = null
    await client.invalidateQueries({ queryKey: ['task'] })
    await screen.findByRole('dialog', { name: 'Session ended' })

    await userEvent.keyboard('{Escape}')
    expect(screen.getByRole('dialog', { name: 'Task' })).toBeInTheDocument()
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
    await screen.findByRole('heading', { name: 'All projects' })
    server.use(http.get('/api/me', () => HttpResponse.json({ error: { code: 'bad_gateway', message: 'Bad gateway' } }, { status: 502 }), { once: true }))
    await client.refetchQueries({ queryKey: keys.me })
    await new Promise((r) => setTimeout(r, 50))

    expect(screen.getByRole('heading', { name: 'All projects' })).toBeInTheDocument()
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
    await userEvent.click(await screen.findByRole('link', { name: 'Create an account' }))
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
  it('opens the task drawer from a card', async () => {
    const { router } = renderApp('/projects/1')
    await userEvent.click(await screen.findByRole('link', { name: /^#12:/ }))

    expect(router.state.location.pathname).toBe('/projects/1/tasks/7')
    expect(await screen.findByRole('dialog', { name: 'Task' })).toBeInTheDocument()
  })

  it('asks to link GitHub when no account is linked', async () => {
    fake.github = null
    renderApp('/projects/1')
    expect(await screen.findByRole('heading', { name: 'Link GitHub first' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Link GitHub' })).toHaveAttribute('href', '/settings#github')
  })

  it('says a project the user cannot see is not found, with no pages for it in the rail', async () => {
    const { client } = renderApp('/projects/2')
    expect(await screen.findByRole('heading', { name: 'Project not found' })).toBeInTheDocument()
    expect(within(screen.getByRole('main')).getByRole('link', { name: 'Add a project' })).toHaveAttribute('href', '/projects/new')
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

  it('shows not found for a project path with no page', async () => {
    for (const path of ['agents', 'activity', 'new/feature']) {
      const { unmount } = renderApp(`/projects/1/${path}`)
      expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
      unmount()
    }
  })

  it('offers the board, the Backlog, planning and automation in the rail', async () => {
    renderApp('/projects/1')
    const rail = await screen.findByRole('navigation', { name: 'Primary' })
    // The project's pages show once the projects list answered, so they don't jump into it.
    await within(rail).findByRole('link', { name: 'Automation' })
    expect(Array.from(rail.querySelectorAll('a')).map((a) => a.textContent)).toEqual(['GanttStopMe', 'Board', 'Backlog', 'Plan a feature', 'Automation', 'All projects', 'Add a project', 'Settings'])
  })
})
