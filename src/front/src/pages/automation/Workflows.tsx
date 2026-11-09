import { useState } from 'react'
import { Button, Textarea } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { InkTab, Tile } from '../../components/Tile'
import { errorCode } from '../../data/errors'
import { useSaveWorkflows, useWorkflows } from '../../data/views/workflows'
import { Card, Problem } from '../settings/Section'

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
  // A failed refetch keeps the file and the draft.
  if (file.error && !file.data)
    return (
      <Tile as="section" aria-labelledby="workflows-title" tab={tab()}>
        <Problem alert action={<Button onClick={() => file.refetch()}>Try again</Button>}>
          Couldn't load the workflows. {file.error.message}
        </Problem>
      </Tile>
    )

  const { content, sha, workflows, error } = file.data
  const text = draft ?? content
  const edited = draft !== null && draft !== content
  // The draft stays after saving, so the file just sent stays in view.
  const opened = save.data && text === save.variables?.content
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
            {readOnly
              ? 'Read only: you need write access to change it.'
              : opened
                ? 'Sent in the pull request below. Edit it again to propose more.'
                : edited
                  ? 'Edited here. Open a pull request to propose it.'
                  : 'Edit it here; saving opens a pull request.'}
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

      {!readOnly && (
        <div className="flex flex-wrap items-center justify-end gap-2.5 border-t border-tile-keyline pt-3">
          {opened && (
            <p role="status" className="m-0 mr-auto inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 px-0.5 text-[13px] font-semibold">
              <Icon name="pr" size={16} className="flex-none text-good" />
              <span className="min-w-0 wrap-anywhere">
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
          <Button
            variant="primary"
            busy={save.isPending}
            disabled={!edited || opened}
            // After workflows_changed, refetch the sha so saving again goes through.
            onClick={() => save.mutate({ content: text, sha }, { onError: (e) => errorCode(e) === 'workflows_changed' && file.refetch() })}
          >
            <Icon name="pr" size={16} />
            Open a pull request
          </Button>
        </div>
      )}
      {/* Under the button that sent it. */}
      {save.error && (
        <Problem alert className="whitespace-pre-wrap">
          {save.error.message}
          {errorCode(save.error) === 'workflows_changed' && ' Opening a pull request again replaces the newer file with yours.'}
        </Problem>
      )}
    </Tile>
  )
}
