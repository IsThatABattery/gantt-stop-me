import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Button, Field, TextInput } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { errorCode } from '../../data/errors'
import { useMe } from '../../data/session'
import { useAddProject } from '../../data/views/addProject'
import { labels } from '../../labels/labels'

/** Plain words for the API's refusals; anything else shows the API's message. */
const reasons: Record<string, string> = {
  invalid_repo_url: 'Use a repository URL like https://github.com/owner/name.',
  not_found: "There's no repository at that URL, or your GitHub account can't see it.",
  forbidden: 'Adding a repository for the first time needs write access to it on GitHub.',
}

// Adds a project by its repository URL; one added before opens its board. The
// Flight deck shows it, centered, until the user has opened a project.
export function AddProjectPage() {
  const { data: me } = useMe()
  const navigate = useNavigate()
  const add = useAddProject()
  const [url, setUrl] = useState('')
  const code = errorCode(add.error)
  const linked = !!me?.github && code !== 'github_not_linked'

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const project = await add.mutateAsync(url.trim()).catch(() => null)
    if (project) navigate(`/projects/${project.id}`)
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 px-8 py-10">
      <header className="w-full max-w-[640px]">
        <Eyebrow>{labels.flightDeck}</Eyebrow>
        <h1 className="mt-1.5 mb-0 font-heading text-[26px] font-semibold">{labels.addProject}</h1>
        <p className="mt-2 mb-0 leading-normal text-text-muted-strong">
          A {labels.project.toLowerCase()} is a GitHub repository, and its open issues become {labels.tasks.toLowerCase()}. Paste the
          repository's URL to open its board. If it was added before, you'll go straight to it.
        </p>
      </header>

      {!linked ? (
        <div className="w-full max-w-[640px] rounded-[14px] border border-line bg-surface">
          <StatusMessage
            title="Link GitHub first"
            body={`The board reads and writes the repository's issues with your GitHub account, so link it in ${labels.settings}.`}
          >
            <Link to="/settings#github" className={buttonClass('primary')}>
              <Icon name="github" />
              {labels.linkGitHub}
            </Link>
          </StatusMessage>
        </div>
      ) : (
        <form onSubmit={submit} noValidate className="flex w-full max-w-[640px] flex-col gap-4 rounded-[14px] border border-line bg-surface p-6">
          <Field id="repo-url" label="Repository URL">
            <TextInput
              id="repo-url"
              type="url"
              inputMode="url"
              autoComplete="off"
              spellCheck={false}
              placeholder="https://github.com/owner/name"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              aria-invalid={!!add.error}
              aria-describedby={add.error ? 'repo-url-error' : 'repo-url-hint'}
            />
            {add.error ? (
              <span id="repo-url-error" role="alert" className="text-[13px] text-critical">
                {(code && reasons[code]) || add.error.message}
              </span>
            ) : (
              <span id="repo-url-hint" className="text-xs text-text-muted">
                Linked as <span className="font-mono">{me?.github?.login}</span>.
              </span>
            )}
          </Field>
          <div className="flex justify-end">
            <Button type="submit" variant="primary" disabled={add.isPending || !url.trim()} className="px-[22px]">
              {add.isPending ? 'Opening…' : `Open ${labels.project.toLowerCase()}`}
            </Button>
          </div>
        </form>
      )}
    </div>
  )
}
