import { createBrowserRouter, Outlet } from 'react-router'
import { labels } from '../labels/labels'
import { BoardPage } from '../pages/board/BoardPage'
import { LoginPage } from '../pages/LoginPage'
import { NotFoundPage } from '../pages/NotFoundPage'
import { PlaceholderPage } from '../pages/PlaceholderPage'
import { ProjectSelectPage } from '../pages/ProjectSelectPage'
import { AppShell } from './AppShell'
import { ProjectLayout } from './ProjectLayout'
import { RequireSession } from './RequireSession'

const placeholders = [
  { path: 'backlog', title: labels.backlog },
  { path: 'sprints', title: labels.sprints },
  { path: 'agents', title: labels.agents },
  { path: 'activity', title: labels.activity },
  { path: 'retro', title: labels.retro },
]

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireSession />,
    children: [
      { path: '/', element: <ProjectSelectPage /> },
      {
        path: '/settings',
        element: (
          <AppShell>
            <Outlet />
          </AppShell>
        ),
        children: [{ index: true, element: <PlaceholderPage title={labels.settings} /> }],
      },
      {
        path: '/projects/:projectId',
        element: <ProjectLayout />,
        children: [
          { index: true, element: <BoardPage /> },
          ...placeholders.map((p) => ({ path: p.path, element: <PlaceholderPage title={p.title} /> })),
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
])
