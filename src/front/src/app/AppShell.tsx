import type { ReactNode } from 'react'
import { NavRail } from '../components/NavRail'

/** Rail on the left, page on the right. Stacks on narrow widths. */
export function AppShell({ projectId, children }: { projectId?: string; children: ReactNode }) {
  return (
    <div className="stars flex min-h-screen flex-wrap bg-background text-text max-md:content-start">
      <NavRail projectId={projectId} />
      <main className="flex min-w-0 flex-[999_1_560px] flex-col">{children}</main>
    </div>
  )
}
