import { useState } from 'react'
import { Button, Textarea } from '../../components/controls'
import { errorCode } from '../../data/errors'
import { useSaveWorkflows, useWorkflows } from '../../data/views/workflows'

// One workflow's fields, as the parser documents them; context and dockerfile default to . and Dockerfile.
const workflowBlock = (name: string) => `  ${name}:
    description: What this workflow can test; the decision agent reads it to pick.
    context: .
    dockerfile: Dockerfile
`

/** The repository's workflows: where tests run. Saving opens a pull request that changes .gantt/workflows.yml. */
export function Workflows({ projectId, readOnly }: { projectId: string; readOnly: boolean }) {
  const file = useWorkflows(projectId)
  const save = useSaveWorkflows(projectId)
  // null until edited, so a reload shows GitHub's file.
  const [draft, setDraft] = useState<string | null>(null)
  if (file.isPending) return null
  if (file.error && !file.data)
    return (
      <p role="alert" className="m-0 flex flex-wrap items-center gap-3 text-sm text-critical">
        <span className="min-w-0 wrap-anywhere">Couldn't load the workflows: {file.error.message}</span>
        <Button onClick={() => file.refetch()}>Try again</Button>
      </p>
    )

  const { content, sha, workflows, error } = file.data
  const text = draft ?? content
  // The draft stays after saving, so the file just sent stays in view.
  const opened = save.data && text === save.variables?.content
  const add = () => {
    const base = text.trim() ? text.replace(/\n*$/, '\n') : 'workflows:\n'
    setDraft(base + workflowBlock(`workflow-${workflows.length + 1}`))
  }

  return (
    <section className="flex flex-col gap-3 border-t border-line-soft pt-6" aria-labelledby="workflows-title">
      <div>
        <h2 id="workflows-title" className="m-0 font-heading text-lg font-semibold">
          Workflows
        </h2>
        <p className="m-0 mt-1 text-[13px] text-text-muted-strong">
          Each workflow builds an image the tests run in. A test pinned with <code>[name]</code> runs in that workflow; otherwise the
          decision agent picks one. Runs read the file at the commit they test.
        </p>
      </div>
      <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
        {workflows.map((w) => (
          <li key={w.name} title={w.description} className="rounded-lg border border-line-strong bg-surface-sunken px-2.5 py-1 font-mono text-xs">
            {w.name}
          </li>
        ))}
        {!sha && <li className="text-xs text-text-muted">No file yet: runs build the Dockerfile at the repository's root.</li>}
      </ul>
      {error && <p className="m-0 text-[13px] wrap-anywhere text-critical">The file on GitHub doesn't parse: {error}</p>}
      <Textarea
        aria-label="Workflows file"
        value={text}
        readOnly={readOnly}
        placeholder={'workflows:\n' + workflowBlock('app')}
        rows={Math.min(24, Math.max(8, text.split('\n').length + 1))}
        onChange={(e) => setDraft(e.target.value)}
        className="font-mono text-[13px]"
        spellCheck={false}
      />
      {!readOnly && (
        <div className="flex flex-wrap items-center justify-end gap-2.5">
          {opened && (
            <span className="mr-auto text-[13px] text-good">
              Opened{' '}
              <a href={save.data.pullRequest.url} target="_blank" rel="noreferrer">
                pull request #{save.data.pullRequest.number}
              </a>{' '}
              from <code>{save.data.branch}</code>. Runs use it once it merges.
            </span>
          )}
          <Button onClick={add}>Add a workflow</Button>
          <Button
            variant="primary"
            disabled={save.isPending || draft === null || draft === content || opened}
            // After workflows_changed, refetch the sha so saving again goes through.
            onClick={() => save.mutate({ content: text, sha }, { onError: (e) => errorCode(e) === 'workflows_changed' && file.refetch() })}
          >
            Open a pull request
          </Button>
        </div>
      )}
      {save.error && (
        <p role="alert" className="m-0 text-[13px] whitespace-pre-wrap wrap-anywhere text-critical">
          {save.error.message}
          {errorCode(save.error) === 'workflows_changed' && ' Opening a pull request again replaces the newer file with yours.'}
        </p>
      )}
    </section>
  )
}
