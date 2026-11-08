// A small in-memory stand-in for the Go API, served to tests through MSW.
// It follows the status codes and error codes in docs/*.md closely enough for
// the pages to behave as they would against the real thing. Tests read
// `fake` to check what was saved, and `fake.requests` to check what was sent.

import { http, HttpResponse, type JsonBodyType } from 'msw'
import type { AgentSettings, Automation, Column, GitHubLink, PlanningSettings, Project, Provider, TaskDetail } from '../data/api/types'
import { trimSpace } from '../data/views/agentSettings'

interface Account {
  id: number
  email: string
  password: string
}

export interface SentRequest {
  method: string
  path: string
  body: unknown
}

/** An error the next matching request answers with, for testing failure paths. */
export interface ForcedError {
  status: number
  code: string
  message: string
  /** Extra top-level fields, e.g. `task` on a move conflict */
  extra?: Record<string, unknown>
}

function fresh() {
  const project: Project = { id: 1, owner: 'ada', name: 'gantt', permission: 'write' }
  const keys = ['backlog', 'in-progress', 'testing', 'user-check', 'closed'] as const
  const names = ['Backlog', 'In Progress', 'Testing', 'User Check', 'Closed']
  const columns: Column[] = keys.map((key, i) => ({ id: i + 1, key, name: names[i], position: i, closesIssue: key === 'closed' }))
  const task = (id: number, issueNumber: number, columnId: number, position: number, title: string, extra: Partial<TaskDetail> = {}): TaskDetail => ({
    id,
    issueNumber,
    columnId,
    position,
    version: 1,
    title,
    state: columnId === 5 ? 'closed' : 'open',
    url: `https://github.com/ada/gantt/issues/${issueNumber}`,
    runStatus: null,
    needsAttention: null,
    assignees: [],
    branch: null,
    body: '',
    labels: [],
    tests: [],
    testsError: null,
    ...extra,
  })

  return {
    accounts: [{ id: 1, email: 'ada@example.com', password: 'correct horse' }] as Account[],
    /** The account the session cookie belongs to; null when signed out */
    signedInId: null as number | null,
    github: { login: 'ada', kind: 'pat' } as GitHubLink | null,
    /** GET /api/projects/{id}/workflows */
    workflows: { content: '', sha: null as string | null, workflows: [{ name: 'default', description: '' }], error: null as string | null },
    /** GET /api/projects/{id}/assignees */
    assignable: [
      { login: 'ada', avatarUrl: 'https://avatars.example/ada' },
      { login: 'bob', avatarUrl: 'https://avatars.example/bob' },
    ],
    /** GET /api/projects: what the user has opened */
    opened: [] as Project[],
    project,
    columns,
    tasks: [
      task(7, 12, 1, 1, 'Draw the timeline', {
        body: 'One bar per task, by week.\n\nSecond paragraph.',
        labels: [
          { name: 'board:backlog', color: 'ededed' },
          { name: 'enhancement', color: 'a2eeef' },
        ],
      }),
      task(8, 13, 1, 2, 'Zoom the timeline'),
      task(9, 14, 2, 1, 'Set up CI', { runStatus: 'running' }),
      task(10, 15, 3, 1, 'Flaky login test', { runStatus: 'error', needsAttention: 'The runner could not start the container.' }),
      task(11, 16, 4, 1, 'Export to PNG', { runStatus: 'passed' }),
      task(12, 17, 5, 1, 'Pick a name', { runStatus: 'passed' }),
    ],
    providers: [
      { id: 1, name: 'Local', baseUrl: 'http://ollama:11434/v1', keyLast4: 'abcd', models: ['llama3'] },
    ] as Provider[],
    /** What every provider's model listing answers */
    availableModels: ['llama3', 'qwen2', 'mistral'],
    planning: {
      review: true,
      interviewer: { providerId: null, model: null, instructions: '' },
      taskAgent: { providerId: null, model: null, instructions: '' },
    } as PlanningSettings,
    /** The project's automation; null when the API has no runner, so its routes answer 404 */
    automation: { enabled: false } as Automation | null,
    /** The next request to this "METHOD /path" answers with this error instead */
    failNext: {} as Record<string, ForcedError>,
    requests: [] as SentRequest[],
  }
}

export let fake = fresh()

export function resetFake() {
  fake = fresh()
}

/** Make the next request to `route` (e.g. "POST /api/projects/1/tasks/7/move") fail. */
export function failNext(route: string, error: ForcedError) {
  fake.failNext[route] = error
}

const fail = (status: number, code: string, message: string, extra: Record<string, unknown> = {}) =>
  HttpResponse.json({ error: { code, message }, ...extra } as JsonBodyType, { status })

const noContent = () => new HttpResponse(null, { status: 204 })

class Refused extends Error {
  readonly response: Response
  constructor(response: Response) {
    super('refused')
    this.response = response
  }
}

function signedIn(): Account {
  const me = fake.accounts.find((a) => a.id === fake.signedInId)
  if (!me) throw new Refused(fail(401, 'unauthorized', 'Sign in first.'))
  return me
}

function projectAccess(id: string) {
  signedIn()
  if (!fake.github) throw new Refused(fail(403, 'github_not_linked', 'Link your GitHub account first.'))
  if (Number(id) !== fake.project.id) throw new Refused(fail(404, 'not_found', 'Not found.'))
}

function findTask(id: string) {
  const task = fake.tasks.find((t) => t.id === Number(id))
  if (!task) throw new Refused(fail(404, 'not_found', 'Not found.'))
  return task
}

/** The task as most endpoints answer it: without body, labels or tests. */
function brief(t: TaskDetail) {
  const { body: _body, labels: _labels, tests: _tests, testsError: _testsError, ...rest } = t
  return rest
}

type Handler = (args: { params: Record<string, string>; body: Record<string, unknown> }) => Response | Promise<Response>

function route(method: 'get' | 'post' | 'put' | 'patch' | 'delete', path: string, handler: Handler) {
  return http[method](`/api${path}`, async ({ request, params }) => {
    const url = new URL(request.url)
    const text = method === 'get' || method === 'delete' ? '' : await request.text()
    const body = text ? JSON.parse(text) : {}
    fake.requests.push({ method: method.toUpperCase(), path: url.pathname, body: text ? body : undefined })
    const key = `${method.toUpperCase()} ${url.pathname}`
    const forced = fake.failNext[key]
    if (forced) {
      delete fake.failNext[key]
      return fail(forced.status, forced.code, forced.message, forced.extra)
    }
    try {
      return await handler({ params: params as Record<string, string>, body })
    } catch (e) {
      if (e instanceof Refused) return e.response
      throw e
    }
  })
}

const meBody = (a: Account) => ({ id: a.id, email: a.email, github: fake.github })

function checkProvider(body: Record<string, unknown>, creating: boolean) {
  if (creating && !String(body.name ?? '').trim()) throw new Refused(fail(400, 'invalid_name', 'Give the provider a name.'))
  if (body.baseUrl !== undefined && !/^https?:\/\//.test(String(body.baseUrl))) {
    throw new Refused(fail(400, 'invalid_base_url', 'Use an http or https URL.'))
  }
  if (String(body.baseUrl ?? '').includes('unreachable')) {
    throw new Refused(fail(400, 'provider_unreachable', "Couldn't list the provider's models."))
  }
  const models = (body.models as string[] | undefined) ?? []
  const unknown = models.find((m) => !fake.availableModels.includes(m))
  if (unknown) throw new Refused(fail(400, 'unknown_model', `The provider doesn't list ${unknown}.`))
}

/** A model as the API checks it: on one of the caller's providers and its chosen models, or none when not required. */
function checkModel(providerId: number | null, model: string | null, required: boolean) {
  if (providerId === null && model === null && !required) return
  const provider = fake.providers.find((p) => p.id === providerId)
  if (!provider) throw new Refused(fail(400, 'invalid_provider', 'Pick one of your providers.'))
  if (!provider.models.includes(model ?? '')) throw new Refused(fail(400, 'invalid_model', "Pick one of this provider's chosen models."))
}

function checkInstructions(s: unknown) {
  const instructions = trimSpace(String(s ?? ''))
  if ([...instructions].length > 4000 || instructions.includes('\0')) {
    throw new Refused(fail(400, 'invalid_instructions', 'Use at most 4000 characters and no NUL.'))
  }
  return instructions
}

/** A planning agent as the API checks and stores it. */
function checkAgent(a: Partial<AgentSettings>): AgentSettings {
  const providerId = a.providerId ?? null
  const model = a.model ?? null
  checkModel(providerId, model, false)
  return { providerId, model, instructions: checkInstructions(a.instructions) }
}

/** Whether planning or the project's automation uses the provider, as the database's foreign keys see it. */
function providerInUse(id: number) {
  const a = fake.automation
  const { interviewer, taskAgent } = fake.planning
  return interviewer.providerId === id || taskAgent.providerId === id || (!!a?.enabled && (a.decisionProviderId === id || a.chatProviderId === id))
}

function automationAccess(id: string, write: boolean) {
  projectAccess(id)
  if (!fake.automation) throw new Refused(fail(404, 'not_found', 'Not found.'))
  if (write && fake.project.permission === 'read') throw new Refused(fail(403, 'forbidden', 'You need write access.'))
}

export const handlers = [
  // ---- Accounts ----
  route('post', '/auth/signup', ({ body }) => {
    const email = String(body.email ?? '').trim()
    const password = String(body.password ?? '')
    if (!/^[^@\s]+@[^@\s]+$/.test(email)) return fail(400, 'invalid_email', 'Enter a valid email address.')
    if (password.length < 8) return fail(400, 'weak_password', 'Use at least 8 characters.')
    if (fake.accounts.some((a) => a.email === email)) return fail(409, 'email_taken', 'An account with this email already exists.')
    const account = { id: fake.accounts.length + 1, email, password }
    fake.accounts.push(account)
    fake.signedInId = account.id
    fake.github = null
    return HttpResponse.json({ id: account.id, email }, { status: 201 })
  }),
  route('post', '/auth/login', ({ body }) => {
    const account = fake.accounts.find((a) => a.email === body.email && a.password === body.password)
    if (!account) return fail(401, 'invalid_credentials', 'Wrong email or password.')
    fake.signedInId = account.id
    return HttpResponse.json({ id: account.id, email: account.email })
  }),
  route('post', '/auth/logout', () => {
    fake.signedInId = null
    return noContent()
  }),
  route('get', '/me', () => HttpResponse.json(meBody(signedIn()))),
  route('patch', '/me/password', ({ body }) => {
    const me = signedIn()
    if (body.currentPassword !== me.password) return fail(403, 'wrong_password', 'Your current password is wrong.')
    if (String(body.newPassword ?? '').length < 8) return fail(400, 'weak_password', 'Use at least 8 characters.')
    me.password = String(body.newPassword)
    return noContent()
  }),

  // ---- GitHub ----
  route('post', '/github/token', ({ body }) => {
    signedIn()
    if (body.token === 'bad-token') return fail(400, 'invalid_token', 'GitHub did not accept that token.')
    fake.github = { login: 'octo', kind: 'pat' }
    return HttpResponse.json(fake.github)
  }),
  route('delete', '/github', () => {
    signedIn()
    fake.github = null
    return noContent()
  }),

  // ---- Projects and board ----
  route('post', '/projects', ({ body }) => {
    signedIn()
    if (!fake.github) return fail(403, 'github_not_linked', 'Link your GitHub account first.')
    const m = /^https:\/\/github\.com\/([\w-]+)\/([\w.-]+?)(\.git)?\/?$/.exec(String(body.url ?? ''))
    if (!m) return fail(400, 'invalid_repo_url', 'Use a URL like https://github.com/owner/name.')
    if (m[1] !== fake.project.owner || m[2] !== fake.project.name) return fail(404, 'not_found', 'Not found.')
    return HttpResponse.json(fake.project)
  }),
  route('get', '/projects', () => {
    signedIn()
    if (!fake.github) return fail(403, 'github_not_linked', 'Link your GitHub account first.')
    return HttpResponse.json({ projects: fake.opened })
  }),
  route('get', '/me/tasks', () => {
    signedIn()
    if (!fake.github) return fail(403, 'github_not_linked', 'Link your GitHub account first.')
    const login = fake.github.login
    const mine = fake.tasks
      .filter((t) => t.state === 'open' && t.assignees.some((a) => a.login === login))
      .map((t) => {
        const c = fake.columns.find((c) => c.id === t.columnId)!
        return { ...brief(t), column: { key: c.key, name: c.name } }
      })
    const opened = fake.opened.some((p) => p.id === fake.project.id)
    return HttpResponse.json({ projects: opened && mine.length ? [{ project: fake.project, tasks: mine }] : [] })
  }),
  route('get', '/projects/:id/board', ({ params }) => {
    projectAccess(params.id)
    return HttpResponse.json({ project: fake.project, columns: fake.columns, tasks: fake.tasks.map(brief) })
  }),
  route('post', '/projects/:id/tasks', ({ params, body }) => {
    projectAccess(params.id)
    const title = String(body.title ?? '').trim()
    if (!title || title.length > 256) return fail(400, 'invalid_title', 'Give the task a title of 1 to 256 characters.')
    const backlog = fake.tasks.filter((t) => t.columnId === 1)
    const task: TaskDetail = {
      id: Math.max(...fake.tasks.map((t) => t.id)) + 1,
      issueNumber: Math.max(...fake.tasks.map((t) => t.issueNumber)) + 1,
      columnId: 1,
      position: Math.max(0, ...backlog.map((t) => t.position)) + 1,
      version: 1,
      title,
      state: 'open',
      url: '',
      runStatus: null,
      needsAttention: null,
      assignees: [],
      branch: null,
      body: String(body.body ?? ''),
      labels: [{ name: 'board:backlog', color: 'ededed' }],
      tests: [],
      testsError: null,
    }
    task.url = `https://github.com/ada/gantt/issues/${task.issueNumber}`
    fake.tasks.push(task)
    return HttpResponse.json(brief(task), { status: 201 })
  }),
  route('get', '/projects/:id/tasks/:taskId', ({ params }) => {
    projectAccess(params.id)
    return HttpResponse.json(findTask(params.taskId))
  }),
  route('patch', '/projects/:id/tasks/:taskId', ({ params, body }) => {
    projectAccess(params.id)
    const task = findTask(params.taskId)
    if (body.title === undefined && body.body === undefined && body.assignees === undefined) return fail(400, 'bad_request', 'Send a title, a body or assignees.')
    if (body.assignees !== undefined) {
      // Like GitHub, someone who can't be assigned is dropped.
      task.assignees = fake.assignable.filter((a) => (body.assignees as string[]).includes(a.login))
    }
    if (body.title !== undefined) {
      const title = String(body.title).trim()
      if (!title || title.length > 256) return fail(400, 'invalid_title', 'Give the task a title of 1 to 256 characters.')
      task.title = title
    }
    if (body.body !== undefined) task.body = String(body.body)
    return HttpResponse.json(brief(task))
  }),
  route('get', '/projects/:id/tasks/:taskId/runs', ({ params }) => {
    projectAccess(params.id)
    findTask(params.taskId)
    return HttpResponse.json({ runs: [] })
  }),
  route('post', '/projects/:id/tasks/:taskId/retest', ({ params }) => {
    projectAccess(params.id)
    return HttpResponse.json(findTask(params.taskId))
  }),
  route('get', '/projects/:id/assignees', ({ params }) => {
    projectAccess(params.id)
    return HttpResponse.json({ assignees: fake.assignable })
  }),
  route('post', '/projects/:id/tasks/:taskId/move', ({ params, body }) => {
    projectAccess(params.id)
    const task = findTask(params.taskId)
    if (body.version !== task.version) {
      return fail(409, 'conflict', 'Someone else moved this task first.', { task: brief(task) })
    }
    const columnId = Number(body.columnId)
    if (!fake.columns.some((c) => c.id === columnId)) return fail(400, 'invalid_column', 'No such column.')
    const siblings = fake.tasks.filter((t) => t.columnId === columnId && t.id !== task.id).sort((a, b) => a.position - b.position)
    const at = body.beforeTaskId === null ? -1 : siblings.findIndex((t) => t.id === body.beforeTaskId)
    const next = at === -1 ? null : siblings[at].position
    const prev = at === -1 ? (siblings.at(-1)?.position ?? 0) : at > 0 ? siblings[at - 1].position : next! - 2
    task.position = next === null ? prev + 1 : (prev + next) / 2
    task.columnId = columnId
    task.state = columnId === 5 ? 'closed' : 'open'
    task.version++
    return HttpResponse.json(brief(task))
  }),

  // ---- Planning and automation ----
  route('get', '/me/planning', () => {
    signedIn()
    return HttpResponse.json(fake.planning)
  }),
  route('put', '/me/planning', ({ body }) => {
    signedIn()
    if (!body.interviewer || !body.taskAgent) return fail(400, 'bad_request', 'Send both agents.')
    fake.planning = {
      review: body.review !== false,
      interviewer: checkAgent(body.interviewer as AgentSettings),
      taskAgent: checkAgent(body.taskAgent as AgentSettings),
    }
    return HttpResponse.json(fake.planning)
  }),
  route('get', '/projects/:id/workflows', ({ params }) => {
    projectAccess(params.id)
    return HttpResponse.json(fake.workflows)
  }),
  route('put', '/projects/:id/workflows', ({ params, body }) => {
    projectAccess(params.id)
    if (!String(body.content).startsWith('workflows:')) return fail(400, 'invalid_workflows', 'workflows: is required')
    return HttpResponse.json({ branch: 'chore/gantt-workflows-1', pullRequest: { number: 33, url: 'https://github.com/ada/gantt/pull/33' } })
  }),
  route('get', '/projects/:id/automation', ({ params }) => {
    automationAccess(params.id, false)
    return HttpResponse.json(fake.automation)
  }),
  route('put', '/projects/:id/automation', ({ params, body }) => {
    automationAccess(params.id, true)
    // As enable.go: both models, then both instructions.
    const decisionProviderId = body.decisionProviderId as number
    const decisionModel = body.decisionModel as string
    const chatProviderId = body.chatProviderId as number
    const chatModel = body.chatModel as string
    checkModel(decisionProviderId, decisionModel, true)
    checkModel(chatProviderId, chatModel, true)
    const decisionInstructions = checkInstructions(body.decisionInstructions)
    const chatInstructions = checkInstructions(body.chatInstructions)
    fake.automation = {
      enabled: true,
      owner: { id: signedIn().id, login: fake.github?.login ?? null },
      decisionProviderId,
      decisionModel,
      decisionInstructions,
      chatProviderId,
      chatModel,
      chatInstructions,
      pausedReason: null,
      payloadUrl: `https://gantt.example.com/api/github/webhook/${fake.project.id}`,
      webhookSecretSet: fake.automation?.enabled ? fake.automation.webhookSecretSet : false,
    }
    return HttpResponse.json(fake.automation)
  }),
  route('delete', '/projects/:id/automation', ({ params }) => {
    automationAccess(params.id, true)
    fake.automation = { enabled: false }
    return noContent()
  }),

  // ---- Providers ----
  route('get', '/providers', () => {
    signedIn()
    return HttpResponse.json({ providers: [...fake.providers].sort((a, b) => a.name.localeCompare(b.name)) })
  }),
  route('post', '/providers', ({ body }) => {
    signedIn()
    checkProvider(body, true)
    if (!String(body.apiKey ?? '')) return fail(400, 'invalid_api_key', 'Enter the API key.')
    if (fake.providers.some((p) => p.name === body.name)) return fail(409, 'name_taken', 'You already have a provider with this name.')
    const key = String(body.apiKey)
    const provider: Provider = {
      id: Math.max(0, ...fake.providers.map((p) => p.id)) + 1,
      name: String(body.name).trim(),
      baseUrl: String(body.baseUrl),
      keyLast4: key.length > 4 ? key.slice(-4) : '',
      models: (body.models as string[] | undefined) ?? [],
    }
    fake.providers.push(provider)
    return HttpResponse.json({ ...provider, availableModels: fake.availableModels }, { status: 201 })
  }),
  route('patch', '/providers/:providerId', ({ params, body }) => {
    signedIn()
    const provider = fake.providers.find((p) => p.id === Number(params.providerId))
    if (!provider) return fail(404, 'not_found', 'Not found.')
    checkProvider(body, false)
    if (body.baseUrl !== undefined && body.baseUrl !== provider.baseUrl && !body.apiKey) {
      return fail(400, 'invalid_api_key', 'Enter the API key again to change the URL.')
    }
    if (body.baseUrl !== undefined) provider.baseUrl = String(body.baseUrl)
    if (body.apiKey) {
      const key = String(body.apiKey)
      provider.keyLast4 = key.length > 4 ? key.slice(-4) : ''
    }
    if (body.models !== undefined) provider.models = body.models as string[]
    return HttpResponse.json({ ...provider, availableModels: fake.availableModels })
  }),
  route('delete', '/providers/:providerId', ({ params }) => {
    signedIn()
    const id = Number(params.providerId)
    if (!fake.providers.some((p) => p.id === id)) return fail(404, 'not_found', 'Not found.')
    if (providerInUse(id)) return fail(409, 'provider_in_use', 'Automation or planning uses this provider.')
    fake.providers = fake.providers.filter((p) => p.id !== id)
    return noContent()
  }),
]
