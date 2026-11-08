import { Link } from 'react-router'
import { buttonClass } from '../components/buttonClass'
import { StatusMessage } from '../components/StatusMessage'
import { labels } from '../labels/labels'

export function NotFoundPage() {
  return (
    <div className="stars flex min-h-screen bg-background text-text">
      <StatusMessage title="Page not found" body="This page does not exist.">
        <Link to="/" className={buttonClass('accent')}>
          {labels.allProjects}
        </Link>
      </StatusMessage>
    </div>
  )
}
