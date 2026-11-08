import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'

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
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual(['Account', 'GitHub', 'LLM providers', 'Agents'])
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
})

describe('GitHub', () => {
  it('shows the linked account and unlinks it after a confirm step', async () => {
    await openSettings()
    const s = within(section('GitHub'))
    expect(s.getByText('ada')).toBeInTheDocument()
    expect(s.getByText('with a personal access token')).toBeInTheDocument()

    await userEvent.click(s.getByRole('button', { name: 'Unlink' }))
    expect(s.getByRole('alertdialog')).toHaveTextContent('Unlinking removes your access to every mission')
    expect(s.getByRole('button', { name: 'Keep it linked' })).toHaveFocus()
    await userEvent.click(s.getByRole('button', { name: 'Unlink GitHub' }))

    expect(await s.findByRole('link', { name: 'Link with GitHub' })).toHaveAttribute('href', '/api/github/link')
    expect(fake.github).toBeNull()
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
