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
import { Card, Confirm, Problem, Section } from './Section'
import { sectionIds } from './sections'

const kindLabel = { oauth: 'GitHub sign-in', pat: 'personal access token' } as const

/** What the OAuth callback says in `?github=` when it sends the user back here. */
const returnNotes = {
  linked: { look: 'border border-line bg-paper text-ink [&>svg]:text-good', icon: 'check', text: 'GitHub is linked.' },
  error: { look: 'bg-signal-wash text-signal-ink', icon: 'alert', text: "GitHub didn't link. Nothing was saved; try again." },
} as const

// Link one GitHub account, with OAuth or a personal access token, or unlink
// it (docs/api/github.md). Every project needs it: the board reads and writes the
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
      {refresh.error && (
        <Problem alert action={<Button onClick={() => refresh.refetch()}>Try again</Button>}>
          Couldn't refresh your account: {refresh.error.message}
        </Problem>
      )}
      <Card>{me.github ? <Linked login={me.github.login} kind={me.github.kind} /> : <NotLinked />}</Card>
    </Section>
  )
}

function Linked({ login, kind }: { login: string; kind: keyof typeof kindLabel }) {
  const unlink = useUnlinkGitHub()
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
        {!confirming && (
          <Button id={`${id}-open`} onClick={() => confirm(true)}>
            Unlink
          </Button>
        )}
      </div>
      {confirming && (
        <Confirm
          id={id}
          question={`Unlinking removes your access to every ${labels.project.toLowerCase()} until you link again.`}
          onCancel={() => confirm(false)}
        >
          <Button id={`${id}-keep`} onClick={() => confirm(false)}>
            Keep it linked
          </Button>
          <Button variant="danger" onClick={() => unlink.mutate()} busy={unlink.isPending}>
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
      <div className="flex flex-col gap-3">
        <h3 className="m-0 text-base font-bold">Link your GitHub account</h3>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {/* A full-page visit: the API redirects to GitHub, and GitHub back to /settings. */}
          <a href={GITHUB_OAUTH_URL} className={buttonClass('primary')}>
            <Icon name="github" />
            Link with GitHub
          </a>
          <p className="m-0 max-w-[46ch] text-xs leading-normal text-ink-2">Needs the server's GitHub OAuth app. If it isn't set up, use a token instead.</p>
        </div>
      </div>

      <form onSubmit={submit} noValidate className="flex flex-col gap-3 border-t border-rule pt-5">
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
            className="max-w-[440px]"
          />
          <span id="github-token-help" className="text-xs text-ink-2">
            It needs read and write access to issues and repository metadata.
          </span>
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
