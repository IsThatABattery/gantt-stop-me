import { useState } from 'react'
import { Button, Textarea } from '../../components/controls'
import { Eyebrow } from '../../components/Eyebrow'
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
  if (file.error) return <p className="m-0 text-sm text-critical">Couldn't load the workflows: {file.error.message}</p>

  const { content, sha, workflows, error } = file.data
  const text = draft ?? content
  const add = () => {
    const base = text.trim() ? text.replace(/\n*$/, '\n') : 'workflows:\n'
    setDraft(base + workflowBlock(`workflow-${workflows.length + 1}`))
  }

  return (
    <section className="flex flex-col gap-3 border-t border-line-soft pt-6" aria-labelledby="workflows-title">
      <div>
        <Eyebrow>.gantt/workflows.yml</Eyebrow>
        <h2 id="workflows-title" className="mt-1 mb-0 font-heading text-lg font-semibold">
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
      {error && <p className="m-0 text-[13px] text-critical">The file on GitHub doesn't parse: {error}</p>}
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
      {save.error && (
        <p role="alert" className="m-0 text-[13px] whitespace-pre-wrap text-critical">
          {save.error.message}
        </p>
      )}
      {!readOnly && (
        <div className="flex flex-wrap items-center justify-end gap-2.5">
          {save.data && draft === null && (
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
            disabled={save.isPending || draft === null || draft === content}
            onClick={() => save.mutate({ content: text, sha }, { onSuccess: () => setDraft(null) })}
          >
            Open a pull request
          </Button>
        </div>
      )}
    </section>
  )
}
