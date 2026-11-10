import { useEffect, useId, useState, type FormEvent } from 'react'
import { flushSync } from 'react-dom'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router'
import { Button, Field, TextInput } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { Icon } from '../../components/Icon'
import type { Me } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { useMe } from '../../data/session'
import { GITHUB_OAUTH_URL, useConfirmPassword, useGitHubOAuth, useLinkToken, useUnlinkGitHub } from '../../data/views/settings'
import { labels, settingsLabels } from '../../labels/labels'
import { page } from '../../lib/page'
import { Card, Confirm, Problem, Section } from './Section'
import { sectionIds } from './sections'

const kindLabel = { oauth: 'GitHub sign-in', pat: 'personal access token' } as const

const failedLook = 'bg-signal-wash text-signal-ink'

/** What the API says in `?github=` when it sends the user back here from linking with OAuth (docs/api/github.md). */
const returnNotes = {
  linked: { look: 'border border-line bg-paper text-ink [&>svg]:text-good', icon: 'check', text: 'GitHub is linked.' },
  error: { look: failedLook, icon: 'alert', text: "GitHub didn't link. Nothing was saved; try again." },
  denied: { look: failedLook, icon: 'alert', text: 'You cancelled on GitHub. Nothing was linked.' },
  // Refused, so the link is as it was: another user here signs in with that GitHub account, or this one has no
  // password and signs in only with the GitHub account linked now.
  taken: { look: failedLook, icon: 'alert', text: 'That GitHub account is linked to another account here. Nothing changed.' },
  password_required: { look: failedLook, icon: 'alert', text: 'Set a password before linking another GitHub account, so you can still sign in.' },
  unavailable: { look: failedLook, icon: 'alert', text: "GitHub sign-in isn't set up on this server. Use a personal access token instead." },
} as const

type Outcome = keyof typeof returnNotes

/** The Link with GitHub link, or Link again while linked: only one shows. Not now on the confirm puts focus back on it. */
const oauthLinkId = 'github-oauth-link'

/** Which GitHub account is linked, and how: a change means the link changed. */
const linkKey = (me: Me) => (me.github ? `${me.github.kind}:${me.github.login}` : '')

/**
 * GitHub's new fine-grained token page, filled in with the repository permissions the API uses: it writes branches,
 * file contents (.gantt/workflows.yml), issues and pull requests, and merges, workflow files included. Metadata
 * (read) is added on its own. The page can't pick the repositories, so the person does.
 */
const NEW_TOKEN_URL = `https://github.com/settings/personal-access-tokens/new?${Object.entries({
  name: labels.appName,
  description: `${labels.appName} reads and writes your repositories' issues, branches and pull requests, and merges them.`,
  contents: 'write',
  issues: 'write',
  pull_requests: 'write',
  workflows: 'write',
})
  .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
  .join('&')}`

/** The repository permissions NEW_TOKEN_URL asks for, as GitHub's token page names them */
const tokenPermissions = [
  { name: 'Contents', access: 'Read and write' },
  { name: 'Issues', access: 'Read and write' },
  { name: 'Pull requests', access: 'Read and write' },
  { name: 'Workflows', access: 'Read and write', note: 'to merge changes to workflow files' },
  { name: 'Metadata', access: 'Read-only', note: 'added automatically' },
]

// Link one GitHub account, with OAuth or a personal access token, or unlink
// it (docs/api/github.md). Every project needs it: the board reads and writes the
// repository's issues with this credential.
export function GitHubSection({ me }: { me: Me }) {
  // Linking and unlinking refetch the user; if that fails, what shows here is out of date.
  const refresh = useMe()
  const oauth = useGitHubOAuth()
  const [params] = useSearchParams()
  const returned = params.get('github')
  const outcome = returned && Object.hasOwn(returnNotes, returned) ? (returned as Outcome) : null
  // Only while it still holds: linked while an account is linked; unavailable while none is and the server hasn't
  // since said OAuth is set up; a failed, refused or cancelled link, relinking included, while the link is as it was
  // when the user came back. Linking with a token or unlinking afterwards outdates it.
  const [linkOnReturn] = useState(() => linkKey(me))
  const holds =
    outcome === 'linked' ? Boolean(me.github) : outcome === 'unavailable' ? !me.github && !oauth.data?.enabled : linkKey(me) === linkOnReturn
  const note = outcome && holds ? returnNotes[outcome] : null

  return (
    <Section
      id={sectionIds.github}
      title={settingsLabels.github}
      intro={`Each ${labels.project.toLowerCase()} is a GitHub repository, and its board reads and writes the issues with your GitHub account.`}
      // Nothing opens without it, so the tab lights until it's linked.
      mark={me.github ? undefined : 'signal'}
      markLabel={me.github ? undefined : 'Not linked'}
    >
      {note && (
        <p role="status" className={`m-0 flex items-center gap-2 rounded-[2px] px-3 py-2.5 text-[13px] font-semibold ${note.look}`}>
          <Icon name={note.icon} size={16} className="flex-none" />
          {note.text}
        </p>
      )}
      {returned === 'confirm' && <ConfirmToLink />}
      {refresh.error && (
        <Problem alert action={<Button onClick={() => refresh.refetch()}>Try again</Button>}>
          Couldn't refresh your account: {refresh.error.message}
        </Problem>
      )}
      <Card>
        {me.github ? (
          <Linked login={me.github.login} kind={me.github.kind} hasPassword={me.hasPassword} />
        ) : (
          <NotLinked oauthNoted={note === returnNotes.unavailable} />
        )}
      </Card>
    </Section>
  )
}

/** `hasPassword` false: GitHub is the account's only way in, so it can't be unlinked until the account has a password. */
function Linked({ login, kind, hasPassword }: { login: string; kind: keyof typeof kindLabel; hasPassword: boolean }) {
  const oauth = useGitHubOAuth()
  const unlink = useUnlinkGitHub()
  const refetchMe = useMe().refetch
  const [confirming, setConfirming] = useState(false)
  const id = useId()
  // Focus goes into the confirm when it opens, and back to Unlink when it closes.
  const confirm = (open: boolean) => {
    flushSync(() => setConfirming(open))
    document.getElementById(`${id}-${open ? 'keep' : 'open'}`)?.focus()
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <span aria-hidden="true" className="grid size-10 flex-none place-items-center rounded-[2px] bg-ink text-on-ink">
          <Icon name="github" size={20} />
        </span>
        <p className="m-0 flex min-w-0 flex-1 flex-col">
          <span className="font-semibold wrap-anywhere">
            Linked as <span className="font-bold">{login}</span>
          </span>
          <span className="text-[13px] text-ink-2">with a {kindLabel[kind]}</span>
        </p>
        {!confirming && oauth.data?.enabled && (
          // When GitHub stops taking the link, such as a sign-in idle for six months, linking again replaces it and
          // keeps the projects; unlinking first would drop them.
          <a id={oauthLinkId} href={GITHUB_OAUTH_URL} className={buttonClass('secondary')}>
            <Icon name="github" />
            Link again with GitHub
          </a>
        )}
        {!confirming && hasPassword && (
          <Button id={`${id}-open`} onClick={() => confirm(true)}>
            Unlink
          </Button>
        )}
      </div>
      {!hasPassword && (
        <p className="m-0 text-[13px] leading-normal text-ink-2">
          <Link to={{ hash: sectionIds.account }} className="font-semibold">
            Set a password
          </Link>{' '}
          before unlinking, so you can still sign in.
        </p>
      )}
      {confirming && hasPassword && (
        <Confirm
          id={id}
          question={`Unlinking removes your access to every ${labels.project.toLowerCase()} until you link again.`}
          onCancel={() => confirm(false)}
        >
          <Button id={`${id}-keep`} onClick={() => confirm(false)}>
            Keep it linked
          </Button>
          <Button
            variant="danger"
            onClick={() =>
              unlink.mutate(undefined, {
                // The account has no password after all: the user here is out of date, so it refetches.
                onError: (e) => {
                  if (errorCode(e) !== 'password_required') return
                  confirm(false)
                  void refetchMe()
                },
              })
            }
            busy={unlink.isPending}
          >
            Unlink GitHub
          </Button>
        </Confirm>
      )}
      {unlink.error && (
        <p role="alert" className="m-0 flex items-center gap-1.5 text-[13px] font-semibold text-signal-ink">
          <Icon name="alert" size={16} className="flex-none" />
          <span className="min-w-0 wrap-anywhere">{unlink.error.message}</span>
        </p>
      )}
    </div>
  )
}

/**
 * Linking with OAuth sends a user with a password back with ?github=confirm when their session wasn't made or confirmed
 * in the last 10 minutes: a GitHub account linked with OAuth signs in, so a stolen session cookie mustn't be enough to
 * add one. Confirming the password makes the session recent, then the link starts again.
 */
function ConfirmToLink() {
  const confirm = useConfirmPassword()
  const [password, setPassword] = useState('')
  const [empty, setEmpty] = useState(false)
  // Confirmed: it stays busy until the visit to GitHub replaces the page.
  const [leaving, setLeaving] = useState(false)
  const navigate = useNavigate()
  const { search, hash } = useLocation()
  const id = useId()
  const fieldId = `${id}-password`
  const focusField = () => document.getElementById(fieldId)?.focus()
  const wrong = errorCode(confirm.error) === 'wrong_password'
  const fieldError = empty ? 'Enter your password.' : wrong ? confirm.error?.message : null
  const busy = confirm.isPending || leaving

  // Into the field when the form appears, and back to it, marked, after each wrong password.
  useEffect(() => {
    document.getElementById(fieldId)?.focus()
  }, [fieldId])
  useEffect(() => {
    if (wrong) document.getElementById(fieldId)?.focus()
  }, [wrong, confirm.error, fieldId])
  // Back from GitHub can restore this page as it was left (bfcache), so it stops waiting to leave.
  useEffect(() => {
    const back = (e: PageTransitionEvent) => e.persisted && setLeaving(false)
    window.addEventListener('pageshow', back)
    return () => window.removeEventListener('pageshow', back)
  }, [])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    confirm.reset()
    setEmpty(!password)
    if (!password) {
      focusField()
      return
    }
    confirm.mutate(password, {
      onSuccess: () => {
        setLeaving(true)
        page.visit(GITHUB_OAUTH_URL)
      },
    })
  }

  const notNow = () => {
    const rest = new URLSearchParams(search)
    rest.delete('github')
    document.getElementById(oauthLinkId)?.focus()
    navigate({ search: rest.size ? `?${rest}` : '', hash }, { replace: true })
  }

  return (
    <form
      onSubmit={submit}
      noValidate
      aria-labelledby={`${id}-title`}
      className="flex flex-col gap-3 rounded-[2px] border border-line bg-paper px-3 py-3"
    >
      <p id={`${id}-title`} className="m-0 flex items-center gap-2 text-[13px] font-semibold text-ink">
        <Icon name="lock" size={16} className="flex-none" />
        Confirm your password to link GitHub.
      </p>
      <Field id={fieldId} label="Password">
        <TextInput
          id={fieldId}
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          aria-invalid={!!fieldError}
          aria-describedby={fieldError ? `${fieldId}-error` : undefined}
          className="max-w-[320px]"
        />
        {fieldError && (
          <span id={`${fieldId}-error`} className="text-xs font-semibold text-signal-ink">
            {fieldError}
          </span>
        )}
      </Field>
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex flex-wrap gap-2.5">
          <Button variant="primary" type="submit" busy={busy}>
            {busy ? 'Confirming…' : 'Confirm and continue'}
          </Button>
          <Button onClick={notNow}>Not now</Button>
        </div>
        <div role="status" className="flex min-w-0 items-center text-[13px] font-semibold">
          {confirm.error && !wrong && (
            <span className="inline-flex min-w-0 items-center gap-1.5 text-signal-ink">
              <Icon name="alert" size={16} className="flex-none" />
              <span className="min-w-0 wrap-anywhere">{confirm.error.message}</span>
            </span>
          )}
        </div>
      </div>
    </form>
  )
}

/** `oauthNoted`: the note above already says OAuth isn't set up, so the card needn't say it again. */
function NotLinked({ oauthNoted }: { oauthNoted: boolean }) {
  const oauth = useGitHubOAuth()
  const link = useLinkToken()
  const [token, setToken] = useState('')
  const tokenError = errorCode(link.error) === 'invalid_token'

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (token.trim()) link.mutate(token.trim(), { onSuccess: () => setToken('') })
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <h3 className="m-0 text-base font-bold">Link your GitHub account</h3>
        {oauth.isPending ? (
          // Until the server says, nothing: neither a button that may not work nor a note that may be wrong. The
          // space stays, so the token form doesn't jump when the button comes.
          <div className="min-h-11" />
        ) : oauth.data?.enabled === false ? (
          !oauthNoted && <p className="m-0 text-[13px] leading-normal text-ink-2">GitHub sign-in isn't set up on this server.</p>
        ) : (
          // Also when asking failed: if OAuth really is off, the API sends the user back with ?github=unavailable.
          // A full-page visit: the API redirects to GitHub, and GitHub back to /settings.
          <a id={oauthLinkId} href={GITHUB_OAUTH_URL} className={buttonClass('primary', 'self-start')}>
            <Icon name="github" />
            Link with GitHub
          </a>
        )}
      </div>

      <form onSubmit={submit} noValidate className="flex flex-col gap-3 border-t border-rule pt-5">
        <Field id="github-token" label="Or use a personal access token">
          <TextInput
            id="github-token"
            type="password"
            autoComplete="off"
            spellCheck={false}
            placeholder="github_pat_…"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            aria-invalid={tokenError}
            // Only the opening sentence, which ends where the list starts, right after the field: the whole list would
            // be read as one run-on description every time the field takes focus.
            aria-describedby="github-token-help"
            className="max-w-[440px]"
          />
          <TokenHelp id="github-token-help" />
        </Field>
        {link.error && (
          <p role="alert" className="m-0 flex items-center gap-1.5 text-[13px] font-semibold text-signal-ink">
            <Icon name="alert" size={16} className="flex-none" />
            <span className="min-w-0 wrap-anywhere">{link.error.message}</span>
          </p>
        )}
        <div>
          <Button type="submit" busy={link.isPending} disabled={!token.trim()}>
            {link.isPending ? 'Checking…' : 'Link token'}
          </Button>
        </div>
      </form>
    </div>
  )
}

/** Which token to make and how, with a link to GitHub's token page filled in. `id` names the opening sentence. */
function TokenHelp({ id }: { id: string }) {
  return (
    <div className="flex max-w-[64ch] flex-col gap-2 pt-1 text-[13px] leading-normal text-ink-2">
      <p id={id} className="m-0">
        Use a fine-grained token. Set <span className="font-semibold text-ink">Resource owner</span> to whoever owns your repositories, give it access
        to them, and these repository permissions:
      </p>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {tokenPermissions.map((p) => (
          <li key={p.name} className="flex items-baseline gap-2">
            <span className="w-[6.5rem] flex-none font-semibold text-ink">
              {p.name}
              <span className="sr-only">: </span>
            </span>
            <span className="min-w-0 flex-1">
              <span className="text-ink">{p.access}</span>
              {p.note && ` (${p.note})`}
            </span>
          </li>
        ))}
      </ul>
      <a href={NEW_TOKEN_URL} target="_blank" rel="noreferrer" className={buttonClass('link', 'self-start px-0')}>
        <Icon name="github" size={16} className="flex-none" />
        Create a token with these permissions{' '}
        <span className="sr-only">(opens GitHub in a new tab)</span>
      </a>
      <p className="m-0 text-xs">
        A classic token needs the <span className="font-semibold text-ink">repo</span> and{' '}
        <span className="font-semibold text-ink">workflow</span> scopes.
      </p>
    </div>
  )
}
