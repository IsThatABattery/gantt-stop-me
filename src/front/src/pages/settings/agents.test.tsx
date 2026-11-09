import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { PlanningSettings, Provider } from '../../data/api/types'
import { fake, failNext } from '../../test/fakeApi'
import { renderApp } from '../../test/render'

const providers: Provider[] = [
  { id: 1, name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', keyLast4: 'abcd', models: ['gpt-5', 'o3'] },
  { id: 2, name: 'Local', baseUrl: 'http://llm.lan/v1', keyLast4: 'wxyz', models: ['llama'] },
]

const planning: PlanningSettings = {
  review: true,
  interviewer: { providerId: 1, model: 'gpt-5', instructions: '' },
  taskAgent: { providerId: null, model: null, instructions: '' },
}

const puts = () => fake.requests.filter((r) => r.method === 'PUT')

async function openAgents(set: { providers?: Provider[]; planning?: PlanningSettings } = {}) {
  fake.providers = set.providers ?? providers
  fake.planning = set.planning ?? planning
  renderApp('/settings#agents')
  const form = await screen.findByRole('form', { name: 'Agents' })
  const agent = (name: string) => within(within(form).getByRole('group', { name }))
  return { form: within(form), agent }
}

// Opens the model box's list and reads it.
const options = async (input: HTMLElement) => {
  await userEvent.click(input)
  return within(document.getElementById(input.getAttribute('aria-controls')!)!).getAllByRole('option').map((o) => o.textContent)
}

describe('agent settings', () => {
  it('loads the planning settings and saves them as one PUT', async () => {
    const { form, agent } = await openAgents()
    expect(agent('Interviewer').getByLabelText('Provider')).toHaveDisplayValue('OpenAI')
    expect(agent('Interviewer').getByLabelText('Model')).toHaveDisplayValue('gpt-5')
    expect(agent('Task agent').getByLabelText('Provider')).toHaveDisplayValue('None')
    expect(form.getByRole('switch', { name: 'Review plan docs before tasks are made' })).toHaveAttribute('aria-checked', 'true')

    await userEvent.click(form.getByRole('switch', { name: 'Review plan docs before tasks are made' }))
    await userEvent.selectOptions(agent('Task agent').getByLabelText('Provider'), 'Local')
    await userEvent.type(agent('Task agent').getByLabelText(/^Your instructions/), 'Keep tasks small.')
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))

    expect(await form.findByText('Agent settings saved')).toBeInTheDocument()
    expect(puts()).toEqual([
      {
        method: 'PUT',
        path: '/api/me/planning',
        body: {
          review: false,
          interviewer: { providerId: 1, model: 'gpt-5', instructions: '' },
          taskAgent: { providerId: 2, model: 'llama', instructions: 'Keep tasks small.' },
        },
      },
    ])
  })

  it("offers only the chosen provider's models, and resets the model with the provider", async () => {
    const { agent } = await openAgents()
    const model = () => agent('Interviewer').getByLabelText('Model')
    expect(await options(model())).toEqual(['gpt-5', 'o3'])

    await userEvent.selectOptions(agent('Interviewer').getByLabelText('Provider'), 'Local')
    expect(await options(model())).toEqual(['llama'])
    expect(model()).toHaveDisplayValue('llama')

    await userEvent.selectOptions(agent('Interviewer').getByLabelText('Provider'), 'None')
    expect(model()).toBeDisabled()
    expect(model()).toHaveValue('')

    // Planning agents default to a high-reasoning model.
    await userEvent.selectOptions(agent('Interviewer').getByLabelText('Provider'), 'OpenAI')
    expect(model()).toHaveValue('o3')
  })

  it('counts instructions and refuses more than 4000 characters', async () => {
    const { form, agent } = await openAgents()
    const instructions = agent('Interviewer').getByLabelText(/^Your instructions/)
    expect(instructions).toHaveAccessibleDescription(/added to the agent's built-in instructions\. they don't replace them\. 0 \/ 4000/i)

    await userEvent.click(instructions)
    await userEvent.paste('x'.repeat(4001))
    expect(agent('Interviewer').getByText('4001 / 4000')).toBeInTheDocument()
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))

    expect(instructions).toHaveAttribute('aria-invalid', 'true')
    expect(instructions).toHaveAccessibleDescription(/Use 4000 characters or fewer\./)
    expect(instructions).toHaveFocus()
    expect(puts()).toEqual([])
  })

  it('counts characters as the server does, not UTF-16 units', async () => {
    const { form, agent } = await openAgents()
    await userEvent.click(agent('Interviewer').getByLabelText(/^Your instructions/))
    await userEvent.paste(`  ${'🚀'.repeat(3000)}  `)
    expect(agent('Interviewer').getByText('3000 / 4000')).toBeInTheDocument()
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(await form.findByText('Agent settings saved')).toBeInTheDocument()
    expect(puts()).toHaveLength(1)
    expect(fake.planning.interviewer.instructions).toBe('🚀'.repeat(3000))
  })

  it('trims as the server does, keeping U+FEFF and dropping U+0085', async () => {
    const { form, agent } = await openAgents()
    await userEvent.click(agent('Interviewer').getByLabelText(/^Your instructions/))
    await userEvent.paste('\u0085\u3000\uFEFFx\u0085')
    expect(agent('Interviewer').getByText('2 / 4000')).toBeInTheDocument()
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(await form.findByText('Agent settings saved')).toBeInTheDocument()
    expect(fake.planning.interviewer.instructions).toBe('\uFEFFx')
  })

  it('refuses a NUL character', async () => {
    const { form, agent } = await openAgents()
    const instructions = agent('Task agent').getByLabelText(/^Your instructions/)
    await userEvent.click(instructions)
    await userEvent.paste('a\0b')
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(instructions).toHaveAccessibleDescription(/Instructions can't contain a NUL character\./)
    expect(puts()).toEqual([])
  })

  it('asks for a model when the provider has none chosen', async () => {
    const { form, agent } = await openAgents({
      providers: [...providers, { id: 3, name: 'Empty', baseUrl: 'http://x/v1', keyLast4: 'zzzz', models: [] }],
    })
    await userEvent.selectOptions(agent('Task agent').getByLabelText('Provider'), 'Empty')
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(agent('Task agent').getByLabelText('Model')).toHaveAccessibleDescription('Choose a model.')
    expect(puts()).toEqual([])
  })

  it('shows an invalid_instructions it cannot place as the form error', async () => {
    const message = 'Instructions cannot contain a NUL character.'
    const { form } = await openAgents()
    failNext('PUT /api/me/planning', { status: 400, code: 'invalid_instructions', message })
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(await form.findByText(message)).toBeInTheDocument()
  })

  it('warns before leaving with unsaved agent settings', async () => {
    const { form, agent } = await openAgents()
    await userEvent.type(agent('Interviewer').getByLabelText(/^Your instructions/), 'Be brief.')
    expect(form.getByText('Unsaved changes')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: 'Add a project' }))
    expect(await screen.findByRole('alertdialog')).toHaveTextContent('You have unsaved changes')
  })

  it('keeps the typed instructions when the session ends on save', async () => {
    const { form, agent } = await openAgents()
    await userEvent.type(agent('Interviewer').getByLabelText(/^Your instructions/), 'Be brief.')
    fake.signedInId = null
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    const dialog = await screen.findByRole('dialog', { name: 'Session ended' })
    await userEvent.type(within(dialog).getByLabelText('Password'), 'correct horse')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Session ended' })).not.toBeInTheDocument())
    expect(agent('Interviewer').getByLabelText(/^Your instructions/)).toHaveValue('Be brief.')
    expect(form.getByText('Unsaved changes')).toBeInTheDocument()
  })

  it('shows invalid_model on the model field of the agent it is about', async () => {
    // The fake API refuses the retired model as the real one does.
    const message = "Pick one of this provider's chosen models."
    const { form, agent } = await openAgents({ planning: { ...planning, taskAgent: { providerId: 1, model: 'retired', instructions: '' } } })
    expect(agent('Task agent').getByLabelText('Model')).toHaveValue('retired')
    expect(agent('Task agent').getByText('retired is no longer available.')).toBeInTheDocument()
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))

    const model = agent('Task agent').getByLabelText('Model')
    await expect.poll(() => model.getAttribute('aria-invalid')).toBe('true')
    expect(model).toHaveAccessibleDescription(message)
    expect(model).toHaveFocus()
    expect(agent('Interviewer').getByLabelText('Model')).toHaveAttribute('aria-invalid', 'false')
    expect(form.getByText('Fix the marked field and save again.')).toBeInTheDocument()
  })

  it('places a 400 against freshly fetched providers', async () => {
    const { form, agent } = await openAgents()
    // gpt-5 is retired after the page loads, so the fake API refuses it.
    fake.providers = [{ ...providers[0], models: ['o3'] }, providers[1]]
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    const model = agent('Interviewer').getByLabelText('Model')
    await expect.poll(() => model.getAttribute('aria-invalid')).toBe('true')
    expect(model).toHaveFocus()
  })

  it('keeps the form and its edits when refetching providers fails', async () => {
    const { form, agent } = await openAgents()
    await userEvent.type(agent('Interviewer').getByLabelText(/^Your instructions/), 'Be brief.')
    failNext('PUT /api/me/planning', { status: 400, code: 'invalid_model', message: 'Pick a chosen model.' })
    failNext('GET /api/providers', { status: 500, code: 'internal', message: 'Something broke.' })
    await userEvent.click(form.getByRole('button', { name: 'Save agent settings' }))
    expect(await form.findByText('Pick a chosen model.')).toBeInTheDocument()
    expect(agent('Interviewer').getByLabelText(/^Your instructions/)).toHaveValue('Be brief.')
  })

  it('shows the agents with no provider added, and links to the providers, when there are none', async () => {
    const { form, agent } = await openAgents({ providers: [] })
    expect(form.getByRole('link', { name: 'Add a provider' })).toHaveAttribute('href', '/settings#providers')
    expect(agent('Interviewer').getByLabelText('Provider')).toHaveDisplayValue('No provider added')
    expect(agent('Interviewer').getByLabelText('Provider')).toBeDisabled()
    expect(agent('Task agent').getByLabelText('Model')).toBeDisabled()
  })
})
