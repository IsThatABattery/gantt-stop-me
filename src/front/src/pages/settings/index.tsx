import { useEffect } from 'react'
import { Link, useLocation } from 'react-router'
import { Eyebrow } from '../../components/Eyebrow'
import { StatusMessage } from '../../components/StatusMessage'
import { useMe } from '../../data/session'
import { labels, settingsLabels } from '../../labels/labels'
import { GitHubSection } from './GitHubSection'
import { PasswordForm } from './PasswordForm'
import { ProvidersSection } from './ProvidersSection'
import { Section } from './Section'
import { sectionIds, sectionNav } from './sections'

// Settings, from Settings.dc.html, with the sections the API supports: the
// account, the GitHub link and LLM providers. Each section saves on its own.
export function SettingsPage() {
  const { data: me } = useMe()
  const { hash } = useLocation()
  const active = hash.slice(1) || sectionNav[0].id

  // Section links only change the hash; bring that section into view.
  useEffect(() => {
    if (hash) document.getElementById(hash.slice(1))?.scrollIntoView?.({ block: 'start' })
  }, [hash])

  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <Eyebrow>{me?.email ?? '…'}</Eyebrow>
        <h1 className="mt-1.5 mb-0 font-heading text-[26px] font-semibold">{labels.settings}</h1>
      </header>

      <div className="flex flex-wrap items-start gap-7">
        <nav aria-label="Settings sections" className="flex flex-[1_1_180px] flex-col gap-0.5 lg:sticky lg:top-6 lg:max-w-60">
          {sectionNav.map((s) => (
            <Link
              key={s.id}
              to={{ hash: s.id }}
              aria-current={s.id === active ? 'true' : undefined}
              className={`flex min-h-11 items-center rounded-lg border px-3 no-underline ${
                s.id === active ? 'border-line bg-surface font-medium text-text' : 'border-transparent text-text-muted-strong hover:text-text'
              }`}
            >
              {s.label}
            </Link>
          ))}
        </nav>

        {!me ? (
          <StatusMessage title={`Loading ${labels.settings.toLowerCase()}…`} />
        ) : (
          <div className="flex min-w-0 flex-[999_1_520px] flex-col gap-5 lg:max-w-[820px]">
            <Section id={sectionIds.account} title={settingsLabels.account}>
              <p className="m-0 text-[13px] text-text-muted-strong">
                Signed in as <span className="text-text">{me.email}</span>
              </p>
              <PasswordForm />
            </Section>
            <GitHubSection me={me} />
            <ProvidersSection />
          </div>
        )}
      </div>
    </div>
  )
}
