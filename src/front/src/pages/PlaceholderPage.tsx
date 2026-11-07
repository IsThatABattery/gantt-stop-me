import { useOutletContext } from 'react-router'
import { Eyebrow } from '../components/Eyebrow'
import type { Project } from '../data/types'
import { labels } from '../labels/labels'

/** Stand-in for screens not built yet. Lives inside the app shell. */
export function PlaceholderPage({ title }: { title: string }) {
  const project = useOutletContext<Project | undefined>()
  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <Eyebrow>{project ? `${labels.project} · ${project.name}` : labels.appName}</Eyebrow>
        <h1 className="mt-1.5 mb-0 font-heading text-[26px] font-semibold">{title}</h1>
      </header>
      <div
        role="status"
        className="max-w-xl rounded-[14px] border border-dashed border-line-strong bg-surface/60 p-6 leading-normal text-text-muted-strong"
      >
        <div className="mb-1 font-medium text-text">Not built yet</div>
        This screen is planned for a later phase.
      </div>
    </div>
  )
}
