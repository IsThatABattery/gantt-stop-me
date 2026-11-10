import { focusManager } from '@tanstack/react-query'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { keys } from '../../data/keys'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'
import { setTheme } from '../../theme/theme'
import { themes } from '../../theme/themes'

async function openSettings(path = '/settings') {
  const utils = renderApp(path)
  await screen.findByRole('heading', { name: 'Settings' })
  await screen.findByText('Signed in as', { exact: false })
  return utils
}

const section = (name: string) => screen.getByRole('region', { name })

async function removeLocal(s: ReturnType<typeof within>) {
  await userEvent.click(await s.findByRole('button', { name: 'Remove Local' }))
  await userEvent.click(within(s.getByRole('alertdialog')).getByRole('button', { name: 'Remove' }))
}

describe('account', () => {
  it('shows who is signed in, and only the sections the API supports', async () => {
    await openSettings()
    expect(within(section('Account')).getByText('ada@example.com')).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Settings sections' })
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual(['Account', 'Appearance', 'GitHub', 'LLM providers', 'Agents'])
  })

  it('checks the password change here, then with the API, and confirms it', async () => {
    await openSettings()
    const s = within(section('Account'))
    await userEvent.click(s.getByRole('button', { name: 'Change password' }))
    expect(s.getByText('Enter your current password.')).toBeInTheDocument()

    await userEvent.type(s.getByLabelText('Current password'), 'wrong one')
    await userEvent.type(s.getByLabelText('New password'), 'a new password')
    await userEvent.type(s.getByLabelText('Confirm new password'), 'a new password')
    await userEvent.click(s.getByRole('button', { name: 'Change password' }))
    expect(await s.findByText('Your current password is wrong.')).toBeInTheDocument()
    expect(s.getByLabelText('Current password')).toHaveAttribute('aria-invalid', 'true')

    await userEvent.clear(s.getByLabelText('Current password'))
    await userEvent.type(s.getByLabelText('Current password'), 'correct horse')
    await userEvent.click(s.getByRole('button', { name: 'Change password' }))
    expect(await s.findByText('Password changed')).toBeInTheDocument()
    expect(fake.accounts[0].password).toBe('a new password')
    expect(fake.requests.filter((r) => r.method === 'PATCH').at(-1)?.body).toEqual({ currentPassword: 'correct horse', newPassword: 'a new password' })
  })

  it('sets a password for an account made by signing in with GitHub, with no current one, then unlinks', async () => {
    fake.accounts[0].password = null
    fake.github = { login: 'ada', kind: 'oauth' }
    await openSettings()
    const s = within(section('Account'))
    expect(s.getByRole('heading', { name: 'Set a password' })).toBeInTheDocument()
    expect(s.getByText('You sign in with GitHub. A password lets you sign in with your email too.')).toBeInTheDocument()
    expect(s.queryByLabelText('Current password')).not.toBeInTheDocument()

    await userEvent.click(s.getByRole('button', { name: 'Set a password' }))
    expect(s.getByText('Use 8 or more characters.')).toBeInTheDocument()
    expect(s.getByLabelText('New password')).toHaveFocus()
    expect(fake.requests.some((r) => r.method === 'PATCH')).toBe(false)

    await userEvent.type(s.getByLabelText('New password'), 'a new password')
    await userEvent.type(s.getByLabelText('Confirm new password'), 'a new password')
    await userEvent.click(s.getByRole('button', { name: 'Set a password' }))
    expect(await s.findByText('Password set')).toBeInTheDocument()
    expect(fake.requests.filter((r) => r.method === 'PATCH').at(-1)?.body).toEqual({ newPassword: 'a new password' })
    expect(fake.accounts[0].password).toBe('a new password')

    // It has one now: the form changes it, and GitHub can be unlinked.
    expect(await s.findByLabelText('Current password')).toHaveValue('')
    expect(s.getByRole('button', { name: 'Change password' })).toBeInTheDocument()
    expect(s.getByText('Password set')).toBeInTheDocument()
    const github = within(section('GitHub'))
    await userEvent.click(github.getByRole('button', { name: 'Unlink' }))
    await userEvent.click(github.getByRole('button', { name: 'Unlink GitHub' }))
    expect(await github.findByRole('link', { name: 'Link with GitHub' })).toBeInTheDocument()
    expect(fake.github).toBeNull()
  })

  it('turns into the change form when another session set a password first', async () => {
    fake.accounts[0].password = null
    await openSettings()
    const s = within(section('Account'))
    fake.accounts[0].password = 'set elsewhere' // another device set one; this page still says there is none
    await userEvent.type(s.getByLabelText('New password'), 'a new password')
    await userEvent.type(s.getByLabelText('Confirm new password'), 'a new password')
    await userEvent.click(s.getByRole('button', { name: 'Set a password' }))
    expect(await s.findByLabelText('Current password')).toHaveAttribute('aria-invalid', 'true')
    expect(s.getByText('Your current password is wrong.')).toBeInTheDocument()
    expect(s.getByRole('button', { name: 'Change password' })).toBeInTheDocument()
    expect(fake.accounts[0].password).toBe('set elsewhere')
  })

  it('asks to sign in with GitHub again when the session is too old to set a password, and sets none', async () => {
    fake.accounts[0].password = null
    fake.github = { login: 'ada', kind: 'oauth' }
    fake.sessionFresh = false // signed in with GitHub more than 10 minutes ago
    await openSettings()
    const s = within(section('Account'))
    await userEvent.type(s.getByLabelText('New password'), 'a new password')
    await userEvent.type(s.getByLabelText('Confirm new password'), 'a new password')
    await userEvent.click(s.getByRole('button', { name: 'Set a password' }))

    await s.findByText('Sign in with GitHub again, then set your password.')
    const status = s.getByRole('status')
    expect(status).toHaveTextContent('Sign in with GitHub again, then set your password.')
    expect(within(status).getByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github?next=/settings')
    expect(s.getByLabelText('New password')).toHaveAttribute('aria-invalid', 'false')
    expect(s.getByRole('heading', { name: 'Set a password' })).toBeInTheDocument()
    expect(fake.accounts[0].password).toBeNull()
  })

  it("shows the API's refusal of a password being set on its field", async () => {
    fake.accounts[0].password = null
    await openSettings()
    const s = within(section('Account'))
    failNext('PATCH /api/me/password', { status: 400, code: 'weak_password', message: 'That password is too common.' })
    await userEvent.type(s.getByLabelText('New password'), 'password')
    await userEvent.type(s.getByLabelText('Confirm new password'), 'password')
    await userEvent.click(s.getByRole('button', { name: 'Set a password' }))
    expect(await s.findByText('That password is too common.')).toBeInTheDocument()
    expect(s.getByLabelText('New password')).toHaveAttribute('aria-invalid', 'true')
    expect(fake.accounts[0].password).toBeNull()
  })
})

describe('appearance', () => {
  const reset = () => {
    localStorage.clear()
    delete document.documentElement.dataset.theme
  }
  beforeEach(reset)
  afterEach(reset)

  it('lists every theme, with the default picked when none is chosen', async () => {
    await openSettings()
    const s = within(section('Appearance'))
    const group = s.getByRole('group', { name: 'Theme' })
    expect(within(group).getAllByRole('radio').map((r) => r.getAttribute('value'))).toEqual(themes.map((t) => t.id))
    expect(s.getByRole('radio', { name: /^Light/ })).toBeChecked()
  })

  it('picks the theme in use, and follows a change made elsewhere', async () => {
    document.documentElement.dataset.theme = 'dark'
    await openSettings()
    const s = within(section('Appearance'))
    expect(s.getByRole('radio', { name: /^Dark/ })).toBeChecked()

    // A change made outside this section, such as one from another tab, reaches it through the store.
    act(() => setTheme('space-light'))
    expect(s.getByRole('radio', { name: /^Space light/ })).toBeChecked()
    expect(s.getByRole('radio', { name: /^Dark/ })).not.toBeChecked()
  })

  it('applies a picked theme at once and saves it in this browser', async () => {
    await openSettings()
    const s = within(section('Appearance'))
    await userEvent.click(s.getByRole('radio', { name: /^Space dark/ }))
    expect(document.documentElement.dataset.theme).toBe('space-dark')
    expect(localStorage.getItem('gsm.theme')).toBe('space-dark')
    expect(s.getByRole('radio', { name: /^Space dark/ })).toBeChecked()
    expect(s.getByRole('radio', { name: /^Light/ })).not.toBeChecked()
  })
})

describe('GitHub', () => {
  it('shows the linked account and unlinks it after a confirm step', async () => {
    await openSettings()
    const s = within(section('GitHub'))
    expect(s.getByText('ada')).toBeInTheDocument()
    expect(s.getByText('with a personal access token')).toBeInTheDocument()

    await userEvent.click(s.getByRole('button', { name: 'Unlink' }))
    expect(s.getByRole('alertdialog')).toHaveTextContent('Unlinking removes your access to every project')
    expect(s.getByRole('button', { name: 'Keep it linked' })).toHaveFocus()
    await userEvent.click(s.getByRole('button', { name: 'Unlink GitHub' }))

    expect(await s.findByRole('link', { name: 'Link with GitHub' })).toHaveAttribute('href', '/api/github/link')
    expect(fake.github).toBeNull()
  })

  it('asks an account without a password to set one before unlinking, still offering to link again', async () => {
    fake.accounts[0].password = null
    fake.github = { login: 'ada', kind: 'oauth' }
    const { router } = await openSettings()
    const s = within(section('GitHub'))
    expect(s.getByText('with a GitHub sign-in')).toBeInTheDocument()
    expect(await s.findByRole('link', { name: 'Link again with GitHub' })).toHaveAttribute('href', '/api/github/link')
    expect(s.queryByRole('button', { name: 'Unlink' })).not.toBeInTheDocument()
    expect(s.getByText(/before unlinking, so/)).toHaveTextContent('Set a password before unlinking, so you can still sign in.')

    await userEvent.click(s.getByRole('link', { name: 'Set a password' }))
    expect(router.state.location.hash).toBe('#account')
    expect(router.state.location.pathname).toBe('/settings')
  })

  it("says so when the API won't unlink because the account has no password, and catches up", async () => {
    await openSettings()
    const s = within(section('GitHub'))
    await userEvent.click(s.getByRole('button', { name: 'Unlink' }))
    // The API knows what this page doesn't yet.
    fake.accounts[0].password = null
    await userEvent.click(s.getByRole('button', { name: 'Unlink GitHub' }))

    expect(await s.findByRole('alert')).toHaveTextContent('Set a password before unlinking GitHub, so you can still sign in.')
    expect(s.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(await s.findByText(/before unlinking, so/)).toBeInTheDocument()
    expect(s.queryByRole('button', { name: 'Unlink' })).not.toBeInTheDocument()
    expect(within(section('Account')).getByRole('heading', { name: 'Set a password' })).toBeInTheDocument()
    expect(fake.github).not.toBeNull()
  })

  it('cancels the unlink confirm with Esc', async () => {
    await openSettings()
    const s = within(section('GitHub'))
    await userEvent.click(s.getByRole('button', { name: 'Unlink' }))
    await userEvent.keyboard('{Escape}')
    expect(s.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(s.getByRole('button', { name: 'Unlink' })).toHaveFocus()
  })

  it('links a personal access token, showing a refused one', async () => {
    fake.github = null
    await openSettings()
    const s = within(section('GitHub'))
    const field = await s.findByLabelText('Or use a personal access token')

    await userEvent.type(field, 'bad-token')
    await userEvent.click(s.getByRole('button', { name: 'Link token' }))
    expect(await s.findByRole('alert')).toHaveTextContent('GitHub did not accept that token.')

    await userEvent.clear(field)
    await userEvent.type(field, 'ghp_good')
    await userEvent.click(s.getByRole('button', { name: 'Link token' }))
    expect(await s.findByText('octo')).toBeInTheDocument()
    expect(fake.github).toEqual({ login: 'octo', kind: 'pat' })
  })

  it('says when the account fails to refresh after linking, and tries again', async () => {
    fake.github = null
    await openSettings()
    const s = within(section('GitHub'))
    failNext('GET /api/me', { status: 500, code: 'internal', message: 'boom' })
    await userEvent.type(await s.findByLabelText('Or use a personal access token'), 'ghp_good')
    await userEvent.click(s.getByRole('button', { name: 'Link token' }))
    expect(await s.findByRole('alert')).toHaveTextContent("Couldn't refresh your account: boom")
    await userEvent.click(s.getByRole('button', { name: 'Try again' }))
    expect(await s.findByText('octo')).toBeInTheDocument()
    expect(s.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says how the OAuth link went when GitHub sends the user back, until linking another way', async () => {
    fake.github = null
    await openSettings('/settings?github=error')
    const s = within(section('GitHub'))
    expect(s.getByRole('status')).toHaveTextContent("GitHub didn't link.")
    await userEvent.type(await s.findByLabelText('Or use a personal access token'), 'ghp_good')
    await userEvent.click(s.getByRole('button', { name: 'Link token' }))
    expect(await s.findByText('octo')).toBeInTheDocument()
    expect(s.queryByRole('status')).not.toBeInTheDocument()
  })

  it.each([
    ['error', "GitHub didn't link."],
    ['denied', 'You cancelled on GitHub.'],
  ])('says so when linking again comes back with %s, and the old link stays', async (outcome, text) => {
    fake.github = { login: 'ada', kind: 'oauth' }
    await openSettings(`/settings?github=${outcome}`)
    const s = within(section('GitHub'))
    expect(s.getByRole('status')).toHaveTextContent(text)
    expect(s.getByText('ada')).toBeInTheDocument()
  })

  describe.each([
    ['taken', 'That GitHub account is linked to another account here. Nothing changed.'],
    ['password_required', 'Set a password before linking another GitHub account, so you can still sign in.'],
  ])('when linking with GitHub comes back with %s', (outcome, text) => {
    it('says so, and the old link stays', async () => {
      fake.accounts[0].password = null
      fake.github = { login: 'ada', kind: 'oauth' }
      await openSettings(`/settings?github=${outcome}`)
      const s = within(section('GitHub'))
      expect(s.getByRole('status')).toHaveTextContent(text)
      expect(s.getByText('ada')).toBeInTheDocument()
      expect(s.getByText('with a GitHub sign-in')).toBeInTheDocument()
    })

    it('says so with none linked, until linking another way', async () => {
      fake.github = null
      await openSettings(`/settings?github=${outcome}`)
      const s = within(section('GitHub'))
      expect(s.getByRole('status')).toHaveTextContent(text)
      expect(await s.findByRole('link', { name: 'Link with GitHub' })).toBeInTheDocument()
      await userEvent.type(s.getByLabelText('Or use a personal access token'), 'ghp_good')
      await userEvent.click(s.getByRole('button', { name: 'Link token' }))
      expect(await s.findByText('octo')).toBeInTheDocument()
      expect(s.queryByRole('status')).not.toBeInTheDocument()
    })
  })

  it('says so when the user cancelled on GitHub', async () => {
    fake.github = null
    await openSettings('/settings?github=denied')
    const s = within(section('GitHub'))
    expect(s.getByRole('status')).toHaveTextContent('You cancelled on GitHub. Nothing was linked.')
    expect(await s.findByRole('link', { name: 'Link with GitHub' })).toBeInTheDocument()
  })

  it("drops the note that OAuth isn't set up once the server says it is", async () => {
    fake.github = null
    await openSettings('/settings?github=unavailable')
    const s = within(section('GitHub'))
    expect(await s.findByRole('link', { name: 'Link with GitHub' })).toBeInTheDocument()
    expect(s.queryByRole('status')).not.toBeInTheDocument()
  })

  it('asks the server again on focus, so OAuth set up meanwhile shows the button', async () => {
    fake.github = null
    fake.githubOAuth = false
    await openSettings()
    const s = within(section('GitHub'))
    await s.findByText("GitHub sign-in isn't set up on this server.")
    fake.githubOAuth = true
    try {
      act(() => {
        focusManager.setFocused(false)
        focusManager.setFocused(true)
      })
      expect(await s.findByRole('link', { name: 'Link with GitHub' })).toBeInTheDocument()
      expect(s.queryByText("GitHub sign-in isn't set up on this server.")).not.toBeInTheDocument()
    } finally {
      focusManager.setFocused(undefined)
    }
  })

  it('offers to link again with GitHub while linked, and not when OAuth is off', async () => {
    const { client } = await openSettings()
    const s = within(section('GitHub'))
    expect(await s.findByRole('link', { name: 'Link again with GitHub' })).toHaveAttribute('href', '/api/github/link')
    await userEvent.click(s.getByRole('button', { name: 'Unlink' }))
    expect(s.queryByRole('link', { name: 'Link again with GitHub' })).not.toBeInTheDocument()

    fake.githubOAuth = false
    await act(() => client.refetchQueries({ queryKey: keys.githubOAuth }))
    await userEvent.click(s.getByRole('button', { name: 'Keep it linked' }))
    expect(s.getByRole('button', { name: 'Unlink' })).toBeInTheDocument()
    expect(s.queryByRole('link', { name: 'Link again with GitHub' })).not.toBeInTheDocument()
  })

  it('says GitHub is linked when it is', async () => {
    await openSettings('/settings?github=linked')
    expect(within(section('GitHub')).getByRole('status')).toHaveTextContent('GitHub is linked.')
  })

  it('says nothing for an outcome it does not know', async () => {
    fake.github = null
    await openSettings('/settings?github=toString')
    const s = within(section('GitHub'))
    await s.findByRole('link', { name: 'Link with GitHub' })
    expect(s.queryByRole('status')).not.toBeInTheDocument()
  })

  it("offers no OAuth button when the server's OAuth app isn't set up", async () => {
    fake.github = null
    fake.githubOAuth = false
    await openSettings()
    const s = within(section('GitHub'))
    expect(await s.findByText("GitHub sign-in isn't set up on this server.")).toBeInTheDocument()
    expect(s.queryByRole('link', { name: 'Link with GitHub' })).not.toBeInTheDocument()
    expect(s.getByLabelText('Or use a personal access token')).toBeInTheDocument()
  })

  it("says it once when the user comes back because OAuth isn't set up", async () => {
    fake.github = null
    fake.githubOAuth = false
    const { client } = await openSettings('/settings?github=unavailable')
    const s = within(section('GitHub'))
    expect(s.getByRole('status')).toHaveTextContent("GitHub sign-in isn't set up on this server.")
    await waitFor(() => expect(client.getQueryState(keys.githubOAuth)?.status).toBe('success'))
    // The query tells its observers on a timer; let that render land.
    await act(() => new Promise((done) => setTimeout(done, 0)))
    expect(s.getAllByText(/sign-in isn't set up/)).toHaveLength(1)
    expect(s.queryByRole('link', { name: 'Link with GitHub' })).not.toBeInTheDocument()
  })

  it('shows neither the button nor the note until the server says whether OAuth is set up', async () => {
    fake.github = null
    let finish: (() => void) | undefined
    server.use(http.get('/api/github/oauth', () => new Promise<Response>((done) => (finish = () => done(HttpResponse.json({ enabled: false }))))))
    await openSettings()
    const s = within(section('GitHub'))
    await s.findByLabelText('Or use a personal access token')
    await waitFor(() => expect(finish).toBeDefined())
    expect(s.queryByRole('link', { name: 'Link with GitHub' })).not.toBeInTheDocument()
    expect(s.queryByText("GitHub sign-in isn't set up on this server.")).not.toBeInTheDocument()
    act(() => finish?.())
    expect(await s.findByText("GitHub sign-in isn't set up on this server.")).toBeInTheDocument()
  })

  it("offers the OAuth button when it can't tell whether OAuth is set up", async () => {
    fake.github = null
    failNext('GET /api/github/oauth', { status: 500, code: 'internal', message: 'boom' })
    await openSettings()
    const s = within(section('GitHub'))
    expect(await s.findByRole('link', { name: 'Link with GitHub' })).toHaveAttribute('href', '/api/github/link')
    expect(s.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says which token to make, and links to making it on GitHub', async () => {
    fake.github = null
    await openSettings()
    const s = within(section('GitHub'))
    const field = await s.findByLabelText('Or use a personal access token')
    expect(field).toHaveAttribute('placeholder', 'github_pat_…')
    expect(field).toHaveAccessibleDescription(
      'Use a fine-grained token. Set Resource owner to whoever owns your repositories, give it access to them, and these repository permissions:',
    )
    expect(
      within(s.getByRole('list'))
        .getAllByRole('listitem')
        .map((li) => li.textContent),
    ).toEqual([
      'Contents: Read and write',
      'Issues: Read and write',
      'Pull requests: Read and write',
      'Workflows: Read and write (to merge changes to workflow files)',
      'Metadata: Read-only (added automatically)',
    ])

    const create = s.getByRole('link', { name: 'Create a token with these permissions (opens GitHub in a new tab)' })
    expect(create).toHaveAttribute('target', '_blank')
    expect(create).toHaveAttribute('rel', 'noreferrer')
    const href = create.getAttribute('href') ?? ''
    expect(href.startsWith('https://github.com/settings/personal-access-tokens/new?')).toBe(true)
    for (const param of ['name=GanttStopMe', 'contents=write', 'issues=write', 'pull_requests=write', 'workflows=write']) {
      expect(href.split(/[?&]/)).toContain(param)
    }
    const description = new URL(href).searchParams.get('description') ?? ''
    expect(description.length).toBeGreaterThan(0)
    expect(description.length).toBeLessThanOrEqual(1024)
    expect(s.getByText(/^A classic token needs the/)).toHaveTextContent('A classic token needs the repo and workflow scopes.')
  })
})

describe('LLM providers', () => {
  it('lists providers with a masked key and their models', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    expect(await s.findByText('Local')).toBeInTheDocument()
    expect(s.getByText('•••• abcd')).toBeInTheDocument()
    expect(s.getByText('llama3')).toBeInTheDocument()
  })

  it('adds a provider, then picks its models', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Add provider' }))
    await userEvent.type(s.getByLabelText('Name'), 'OpenAI')
    await userEvent.type(s.getByLabelText('Base URL'), 'https://api.openai.com/v1')
    await userEvent.type(s.getByLabelText('API key'), 'sk-secret-1234')
    await userEvent.click(within(s.getByRole('form', { name: 'Add provider' })).getByRole('button', { name: 'Add provider' }))

    const pick = await s.findByRole('form', { name: 'Pick models for OpenAI' })
    await userEvent.type(within(pick).getByLabelText('Models'), 'qwen2{Enter}')
    await userEvent.click(within(pick).getByRole('button', { name: 'Save models' }))

    await waitFor(() => expect(fake.providers.find((p) => p.name === 'OpenAI')).toMatchObject({ keyLast4: '1234', models: ['qwen2'] }))
    expect(await s.findByText('qwen2')).toBeInTheDocument()
  })

  it('keeps the providers, and offers to try again, when refreshing them fails', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Add provider' }))
    await userEvent.type(s.getByLabelText('Name'), 'OpenAI')
    await userEvent.type(s.getByLabelText('Base URL'), 'https://api.openai.com/v1')
    await userEvent.type(s.getByLabelText('API key'), 'sk-secret-1234')
    failNext('GET /api/providers', { status: 500, code: 'internal', message: 'Database is restarting.' })
    await userEvent.click(within(s.getByRole('form', { name: 'Add provider' })).getByRole('button', { name: 'Add provider' }))

    expect(await s.findByRole('alert')).toHaveTextContent('Database is restarting.')
    expect(s.getByText('Local')).toBeInTheDocument()
    await userEvent.click(s.getByRole('button', { name: 'Try again' }))
    expect(await s.findByRole('form', { name: 'Pick models for OpenAI' })).toBeInTheDocument()
  })

  it('shows a taken name on the name field', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Add provider' }))
    await userEvent.type(s.getByLabelText('Name'), 'Local')
    await userEvent.type(s.getByLabelText('Base URL'), 'https://example.com/v1')
    await userEvent.type(s.getByLabelText('API key'), 'key')
    await userEvent.click(within(s.getByRole('form', { name: 'Add provider' })).getByRole('button', { name: 'Add provider' }))

    expect(await s.findByText('You already have a provider with this name.')).toBeInTheDocument()
    expect(s.getByLabelText('Name')).toHaveAttribute('aria-invalid', 'true')
  })

  it('edits the models of a provider', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Edit Local' }))
    const form = s.getByRole('form', { name: 'Edit Local' })
    await userEvent.type(await within(form).findByLabelText('Models'), 'mistral{Enter}')
    await userEvent.click(within(form).getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(fake.providers[0].models).toEqual(['llama3', 'mistral']))
    const saved = fake.requests.filter((r) => r.method === 'PATCH').at(-1)
    expect(saved?.body).toEqual({ models: ['llama3', 'mistral'] })
  })

  it('selects all the listed models at once', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Edit Local' }))
    const form = s.getByRole('form', { name: 'Edit Local' })
    await userEvent.click(await within(form).findByLabelText('Models'))
    await userEvent.click(within(form).getByRole('button', { name: 'Select all' }))
    expect(within(form).getAllByRole('option', { selected: true })).toHaveLength(3)
    await userEvent.click(within(form).getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(fake.providers[0].models).toEqual(['llama3', 'qwen2', 'mistral']))
  })

  for (const [what, use] of [
    ['planning', () => (fake.planning.taskAgent = { providerId: 1, model: 'llama3', instructions: '' })],
    [
      'automation',
      () =>
        (fake.automation = {
          enabled: true,
          owner: { id: 1, login: 'ada' },
          decisionProviderId: 2,
          decisionModel: 'gpt-5',
          decisionInstructions: '',
          chatProviderId: 1,
          chatModel: 'llama3',
          chatInstructions: '',
          pausedReason: null,
          payloadUrl: 'https://gantt.example.com/api/github/webhook/1',
          webhookSecretSet: false,
        }),
    ],
  ] as const) {
    it(`says why a provider ${what} uses cannot be removed`, async () => {
      use()
      await openSettings()
      const s = within(section('LLM providers'))
      await removeLocal(s)
      expect(await s.findByRole('alert')).toHaveTextContent('Automation or planning uses this provider.')
      expect(s.getByRole('button', { name: 'Remove Local' })).toHaveFocus()
      expect(fake.providers).toHaveLength(1)
    })
  }

  it('removes a provider nothing uses, after a confirm step', async () => {
    await openSettings()
    const s = within(section('LLM providers'))
    await userEvent.click(await s.findByRole('button', { name: 'Remove Local' }))
    expect(s.getByRole('alertdialog')).toHaveTextContent('Remove Local? Its saved key is deleted too.')
    await userEvent.click(s.getByRole('button', { name: 'Keep it' }))
    expect(s.getByRole('button', { name: 'Remove Local' })).toHaveFocus()
    expect(fake.providers).toHaveLength(1)

    await removeLocal(s)
    await waitFor(() => expect(fake.providers).toEqual([]))
  })
})
