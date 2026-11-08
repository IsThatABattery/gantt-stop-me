import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../data/mock/store'
import { renderApp } from '../../test/render'

async function openSettings() {
  const utils = renderApp('/settings')
  await screen.findByRole('button', { name: 'Save changes' })
  return utils
}

const save = () => userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
const password = () => within(screen.getByRole('form', { name: 'Change password' }))

describe('settings', () => {
  it('shows the profile of the signed-in operator', async () => {
    await openSettings()
    expect(screen.getByLabelText('Name')).toHaveValue('TV')
    expect(screen.getByLabelText('Email')).toHaveValue('tv@team.dev')
    expect(screen.getByRole('navigation', { name: 'Settings sections' })).toHaveTextContent(
      'ProfileAstronauts and autonomyOperators and rolesNotificationsIntegrations',
    )
  })

  it('saves name and email to the user', async () => {
    await openSettings()
    await userEvent.clear(screen.getByLabelText('Name'))
    await userEvent.type(screen.getByLabelText('Name'), 'Tess V')
    await save()
    expect(await screen.findByText('Settings saved')).toBeInTheDocument()
    expect(db.users.find((u) => u.id === 'u-tv')!.name).toBe('Tess V')
  })

  it('checks the password change and confirms it', async () => {
    await openSettings()
    const f = password()
    await userEvent.type(f.getByLabelText('Current password'), 'old-pass')
    await userEvent.type(f.getByLabelText('New password'), 'short')
    await userEvent.type(f.getByLabelText('Confirm new password'), 'shorter')
    await userEvent.click(f.getByRole('button', { name: 'Change password' }))
    expect(f.getByText('Use 8 or more characters.')).toBeInTheDocument()
    expect(f.getByText("The new passwords don't match.")).toBeInTheDocument()
    expect(f.getByLabelText('New password')).toHaveFocus()

    await userEvent.clear(f.getByLabelText('New password'))
    await userEvent.type(f.getByLabelText('New password'), 'long-enough')
    await userEvent.clear(f.getByLabelText('Confirm new password'))
    await userEvent.type(f.getByLabelText('Confirm new password'), 'long-enough')
    await userEvent.click(f.getByRole('button', { name: 'Change password' }))
    expect(await f.findByText('Password changed')).toBeInTheDocument()
    expect(f.getByLabelText('New password')).toHaveValue('')
  })

  it('saves autonomy and approval rules, and they stick after navigating away', async () => {
    const { router } = await openSettings()
    await userEvent.click(screen.getByRole('radio', { name: /Free flight/ }))
    await userEvent.click(screen.getByRole('switch', { name: 'Require two approvals on Core phase' }))
    await save()
    expect(await screen.findByText('Settings saved')).toBeInTheDocument()
    expect(db.settings.defaultAutonomy).toBe(2)
    expect(db.settings.approvalRules.find((r) => r.id === 'rule-two')!.enabled).toBe(true)

    await router.navigate('/')
    await router.navigate('/settings')
    await screen.findByRole('button', { name: 'Save changes' })
    expect(screen.getByRole('radio', { name: /Free flight/ })).toBeChecked()
    expect(screen.getByRole('switch', { name: 'Require two approvals on Core phase' })).toHaveAttribute('aria-checked', 'true')
  })

  it('keeps only the last four characters of the API key', async () => {
    await openSettings()
    await userEvent.type(screen.getByLabelText('API key'), 'sk-test-abcd1234')
    await save()
    expect(await screen.findByText('•••• 1234')).toBeInTheDocument()
    expect(screen.getByLabelText('API key')).toHaveValue('')
    expect(db.settings.provider).toEqual({ name: 'OpenAI', keyLast4: '1234' })
    expect(JSON.stringify(db.settings)).not.toContain('sk-test')
  })

  it('rejects a payload cap below 1', async () => {
    await openSettings()
    const cap = screen.getByLabelText('Concurrent astronaut payloads')
    await userEvent.clear(cap)
    await userEvent.type(cap, '0')
    await save()
    expect(screen.getByText('Use a whole number, 1 or more.')).toBeInTheDocument()
    expect(cap).toHaveFocus()
    expect(db.settings.agentTaskCap).toBe(4)
  })

  it('Cancel puts the form back', async () => {
    await openSettings()
    await userEvent.click(screen.getByRole('radio', { name: /Tethered/ }))
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('radio', { name: /Guided/ })).toBeChecked()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  })

  it('warns before leaving with unsaved changes', async () => {
    const { router } = await openSettings()
    await userEvent.click(screen.getByRole('radio', { name: /Tethered/ }))
    await userEvent.click(screen.getAllByRole('link', { name: /Mission control|All missions/ })[0])

    expect(await screen.findByRole('alertdialog')).toHaveTextContent('You have unsaved changes')
    expect(router.state.location.pathname).toBe('/settings')
    await userEvent.click(screen.getByRole('button', { name: 'Leave without saving' }))
    expect(router.state.location.pathname).not.toBe('/settings')
  })

  it('moves between sections without a warning', async () => {
    const { router } = await openSettings()
    await userEvent.click(screen.getByRole('radio', { name: /Tethered/ }))
    await userEvent.click(screen.getByRole('link', { name: 'Integrations' }))
    expect(router.state.location.hash).toBe('#integrations')
    expect(screen.getByRole('link', { name: 'Integrations' })).toHaveAttribute('aria-current', 'true')
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows the undesigned sections as not built', async () => {
    await openSettings()
    for (const name of ['Operators and roles', 'Notifications']) {
      expect(within(screen.getByRole('region', { name })).getByText(/Not built yet/)).toBeInTheDocument()
    }
  })
})
