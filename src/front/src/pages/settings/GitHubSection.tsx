import { useId, useState, type FormEvent } from 'react'
import { flushSync } from 'react-dom'
import { useSearchParams } from 'react-router'
import { Button, Field, TextInput } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { Icon } from '../../components/Icon'
import type { Me } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { useMe } from '../../data/session'
import { GITHUB_OAUTH_URL, useLinkToken, useUnlinkGitHub } from '../../data/views/settings'
import { labels, settingsLabels } from '../../labels/labels'
import { Section } from './Section'
import { sectionIds } from './sections'

const kindLabel = { oauth: 'GitHub sign-in', pat: 'personal access token' } as const

/** What the OAuth callback says in `?github=` when it sends the user back here. */
const returnNotes = {
  linked: { tone: 'text-good', text: 'GitHub is linked.' },
  error: { tone: 'text-critical', text: "GitHub didn't link. Nothing was saved; try again." },
} as const

// Link one GitHub account, with OAuth or a personal access token, or unlink
// it (docs/github.md). Every project needs it: the board reads and writes the
// repository's issues with this credential.
export function GitHubSection({ me }: { me: Me }) {
  // Linking and unlinking refetch the user; if that fails, what shows here is out of date.
  const refresh = useMe()
  const [params] = useSearchParams()
  const returned = params.get('github')
  // Only while it still holds: linking with a token or unlinking afterwards outdates it.
  const note = returned === 'linked' && me.github ? returnNotes.linked : returned === 'error' && !me.github ? returnNotes.error : null

  return (
    <Section
      id={sectionIds.github}
      title={settingsLabels.github}
      intro={`Each ${labels.project.toLowerCase()} is a GitHub repository, and its board reads and writes the issues with your GitHub account.`}
    >
      {note && (
        <p role="status" className={`m-0 text-[13px] ${note.tone}`}>
          {note.text}
        </p>
      )}
      {refresh.error && (
        <div role="alert" className="flex flex-wrap items-center gap-3 text-[13px] text-critical">
          <span className="min-w-0 wrap-anywhere">Couldn't refresh your account: {refresh.error.message}</span>
          <Button onClick={() => refresh.refetch()}>Try again</Button>
        </div>
      )}
      {me.github ? <Linked login={me.github.login} kind={me.github.kind} /> : <NotLinked />}
    </Section>
  )
}

function Linked({ login, kind }: { login: string; kind: keyof typeof kindLabel }) {
  const unlink = useUnlinkGitHub()
  const id = useId()
  const [confirming, setConfirming] = useState(false)
  const confirm = (open: boolean) => {
    flushSync(() => setConfirming(open))
    document.getElementById(`${id}-${open ? 'keep' : 'unlink'}`)?.focus()
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="m-0 flex flex-wrap items-center gap-2">
        <Icon name="github" />
        Linked as <span className="font-mono">{login}</span>
        <span className="text-[13px] text-text-muted">with a {kindLabel[kind]}</span>
      </p>
      {confirming ? (
        <div
          role="alertdialog"
          aria-label="Unlink GitHub"
          onKeyDown={(e) => e.key === 'Escape' && confirm(false)}
          className="flex flex-wrap items-center gap-3 rounded-card border border-warning bg-warning/6 p-4"
        >
          <p className="m-0 min-w-48 flex-1 text-[13px]">
            Unlinking removes your access to every {labels.project.toLowerCase()} until you link again.
          </p>
          {/* One group for the buttons: where they don't fit beside the text, they go under it together. */}
          <div className="flex flex-wrap gap-3">
            <Button id={`${id}-keep`} onClick={() => confirm(false)}>
              Keep it linked
            </Button>
            <Button
              onClick={() => unlink.mutate()}
              disabled={unlink.isPending}
              className="border-warning text-warning hover:border-warning hover:text-warning"
            >
              Unlink GitHub
            </Button>
          </div>
        </div>
      ) : (
        <div>
          <Button id={`${id}-unlink`} onClick={() => confirm(true)}>
            Unlink
          </Button>
        </div>
      )}
      {unlink.error && (
        <p role="alert" className="m-0 text-[13px] wrap-anywhere text-critical">
          {unlink.error.message}
        </p>
      )}
    </div>
  )
}

function NotLinked() {
  const link = useLinkToken()
  const [token, setToken] = useState('')
  const tokenError = errorCode(link.error) === 'invalid_token'

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (token.trim()) link.mutate(token.trim(), { onSuccess: () => setToken('') })
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <div>
          {/* A full-page visit: the API redirects to GitHub, and GitHub back to /settings. */}
          <a href={GITHUB_OAUTH_URL} className={buttonClass('primary')}>
            <Icon name="github" />
            Link with GitHub
          </a>
        </div>
        <p className="m-0 text-xs text-text-muted">Needs the server's GitHub OAuth app. If it isn't set up, use a token instead.</p>
      </div>

      <form onSubmit={submit} noValidate className="flex flex-col gap-3 border-t border-line-soft pt-5">
        <Field id="github-token" label="Or use a personal access token">
          <TextInput
            id="github-token"
            type="password"
            autoComplete="off"
            spellCheck={false}
            placeholder="ghp_…"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            aria-invalid={tokenError}
            aria-describedby="github-token-help"
          />
          <span id="github-token-help" className="text-xs text-text-muted">
            It needs read and write access to issues and repository metadata.
          </span>
        </Field>
        {link.error && (
          <p role="alert" className="m-0 text-[13px] wrap-anywhere text-critical">
            {link.error.message}
          </p>
        )}
        <div>
          <Button type="submit" disabled={link.isPending || !token.trim()}>
            {link.isPending ? 'Checking…' : 'Link token'}
          </Button>
        </div>
      </form>
    </div>
  )
}
