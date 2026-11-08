import { useState } from 'react'
import { useProjectContext } from '../../app/projectContext'
import { Eyebrow } from '../../components/Eyebrow'
import { StatusMessage } from '../../components/StatusMessage'
import { useActivity, useActivityContext } from '../../data/views/activity'
import { labels } from '../../labels/labels'
import { ActivityFeed, ActivityFilters } from './ActivityFeed'
import { filterEvents, type ActivityFilter } from './feed'

const emptyText: Record<ActivityFilter, string> = {
  all: `No activity on this ${labels.project.toLowerCase()} yet.`,
  users: `No ${labels.user.toLowerCase()} activity yet.`,
  agents: `No ${labels.agent.toLowerCase()} activity yet.`,
  'needs-action': 'Nothing needs you right now.',
}

// Mission log: every move, hail, PR and approval on the mission (AGENT_BRIEF §5).
export function ActivityPage() {
  const project = useProjectContext()
  const events = useActivity(project.id)
  const context = useActivityContext(project.id)
  const [filter, setFilter] = useState<ActivityFilter>('all')

  const error = events.error ?? context.error
  const needsAction = events.data?.filter((e) => e.needsAction).length ?? 0

  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <Eyebrow>
          {labels.project} · {project.name}
        </Eyebrow>
        <h1 className="mt-2 mb-0 font-heading text-[26px] font-semibold">{labels.activity}</h1>
        {events.data && events.data.length > 0 && (
          <p className="m-0 mt-2 text-[13px] text-text-muted-strong">
            {events.data.length} entries · {needsAction > 0 ? `${needsAction} need action` : 'nothing needs you'}
          </p>
        )}
      </header>

      {error ? (
        <StatusMessage title="Something went wrong" body={error.message} />
      ) : !events.data || !context.data ? (
        <StatusMessage title={`Loading ${labels.activity.toLowerCase()}…`} />
      ) : (
        <div className="flex max-w-3xl flex-col gap-5">
          <ActivityFilters events={events.data} value={filter} onChange={setFilter} />
          <ActivityFeed
            events={filterEvents(events.data, filter)}
            users={context.data.users}
            agents={context.data.agents}
            tasks={context.data.tasks}
            emptyText={emptyText[filter]}
          />
        </div>
      )}
    </div>
  )
}
