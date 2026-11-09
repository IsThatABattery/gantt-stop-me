import type { ReactNode } from 'react'
import { NavRail } from '../components/NavRail'

/**
 * The rail on the left, 224px, and the page on the ground beside it. Below md the rail becomes a bar above the page.
 * stars is the Space themes' starfield; the other themes set it to none.
 */
export function AppShell({ projectId, children }: { projectId?: string; children: ReactNode }) {
  return (
    // A column below md, so the rail's bar keeps its own height at the top instead of stretching down the screen.
    <div className="stars flex min-h-screen flex-col bg-ground text-ink md:grid md:grid-cols-[224px_minmax(0,1fr)] md:items-start">
      <NavRail projectId={projectId} />
      <main className="flex min-h-screen min-w-0 flex-1 flex-col max-md:min-h-0">{children}</main>
    </div>
  )
}
