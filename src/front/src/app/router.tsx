import { createBrowserRouter, Navigate, Outlet, type RouteObject } from 'react-router'
import { labels } from '../labels/labels'
import { ActivityPage } from '../pages/activity'
import { AgentsPage } from '../pages/agents'
import { BoardPage } from '../pages/board/BoardPage'
import { HangarPage } from '../pages/hangar/HangarPage'
import { FeatureEditPage, NewFeatureDialog, NewTaskDialog, TaskEditPage } from '../pages/forms'
import { LoginPage } from '../pages/LoginPage'
import { NotFoundPage } from '../pages/NotFoundPage'
import { PlaceholderPage } from '../pages/PlaceholderPage'
import { BriefingPage, DocReviewPage } from '../pages/planner/doc'
import { PlanDonePage, TaskReviewPage } from '../pages/planner/tasks'
import { ProjectSelectPage } from '../pages/projects/ProjectSelectPage'
import { SettingsPage } from '../pages/settings'
import { TaskDrawerPage } from '../pages/task-drawer'
import { AppShell } from './AppShell'
import { PlannerLayout } from './PlannerLayout'
import { ProjectLayout } from './ProjectLayout'
import { RequireSession } from './RequireSession'

// Every route in the app. Each view branch replaces only its own page
// component; change this file through a small PR into feature/frontend.
export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireSession />,
    children: [
      // feature/frontend-projects
      { path: '/', element: <ProjectSelectPage /> },
      {
        path: '/settings',
        element: (
          <AppShell>
            <Outlet />
          </AppShell>
        ),
        // feature/frontend-settings
        children: [{ index: true, element: <SettingsPage /> }],
      },
      {
        path: '/projects/:projectId',
        element: <ProjectLayout />,
        children: [
          {
            // feature/frontend-board; drawer and dialogs open over it
            element: <BoardPage />,
            children: [
              { index: true, element: null },
              { path: 'tasks/:taskId', element: <TaskDrawerPage /> }, // feature/frontend-task-drawer
              { path: 'new/feature', element: <NewFeatureDialog /> }, // feature/frontend-forms
              { path: 'new/task', element: <NewTaskDialog /> }, // feature/frontend-forms
            ],
          },
          { path: 'features/:featureId/edit', element: <FeatureEditPage /> }, // feature/frontend-forms
          { path: 'tasks/:taskId/edit', element: <TaskEditPage /> }, // feature/frontend-forms
          { path: 'agents', element: <AgentsPage /> }, // feature/frontend-agents
          { path: 'activity', element: <ActivityPage /> }, // feature/frontend-activity
          {
            // The Hangar tab: a list of the Hangar column; the drawer opens over it
            path: 'backlog',
            element: <HangarPage />,
            children: [
              { index: true, element: null },
              { path: 'tasks/:taskId', element: <TaskDrawerPage /> },
            ],
          },
          // Not designed yet, no branch
          { path: 'sprints', element: <PlaceholderPage title={labels.sprints} /> },
          { path: 'retro', element: <PlaceholderPage title={labels.retro} /> },
        ],
      },
      {
        path: '/projects/:projectId/plan/:draftId',
        element: <PlannerLayout />,
        children: [
          { index: true, element: <Navigate to="briefing" replace /> },
          { path: 'briefing', element: <BriefingPage /> }, // feature/frontend-planner-doc
          { path: 'doc', element: <DocReviewPage /> }, // feature/frontend-planner-doc
          { path: 'tasks', element: <TaskReviewPage /> }, // feature/frontend-planner-tasks
          { path: 'done', element: <PlanDonePage /> }, // feature/frontend-planner-tasks
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
]

export const router = createBrowserRouter(routes)
