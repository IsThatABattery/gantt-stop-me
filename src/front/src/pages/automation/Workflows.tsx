import { useState, type ReactNode } from 'react'
import { Button, Textarea } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { InkTab, Tile } from '../../components/Tile'
import { useSaveWorkflows, useWorkflows } from '../../data/views/workflows'
import { Card } from '../settings/Section'

// One workflow's fields, as the parser documents them; context and dockerfile default to . and Dockerfile.
const workflowBlock = (name: string) => `  ${name}:
    description: What this workflow can test; the decision agent reads it to pick.
    context: .
    dockerfile: Dockerfile
`

/** Where the file lives in the repository */
const FILE = '.gantt/workflows.yml'

/** The repository's workflows: where tests run. Saving opens a pull request that changes .gantt/workflows.yml. */
export function Workflows({ projectId, readOnly }: { projectId: string; readOnly: boolean }) {
  const file = useWorkflows(projectId)
  const save = useSaveWorkflows(projectId)
  // null until edited, so a reload shows GitHub's file.
  const [draft, setDraft] = useState<string | null>(null)
  if (file.isPending) return null

  const tab = (count?: number) => (
    <InkTab id="workflows-title" count={count} countLabel={count === 1 ? 'workflow' : 'workflows'}>
      Workflows
    </InkTab>
  )
  if (file.error)
    return (
      <Tile as="section" aria-labelledby="workflows-title" tab={tab()}>
        <Problem>Couldn't load the workflows. {file.error.message}</Problem>
      </Tile>
    )

  const { content, sha, workflows, error } = file.data
  const text = draft ?? content
  const edited = draft !== null && draft !== content
  const add = () => {
    const base = text.trim() ? text.replace(/\n*$/, '\n') : 'workflows:\n'
    setDraft(base + workflowBlock(`workflow-${workflows.length + 1}`))
  }

  return (
    <Tile as="section" aria-labelledby="workflows-title" frameClassName="gap-3" tab={tab(workflows.length)}>
      <p className="m-0 max-w-[80ch] px-0.5 text-[13px] leading-[1.45] text-ink-2">
        Each workflow builds an image the tests run in. A test pinned with <code className="text-ink">[name]</code> runs in that workflow;
        otherwise the decision agent picks one. Runs read the file at the commit they test.
      </p>

      <ul className="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(min(100%,240px),1fr))] gap-2 p-0">
        {workflows.map((w) => (
          <Card key={w.name} as="li" compact gap="gap-1">
            <span className="font-mono text-[13px] font-semibold break-all">{w.name}</span>
            <span className="text-[13px] leading-[1.4] text-ink-2">
              {w.description || (sha ? 'No description.' : "No file yet: runs build the Dockerfile at the repository's root.")}
            </span>
          </Card>
        ))}
      </ul>

      {error && <Problem>The file on GitHub doesn't parse: {error}</Problem>}

      <Card gap="gap-2.5">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <span className="font-mono text-[13px] font-semibold">{FILE}</span>
          <span className="text-xs text-ink-2">
            {readOnly ? 'Read only: you need write access to change it.' : edited ? 'Edited here. Open a pull request to propose it.' : 'Edit it here; saving opens a pull request.'}
          </span>
        </div>
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
      </Card>

      {save.error && <Problem alert>{save.error.message}</Problem>}
      {!readOnly && (
        <div className="flex flex-wrap items-center justify-end gap-2.5 border-t border-tile-keyline pt-3">
          {save.data && draft === null && (
            <p role="status" className="m-0 mr-auto inline-flex flex-wrap items-center gap-x-1.5 gap-y-1 px-0.5 text-[13px] font-semibold">
              <Icon name="pr" size={16} className="flex-none text-good" />
              <span>
                Opened{' '}
                <a href={save.data.pullRequest.url} target="_blank" rel="noreferrer">
                  pull request #{save.data.pullRequest.number}
                </a>{' '}
                from <code className="font-mono">{save.data.branch}</code>. Runs use it once it merges.
              </span>
            </p>
          )}
          <Button onClick={add}>
            <Icon name="plus" size={16} />
            Add a workflow
          </Button>
          <Button variant="primary" busy={save.isPending} disabled={!edited} onClick={() => save.mutate({ content: text, sha }, { onSuccess: () => setDraft(null) })}>
            <Icon name="pr" size={16} />
            Open a pull request
          </Button>
        </div>
      )}
    </Tile>
  )
}

/** An error, on the signal wash with the alert mark */
function Problem({ children, alert }: { children: ReactNode; alert?: boolean }) {
  return (
    <p role={alert ? 'alert' : undefined} className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] font-semibold whitespace-pre-wrap text-signal-ink">
      <Icon name="alert" size={16} className="mt-px flex-none" />
      <span>{children}</span>
    </p>
  )
}
