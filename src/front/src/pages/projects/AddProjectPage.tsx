import { useId, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Button, ButtonMark, Field, TextInput } from '../../components/controls'
import { Icon, type IconName } from '../../components/Icon'
import { PageTitle } from '../../components/PageTitle'
import { InkTab, Tile } from '../../components/Tile'
import { errorCode } from '../../data/errors'
import { useSignedInUser } from '../../data/session'
import { useAddProject } from '../../data/views/addProject'
import { labels } from '../../labels/labels'
import { Notice } from './Notice'

/** Plain words for the API's refusals; anything else shows the API's message. */
const reasons: Record<string, string> = {
  invalid_repo_url: 'Use a repository URL like https://github.com/owner/name.',
  not_found: "There's no repository at that URL, or your GitHub account can't see it.",
  forbidden: 'Adding a repository for the first time needs write access to it on GitHub.',
}

/** What adding a repository does, in three plain lines. */
const steps: { icon: IconName; text: string }[] = [
  { icon: 'github', text: 'Its open issues come in as tasks. GitHub stays the source of truth.' },
  { icon: 'board', text: 'Moving a task between columns writes a board label on its issue.' },
  { icon: 'person', text: 'Tests run on a task’s linked pull request, and a person approves before it merges.' },
]

// Adds a project by its repository URL; one added before opens its board. The
// home page shows it until the user has opened a project.
export function AddProjectPage() {
  const { data: me } = useSignedInUser()
  const navigate = useNavigate()
  const add = useAddProject()
  const [url, setUrl] = useState('')
  const tabId = useId()
  const code = errorCode(add.error)
  const linked = !!me?.github && code !== 'github_not_linked'

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const project = await add.mutateAsync(url.trim()).catch(() => null)
    if (project) navigate(`/projects/${project.id}`)
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-6 px-4 pt-5 pb-10 md:px-7">
      <header className="max-w-[640px]">
        <PageTitle>{labels.addProject}</PageTitle>
        <p className="mt-2.5 mb-0 text-[15px] leading-normal text-ink-2">
          A {labels.project.toLowerCase()} is a GitHub repository. Paste its URL to open its board. If someone added it before, you'll go
          straight there.
        </p>
      </header>

      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,640px)_minmax(260px,360px)]">
        {!linked ? (
          <Notice
            icon="github"
            title="Link GitHub first"
            className="max-w-none"
            actions={
              <Link to="/settings#github" className={buttonClass('primary')}>
                <Icon name="github" />
                {labels.linkGitHub}
              </Link>
            }
          >
            The board reads and writes the repository's issues with your GitHub account, so link it in {labels.settings}.
          </Notice>
        ) : (
          <section aria-labelledby={tabId} className="rounded-dialog border-[1.5px] border-ink bg-paper p-1.5">
            <InkTab id={tabId}>Repository</InkTab>
            <form onSubmit={submit} noValidate className="flex flex-col gap-5 px-4 pt-5 pb-4 sm:px-5">
              <Field id="repo-url" label="Repository URL">
                <TextInput
                  id="repo-url"
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  spellCheck={false}
                  placeholder="https://github.com/owner/name"
                  value={url}
                  onChange={(e) => {
                    setUrl(e.target.value)
                    // Editing clears the refusal: it was about the old URL.
                    if (add.isError) add.reset()
                  }}
                  aria-invalid={!!add.error}
                  aria-describedby={add.error ? 'repo-url-error' : 'repo-url-hint'}
                  className="font-mono"
                />
                {add.error ? (
                  <span id="repo-url-error" role="alert" className="flex items-start gap-1.5 text-[13px] leading-snug font-medium text-signal-ink">
                    <Icon name="alert" size={14} className="mt-px flex-none" />
                    <span className="min-w-0 wrap-anywhere">{(code && reasons[code]) || add.error.message}</span>
                  </span>
                ) : (
                  <span id="repo-url-hint" className="flex items-center gap-1.5 text-[13px] text-ink-2">
                    <Icon name="github" size={14} className="flex-none" />
                    <span>
                      Linked as <span className="font-mono font-semibold text-ink">{me?.github?.login}</span>
                    </span>
                  </span>
                )}
              </Field>
              <div className="flex justify-end border-t border-keyline pt-4">
                <Button type="submit" variant="primary" busy={add.isPending} disabled={!url.trim()} className="min-h-[52px] pl-2 pr-[18px]">
                  <ButtonMark icon="chevronRight" />
                  {add.isPending ? 'Opening…' : `Open ${labels.project.toLowerCase()}`}
                </Button>
              </div>
            </form>
          </section>
        )}

        <Tile step={1} tab={<InkTab>On the board</InkTab>}>
          <ul className="m-0 flex list-none flex-col gap-3.5 p-0 py-1">
            {steps.map((s) => (
              <li key={s.icon} className="flex items-start gap-2.5 text-sm leading-snug">
                <span aria-hidden="true" className="grid size-7 flex-none place-items-center rounded-[2px] bg-paper text-ink">
                  <Icon name={s.icon} size={16} />
                </span>
                <span className="pt-1">{s.text}</span>
              </li>
            ))}
          </ul>
        </Tile>
      </div>
    </div>
  )
}
