// Scripted briefing interview, standing in for a real briefing agent until
// the agent runtime exists. Keyed by doc topic (DocSection.key).

export interface ScriptedQuestion {
  question: string
  /** Suggested answers the operator can pick */
  options?: string[]
}

export const briefingScript: Record<string, ScriptedQuestion> = {
  problem: { question: 'Tell me about the idea in your own words. What is it, and why does the team want it?' },
  users: { question: 'Who is hurt most by the problem today, and how do they work around it?' },
  conflicts: {
    question: 'What should happen when two people move the same card at once? Pick a winner, merge the moves, or warn them?',
    options: ['Last write wins', 'Warn and let the mover choose', 'Lock the card while it is moved'],
  },
  'in-scope': {
    question: 'What must the first version include? Name the pieces you would not ship without.',
    options: ['Live card moves only', 'Card moves and who is viewing', 'Every change on the board'],
  },
  'out-of-scope': {
    question: 'What should the first version leave out on purpose?',
    options: ['Editing while offline', 'Push notifications on mobile', 'Syncing across missions'],
  },
  success: {
    question: 'How will we know it worked? Give me something we can measure.',
    options: ['Moves appear for everyone within a few seconds', 'No conflicting move is ever lost'],
  },
  risks: {
    question: 'What worries you most about building this?',
    options: ['Connection load on busy days', 'Conflict prompts slowing operators down'],
  },
}

export const fallbackQuestion = (topic: string) => `Tell me about ${topic.toLowerCase()} for this objective.`

export const doneMessage = 'Thanks, I have enough to draft the doc. Choose Draft the doc now when you are ready.'

export const notedMessage = 'Noted. I will work that into the next draft.'

export const revisingMessage = (where: string, feedback: string) =>
  `Thanks for the feedback on ${where}: “${feedback}” I will revise the doc and bring it back for review. Add anything else below, then choose Draft the doc now.`
