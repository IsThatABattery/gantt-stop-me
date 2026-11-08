import { useEffect } from 'react'
import { Link, useLocation } from 'react-router'
import { Eyebrow } from '../../components/Eyebrow'
import { StatusMessage } from '../../components/StatusMessage'
import { useUser, useWorkspace } from '../../data/core'
import { useSession } from '../../data/session'
import { useSettings } from '../../data/views/settings'
import { labels } from '../../labels/labels'
import { sectionNav } from './sections'
import { SettingsForm } from './SettingsForm'

// Settings, from Settings.dc.html. Profile isn't drawn, so it reuses the section-card style.
export function SettingsPage() {
  const session = useSession()
  const settings = useSettings()
  const user = useUser(session?.userId)
  const { data: workspace } = useWorkspace()
  const { hash } = useLocation()
  const active = hash.slice(1) || sectionNav[0].id

  // Section links only change the hash; bring that section into view.
  useEffect(() => {
    if (hash) document.getElementById(hash.slice(1))?.scrollIntoView?.({ block: 'start' })
  }, [hash])

  const error = settings.error ?? user.error

  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <Eyebrow>
          {labels.workspace} · {workspace?.name ?? '…'}
        </Eyebrow>
        <h1 className="mt-2 mb-0 font-heading text-[26px] font-semibold">{labels.settings}</h1>
      </header>

      <div className="flex flex-wrap items-start gap-7">
        <nav aria-label="Settings sections" className="flex flex-[1_1_180px] flex-col gap-1 lg:sticky lg:top-6 lg:max-w-60">
          {sectionNav.map((s) => (
            <Link
              key={s.id}
              to={{ hash: s.id }}
              aria-current={s.id === active ? 'true' : undefined}
              className={`flex min-h-11 items-center rounded-control border px-3 no-underline ${
                s.id === active ? 'border-line bg-surface font-medium text-text' : 'border-transparent text-text-muted-strong hover:text-text'
              }`}
            >
              {s.label}
            </Link>
          ))}
        </nav>

        {error ? (
          <StatusMessage title="Something went wrong" body={error.message} />
        ) : !settings.data || !user.data ? (
          <StatusMessage title={`Loading ${labels.settings.toLowerCase()}…`} />
        ) : (
          <SettingsForm settings={settings.data} user={user.data} />
        )}
      </div>
    </div>
  )
}
