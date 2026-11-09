import { Link, useLocation, useNavigate } from 'react-router'
import { buttonClass } from '../components/buttonClass'
import { Button, ButtonMark } from '../components/controls'
import { Icon } from '../components/Icon'
import { Logo } from '../components/Logo'
import { PageTitle } from '../components/PageTitle'
import { labels } from '../labels/labels'

/** Any address the app doesn't route: says which one, and offers the way back to the projects. */
export function NotFoundPage() {
  const { pathname, key } = useLocation()
  const navigate = useNavigate()
  // The first page of a visit has the default key: there's nowhere in the app to go back to.
  const cameFromApp = key !== 'default'
  return (
    <div className="stars box-border flex min-h-screen flex-col bg-ground text-ink">
      <header className="mx-auto box-border w-full max-w-[1180px] px-4 pt-3 sm:px-8 sm:pt-5">
        <Link to="/" className="inline-flex min-h-11 items-center rounded-[3px] no-underline">
          <Logo />
        </Link>
      </header>
      <main className="mx-auto box-border flex w-full max-w-[1180px] flex-1 flex-col justify-center gap-5 px-4 pt-10 pb-24 sm:px-8">
        <PageTitle size="lg" className="lg:text-[88px]">
          Page not found
        </PageTitle>
        <p className="m-0 max-w-[56ch] text-base leading-normal text-ink-2 lg:text-[17px]">
          Nothing lives at{' '}
          <code className="rounded-[2px] bg-block px-1.5 py-0.5 font-mono text-[0.9em] font-semibold break-all text-ink">{pathname}</code>. The link may be
          out of date, or the page may have moved.
        </p>
        <div className="mt-2 flex flex-wrap gap-3">
          <Link to="/" className={buttonClass('primary', 'min-h-[52px] pl-2 pr-[18px]')}>
            <ButtonMark icon="projects" />
            Go to your {labels.projects.toLowerCase()}
          </Link>
          {cameFromApp && (
            <Button onClick={() => navigate(-1)} className="min-h-[52px] pr-5 pl-4">
              <Icon name="back" size={16} />
              Go back
            </Button>
          )}
        </div>
      </main>
    </div>
  )
}
