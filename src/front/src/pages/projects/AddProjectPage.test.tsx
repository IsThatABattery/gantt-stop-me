import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { fake } from '../../test/fakeApi'
import { renderApp } from '../../test/render'

async function submit(url: string) {
  await userEvent.type(await screen.findByLabelText('Repository URL'), url)
  await userEvent.click(screen.getByRole('button', { name: 'Open mission' }))
}

describe('flight deck', () => {
  it('lists the projects opened before, and opens one', async () => {
    fake.opened = [fake.project]
    const { router } = renderApp('/')
    await userEvent.click(await screen.findByRole('link', { name: /ada\/gantt/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1'))
  })

  it('shows the URL box when none were opened', async () => {
    renderApp('/')
    expect(await screen.findByLabelText('Repository URL')).toBeInTheDocument()
  })
})

describe('add a project', () => {
  it('opens the board of a repository by its URL', async () => {
    const { router } = renderApp('/')
    expect(await screen.findByText('ada')).toBeInTheDocument()
    await submit('https://github.com/ada/gantt.git')

    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/1'))
    expect(fake.requests.find((r) => r.method === 'POST' && r.path === '/api/projects')?.body).toEqual({ url: 'https://github.com/ada/gantt.git' })
  })

  it('explains a URL that is not a repository', async () => {
    renderApp('/')
    await submit('github.com/ada')
    expect(await screen.findByRole('alert')).toHaveTextContent('Use a repository URL like https://github.com/owner/name.')
  })

  it('explains a repository it cannot find', async () => {
    renderApp('/')
    await submit('https://github.com/ada/secret')
    expect(await screen.findByRole('alert')).toHaveTextContent("There's no repository at that URL, or your GitHub account can't see it.")
  })

  it('asks to link GitHub first', async () => {
    fake.github = null
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Link GitHub first' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Repository URL')).not.toBeInTheDocument()
  })
})
