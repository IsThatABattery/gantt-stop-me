import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router'
import { PageTitle } from '../../components/PageTitle'
import { useSignedInUser } from '../../data/session'
import { labels, settingsLabels } from '../../labels/labels'
import { AppearanceSection } from './AppearanceSection'
import { GitHubSection } from './GitHubSection'
import { PasswordForm } from './PasswordForm'
import { PlanningSection } from './PlanningSection'
import { ProvidersSection } from './ProvidersSection'
import { Card, Section } from './Section'
import { sectionIds, sectionNav } from './sections'

/** How far below the top of the window a section's tab must reach before it counts as the one being read */
const READING_LINE = 120

/**
 * The section being read: the last one whose top has passed the reading line. At the foot of the page, where the
 * last sections can't scroll up to the line, the linked one wins while it's in view, and otherwise the last one.
 */
function sectionInView(linked: string | null) {
  const tops = sectionNav.map((s) => ({ id: s.id, rect: document.getElementById(s.id)?.getBoundingClientRect() }))
  const atFoot = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2
  if (atFoot) {
    const target = tops.find((t) => t.id === linked)?.rect
    if (target && target.top < window.innerHeight && target.bottom > 0) return linked
    return sectionNav[sectionNav.length - 1].id
  }
  return tops.reduce((current, t) => (t.rect && t.rect.top <= READING_LINE ? t.id : current), sectionNav[0].id)
}

// Settings, with the sections the API supports: the account, this browser's theme, the GitHub link, LLM providers
// and the planning agents. Each section saves on its own.
export function SettingsPage() {
  // While the session-ended dialog is up, the last signed-in user keeps the sections and what was typed in them.
  const { data: me } = useSignedInUser()
  const { hash } = useLocation()
  const linked = hash.slice(1) || null
  // Follows the scroll, so the ink block in the section nav is the section on screen.
  const [reading, setReading] = useState<string | null>(null)
  const active = reading ?? linked ?? sectionNav[0].id

  // Section links only change the hash; bring that section into view.
  useEffect(() => {
    if (linked) document.getElementById(linked)?.scrollIntoView?.({ block: 'start' })
  }, [linked])

  useEffect(() => {
    if (!me) return
    const onScroll = () => setReading(sectionInView(linked))
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [me, linked])

  return (
    <div className="flex flex-col gap-6 px-4 pt-5 pb-12 md:px-7">
      <header className="flex flex-col gap-2.5">
        <PageTitle>{labels.settings}</PageTitle>
        <p className="m-0 text-[15px] text-ink-2">Each section saves on its own.</p>
      </header>

      <div className="flex flex-col gap-5 lg:grid lg:grid-cols-[200px_minmax(0,860px)] lg:items-start lg:gap-7">
        <nav aria-label="Settings sections" className="lg:sticky lg:top-5">
          <ul className="m-0 flex list-none flex-wrap gap-1.5 p-0 lg:flex-col">
            {sectionNav.map((s) => {
              const current = s.id === active
              return (
                <li key={s.id}>
                  <Link
                    to={{ hash: s.id }}
                    aria-current={current ? 'true' : undefined}
                    onClick={() => {
                      setReading(null)
                      // The hash won't change, so nothing else brings it back into view.
                      if (linked === s.id) document.getElementById(s.id)?.scrollIntoView?.({ block: 'start' })
                    }}
                    className={`flex min-h-11 items-center rounded-[3px] px-3.5 text-sm font-semibold no-underline transition-colors duration-150 focus-visible:outline-offset-2 ${
                      current ? 'bg-ink text-on-ink hover:bg-ink-hover hover:text-on-ink' : 'bg-block text-ink hover:bg-block-hover hover:text-ink active:bg-block-hover'
                    }`}
                  >
                    {s.label}
                  </Link>
                </li>
              )
            })}
          </ul>
        </nav>

        {!me ? (
          <p role="status" className="m-0 text-sm text-ink-2">
            Loading {labels.settings.toLowerCase()}…
          </p>
        ) : (
          <div className="flex min-w-0 flex-col gap-6">
            <Section
              id={sectionIds.account}
              title={settingsLabels.account}
              intro={
                <>
                  Signed in as <span className="font-semibold text-ink">{me.email}</span>
                </>
              }
            >
              <Card>
                <PasswordForm />
              </Card>
            </Section>
            <AppearanceSection />
            <GitHubSection me={me} />
            <ProvidersSection />
            <PlanningSection />
          </div>
        )}
      </div>
    </div>
  )
}
