import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { db } from '../../../data/mock/store'
import { renderApp } from '../../../test/render'

const BRIEFING = '/projects/orbital-nav/plan/realtime-briefing/briefing'
const REVIEW = '/projects/orbital-nav/plan/realtime-review/doc'

/** The topic row in the draft column, by title. */
const topic = (title: string) => {
  const aside = screen.getByRole('complementary', { name: 'Objective doc draft' })
  return within(aside).getByText(title).closest('li')!
}

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('briefing', () => {
  it('shows question 3 with its suggested answers', async () => {
    renderApp(BRIEFING)
    expect(await screen.findByText('Question 3 of about 8')).toBeInTheDocument()
    const chips = screen.getByRole('group', { name: 'Suggested answers' })
    expect(within(chips).getAllByRole('button').map((b) => b.textContent)).toEqual([
      'Last write wins',
      'Warn and let the mover choose',
      'Lock the card while it is moved',
    ])
    expect(screen.getByText('2 of 7 topics')).toBeInTheDocument()
  })

  it('a picked answer covers the topic and asks the next question', async () => {
    renderApp(BRIEFING)
    await userEvent.click(await screen.findByRole('button', { name: 'Warn and let the mover choose' }))
    expect(screen.getByRole('textbox', { name: 'Your answer' })).toHaveValue('Warn and let the mover choose')
    await userEvent.click(screen.getByRole('button', { name: 'Send answer' }))

    expect(await screen.findByText('Question 4 of about 8')).toBeInTheDocument()
    expect(within(topic('Conflict handling')).getByText('Covered')).toBeInTheDocument()
    expect(within(topic('Scope')).getByText('Asking now')).toBeInTheDocument()
    expect(screen.getByText(/What must the first version include/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Your answer' })).toHaveValue('')
  })

  it('skip leaves the topic empty and moves on', async () => {
    renderApp(BRIEFING)
    await userEvent.click(await screen.findByRole('button', { name: 'Skip question' }))

    expect(await screen.findByText(/What must the first version include/)).toBeInTheDocument()
    expect(within(topic('Conflict handling')).getByText('Not yet')).toBeInTheDocument()
    expect(within(topic('Scope')).getByText('Asking now')).toBeInTheDocument()
  })

  it('drafts the doc with uncovered topics as open questions', async () => {
    const { router } = renderApp(BRIEFING)
    await userEvent.click(await screen.findByRole('button', { name: 'Draft the doc now' }))

    expect(await screen.findByRole('heading', { name: 'Review the objective doc' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/plan/realtime-briefing/doc')
    const open = screen.getByRole('region', { name: 'Open questions' })
    expect(within(open).getByText('Not discussed yet: Conflict handling')).toBeInTheDocument()
    expect(within(open).getByText('Not discussed yet: Risks')).toBeInTheDocument()
    expect(screen.getByText(/Version 1 · drafted by the briefing astronaut from 2 answers/)).toBeInTheDocument()
    expect(db.featureDrafts.find((d) => d.id === 'realtime-briefing')?.status).toBe('needs-review')
  })
})

// Skipped: this screen runs on mock data and isn't routed until its API exists
// (docs/implementation/api-gaps.md). Unskip it when the screen is wired.
describe.skip('doc review', () => {
  it('shows the doc, highlights [TARGET], and links the comment to its section', async () => {
    renderApp(REVIEW)
    const doc = await screen.findByRole('article', { name: 'Objective doc' })
    expect(within(doc).getByRole('heading', { name: 'Realtime sync' })).toBeInTheDocument()
    expect(within(doc).getByText('Needs your review')).toBeInTheDocument()
    expect(doc.querySelector('mark')).toHaveTextContent('[TARGET]')
    expect(screen.getByRole('link', { name: 'Open questions' })).toHaveAttribute('href', '#doc-open-questions')
  })

  it('blocks a denial without text, then sends it back to the briefing', async () => {
    const { router } = renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Deny with feedback' }))
    expect(screen.getByRole('heading', { name: 'Deny the objective doc' })).toBeInTheDocument()

    await userEvent.selectOptions(screen.getByLabelText('Applies to'), 'Success criteria')
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))
    expect(screen.getByText(/Write what should change/)).toBeInTheDocument()
    expect(db.feedback).toHaveLength(0)

    await userEvent.type(screen.getByLabelText(/Feedback/), 'Needs a real latency target.')
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))

    expect(await screen.findByText(/Revising\./)).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/projects/orbital-nav/plan/realtime-review/briefing')
    expect(db.feedback[0]).toMatchObject({ targetType: 'doc', targetId: 'realtime-review', outcome: 'revise', sectionKey: 'success' })
    expect(db.featureDrafts.find((d) => d.id === 'realtime-review')?.status).toBe('denied')
  })

  it('discarding goes back to the board', async () => {
    const { router } = renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Deny with feedback' }))
    await userEvent.click(screen.getByRole('radio', { name: /Discard this objective/ }))
    await userEvent.type(screen.getByLabelText(/Feedback/), 'Not needed this quarter.')
    await userEvent.click(screen.getByRole('button', { name: 'Send feedback' }))

    await screen.findByRole('heading', { name: 'Ship the astronaut approval flow end to end' })
    expect(router.state.location.pathname).toBe('/projects/orbital-nav')
    expect(db.featureDrafts.find((d) => d.id === 'realtime-review')?.status).toBe('discarded')
  })

  it('edits a section in place and saves it', async () => {
    renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Edit the doc myself' }))
    const risks = screen.getByLabelText('Risks')
    await userEvent.clear(risks)
    await userEvent.type(risks, 'Load on review days{enter}Slow conflict prompts{enter}Flaky networks')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    const section = await screen.findByRole('region', { name: 'Risks' })
    expect(within(section).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'Load on review days',
      'Slow conflict prompts',
      'Flaky networks',
    ])
  })

  it('approving creates the objective and opens step 3', async () => {
    const { router } = renderApp(REVIEW)
    await userEvent.click(await screen.findByRole('button', { name: 'Approve and plan payloads' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/projects/orbital-nav/plan/realtime-review/tasks'))
    const draft = db.featureDrafts.find((d) => d.id === 'realtime-review')!
    expect(draft.status).toBe('approved')
    const feature = db.features.find((f) => f.id === draft.featureId)!
    expect(feature).toMatchObject({ key: 'OBJ-9', name: 'Realtime sync', status: 'planned' })
    expect(feature.successCriteria).toHaveLength(3)
  })
})
