import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { Automation, Provider } from '../../data/api/types'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'
import { server } from '../../test/server'

const PATH = '/projects/1/automation'

const providers: Provider[] = [
  { id: 1, name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', keyLast4: 'abcd', models: ['gpt-6-luna', 'gpt-5'] },
  { id: 2, name: 'Local', baseUrl: 'http://llm.lan/v1', keyLast4: 'wxyz', models: ['llama'] },
]

// ada, whom renderApp signs in as, is user 1.
const enabled: Automation = {
  enabled: true,
  owner: { id: 1, login: 'ada' },
  decisionProviderId: 1,
  decisionModel: 'gpt-6-luna',
  decisionInstructions: '',
  chatProviderId: 2,
  chatModel: 'llama',
  chatInstructions: 'Use the staging login.',
  pausedReason: null,
  payloadUrl: 'https://gantt.example.com/api/github/webhook/1',
  webhookSecretSet: false,
}

const sent = (method: string) => fake.requests.filter((r) => r.method === method && r.path === `/api${PATH}`)

async function openAutomation(automation: Automation | null) {
  fake.providers = providers
  fake.automation = automation
  renderApp(PATH)
  await screen.findByRole('heading', { name: 'Automation', level: 1 })
  const agent = (name: string) => within(screen.getByRole('group', { name }))
  return { agent }
}

describe('project automation', () => {
  it('is in the nav rail', async () => {
    await openAutomation(enabled)
    expect(screen.getByRole('link', { name: 'Automation' })).toHaveAttribute('href', PATH)
  })

  it('shows automation that is on and saves both agents', async () => {
    const { agent } = await openAutomation(enabled)
    expect(await screen.findByText('Automation is on.')).toBeInTheDocument()
    expect(screen.getByText(/Owner: ada\./)).toBeInTheDocument()
    expect(agent('Decision agent').getByLabelText('Model')).toHaveDisplayValue('gpt-6-luna')
    expect(agent('Check agent').getByLabelText('Your instructions')).toHaveValue('Use the staging login.')
    expect(screen.queryByText(/Saving makes you the owner/)).not.toBeInTheDocument()

    await userEvent.selectOptions(agent('Decision agent').getByLabelText('Model'), 'gpt-5')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByText('Automation saved')).toBeInTheDocument()
    expect(sent('PUT').map((r) => r.body)).toEqual([
      {
      decisionProviderId: 1,
      decisionModel: 'gpt-5',
      decisionInstructions: '',
      chatProviderId: 2,
      chatModel: 'llama',
      chatInstructions: 'Use the staging login.',
      },
    ])
  })

  it("warns that saving takes over someone else's automation, and shows why it's paused", async () => {
    await openAutomation({ ...enabled, owner: { id: 7, login: 'bob' }, pausedReason: 'GitHub refused the owner’s token.' })
    const warning = await screen.findByText(/Saving makes you the owner instead of bob\./)
    expect(screen.getByRole('button', { name: 'Save changes' })).toHaveAccessibleDescription(warning.parentElement!.textContent!)
    expect(screen.getByText(/Paused: GitHub refused the owner’s token\./)).toBeInTheDocument()
  })

  it('turns on from off, and needs both models first', async () => {
    const { agent } = await openAutomation({ enabled: false })
    expect(await screen.findByText('Automation is off.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Turn off automation' })).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Turn on automation' }))
    expect(agent('Decision agent').getByLabelText('Provider')).toHaveAccessibleDescription('Choose a provider.')
    expect(agent('Check agent').getByLabelText('Provider')).toHaveAccessibleDescription('Choose a provider.')
    expect(sent('PUT')).toEqual([])
  })

  it('starts empty when taking over on providers the caller does not have', async () => {
    const { agent } = await openAutomation({ ...enabled, owner: { id: 7, login: 'bob' }, decisionProviderId: 9, chatProviderId: 9 })
    await screen.findByText('Automation is on.')
    expect(screen.getByText('Decision agent: gpt-6-luna. Check agent: llama.')).toBeInTheDocument()
    for (const name of ['Decision agent', 'Check agent']) {
      expect(agent(name).getByLabelText('Provider')).toHaveDisplayValue('Choose a provider')
      expect(agent(name).getByLabelText('Model')).toHaveDisplayValue('None')
    }
    expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
  })

  it('says when the owner has unlinked GitHub', async () => {
    await openAutomation({ ...enabled, owner: { id: 7, login: null } })
    expect(await screen.findByText(/Owner: none, they unlinked GitHub\./)).toBeInTheDocument()
    expect(screen.getByText(/Saving makes you the owner\./)).toBeInTheDocument()
  })

  it("says so when the server can't run automation or the project isn't found", async () => {
    await openAutomation(null)
    expect(await screen.findByText("Automation isn't available for this project")).toBeInTheDocument()
  })

  for (const [what, method] of [
    ['Save', 'PUT'],
    ['Turn off', 'DELETE'],
  ]) {
    it(`goes read-only after a 403 on ${what}`, async () => {
      // Write access was lost on GitHub after the page loaded.
      const { agent } = await openAutomation({ ...enabled, owner: { id: 7, login: 'bob' } })
      await screen.findByText('Automation is on.')
    await userEvent.type(agent('Check agent').getByLabelText('Your instructions'), ' Then log out.')
      failNext(`${method} /api${PATH}`, { status: 403, code: 'forbidden', message: 'You need write access.' })
      if (method === 'PUT') await userEvent.click(await screen.findByRole('button', { name: 'Save changes' }))
      else {
        await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
        await userEvent.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Turn off' }))
      }
      expect(await screen.findByText(/You have read access only/)).toHaveFocus()
      expect(agent('Decision agent').getByLabelText('Model')).toBeDisabled()
      // It shows what's saved, not the edits.
      expect(agent('Check agent').getByLabelText('Your instructions')).toHaveValue('Use the staging login.')
      expect(agent('Check agent').getByLabelText('Your instructions')).toBeDisabled()
      // The project, with its permission, is refetched.
      expect(fake.requests.filter((r) => r.method === 'GET' && r.path === '/api/projects/1/board')).toHaveLength(2)
      expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Turn off/ })).not.toBeInTheDocument()
      expect(screen.queryByText(/Saving makes you the owner/)).not.toBeInTheDocument()
    })
  }

  it('is read-only up front with read access to the project', async () => {
    fake.project.permission = 'read'
    const { agent } = await openAutomation(enabled)
    expect(await screen.findByText(/You have read access only/)).toBeInTheDocument()
    expect(agent('Decision agent').getByLabelText('Provider')).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Turn off automation' })).not.toBeInTheDocument()
  })

  it('keeps the form and its edits when refetching providers fails', async () => {
    const { agent } = await openAutomation(enabled)
    await screen.findByText('Automation is on.')
    await userEvent.type(agent('Check agent').getByLabelText('Your instructions'), ' Then log out.')
    failNext(`PUT /api${PATH}`, { status: 400, code: 'invalid_model', message: 'Pick a chosen model.' })
    failNext('GET /api/providers', { status: 500, code: 'internal', message: 'Something broke.' })
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(await screen.findByText('Pick a chosen model.')).toBeInTheDocument()
    expect(agent('Check agent').getByLabelText('Your instructions')).toHaveValue('Use the staging login. Then log out.')
  })

  it("doesn't carry one project's read-only state to another", async () => {
    server.use(
      http.get('/api/projects/2/board', () => HttpResponse.json({ project: { ...fake.project, id: 2, name: 'second' }, columns: fake.columns, tasks: [] })),
      http.get('/api/projects/2/automation', () => HttpResponse.json(enabled)),
    )
    fake.providers = providers
    fake.automation = enabled
    const { router } = renderApp(PATH)
    const visit = async (path: string, name: string) => {
      await act(() => router.navigate(path))
      await screen.findByText(`Mission · ${name}`)
      await screen.findByRole('form', { name: 'Automation' })
    }
    // Both projects are cached, so going back and forth keeps the page mounted.
    await visit('/projects/2/automation', 'second')
    await visit(PATH, 'gantt')
    failNext(`PUT /api${PATH}`, { status: 403, code: 'forbidden', message: 'You need write access.' })
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await screen.findByText(/You have read access only/)

    await visit('/projects/2/automation', 'second')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
    expect(screen.queryByText(/You have read access only/)).not.toBeInTheDocument()
  })

  it('places a 400 against fresh providers and focuses that field', async () => {
    const { agent } = await openAutomation(enabled)
    await screen.findByText('Automation is on.')
    // gpt-6-luna is retired between loading the page and saving it, so the fake API refuses it.
    fake.providers = [{ ...providers[0], models: ['gpt-5'] }, providers[1]]
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    const model = agent('Decision agent').getByLabelText('Model')
    await expect.poll(() => model.getAttribute('aria-invalid')).toBe('true')
    expect(model).toHaveAccessibleDescription("Pick one of this provider's chosen models.")
    expect(model).toHaveFocus()
  })

  it('focuses the first field missing a model', async () => {
    const { agent } = await openAutomation({ enabled: false })
    await userEvent.click(await screen.findByRole('button', { name: 'Turn on automation' }))
    expect(agent('Decision agent').getByLabelText('Provider')).toHaveFocus()
  })

  it('moves focus into the turn-off confirm and back out', async () => {
    await openAutomation(enabled)
    await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
    const cancel = within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Cancel' })
    expect(cancel).toHaveFocus()
    await userEvent.click(cancel)
    expect(screen.getByRole('button', { name: 'Turn off automation' })).toHaveFocus()
  })

  it("can't save while turning off", async () => {
    let finish = () => {}
    server.use(http.delete(`/api${PATH}`, () => new Promise<Response>((done) => (finish = () => done(new Response(null, { status: 204 }))))))
    await openAutomation(enabled)
    await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
    await userEvent.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Turn off' }))
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
    // Cancel would close the confirm and focus the disabled Turn off automation.
    expect(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Cancel' })).toBeDisabled()
    finish()
  })

  for (const fromConfirm of [false, true]) {
    it(`can't turn off while saving${fromConfirm ? ', from the confirm' : ''}`, async () => {
      let finish = () => {}
      server.use(http.put(`/api${PATH}`, () => new Promise<Response>((done) => (finish = () => done(HttpResponse.json(enabled))))))
      await openAutomation(enabled)
      if (fromConfirm) await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
      await userEvent.click(await screen.findByRole('button', { name: 'Save changes' }))
      expect(screen.getByRole('button', { name: fromConfirm ? 'Turn off' : 'Turn off automation' })).toBeDisabled()
      finish()
    })
  }

  it('warns before leaving with unsaved changes', async () => {
    const { agent } = await openAutomation(enabled)
    await screen.findByText('Automation is on.')
    await userEvent.type(agent('Check agent').getByLabelText('Your instructions'), ' Then log out.')
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: 'Add a mission' }))
    const dialog = within(await screen.findByRole('alertdialog'))
    expect(dialog.getByText(/You have unsaved changes/)).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Stay' })).toHaveFocus()
    await userEvent.click(dialog.getByRole('button', { name: 'Leave without saving' }))
    expect(await screen.findByRole('heading', { name: 'Add a mission', level: 1 })).toBeInTheDocument()
  })

  it('leaves without asking once the changes are saved', async () => {
    const { agent } = await openAutomation(enabled)
    await screen.findByText('Automation is on.')
    await userEvent.type(agent('Check agent').getByLabelText('Your instructions'), ' Then log out.')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await screen.findByText('Automation saved')
    await userEvent.click(screen.getByRole('link', { name: 'Add a mission' }))
    expect(await screen.findByRole('heading', { name: 'Add a mission', level: 1 })).toBeInTheDocument()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('turns off after an in-page confirm', async () => {
    await openAutomation(enabled)
    await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
    const confirm = within(screen.getByRole('alertdialog', { name: /Turn off automation\?/ }))
    await userEvent.click(confirm.getByRole('button', { name: 'Turn off' }))

    expect(await screen.findByText('Automation is off.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Turn on automation' })).toHaveFocus()
    expect(sent('DELETE')).toHaveLength(1)
    expect(fake.automation).toEqual({ enabled: false })
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('focuses the status after turning off with no providers to turn it back on', async () => {
    fake.providers = []
    fake.automation = enabled
    renderApp(PATH)
    await userEvent.click(await screen.findByRole('button', { name: 'Turn off automation' }))
    await userEvent.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Turn off' }))
    expect(await screen.findByText('Automation is off.')).toHaveFocus()
  })
})
