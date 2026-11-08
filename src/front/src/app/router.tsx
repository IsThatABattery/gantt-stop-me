import { createBrowserRouter, Outlet, type RouteObject } from 'react-router'
import { AutomationPage } from '../pages/automation'
import { PlanPage, PlansPage } from '../pages/plans'
import { BoardPage } from '../pages/board/BoardPage'
import { NewTaskDialog, TaskEditPage } from '../pages/forms'
import { LoginPage } from '../pages/LoginPage'
import { NotFoundPage } from '../pages/NotFoundPage'
import { AddProjectPage } from '../pages/projects/AddProjectPage'
import { SettingsPage } from '../pages/settings'
import { SignupPage } from '../pages/SignupPage'
import { TaskDrawerPage } from '../pages/task-drawer'
import { AppShell } from './AppShell'
import { ProjectLayout } from './ProjectLayout'
import { RequireSession } from './RequireSession'

// Every route in the app. Only screens the API supports are routed. The
// mock-data screens (project list, objectives, agents, activity, planner,
// backlog, sprints, retro) keep their files but aren't reachable until their
// API exists; see docs/implementation/api-gaps.md.
export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  { path: '/signup', element: <SignupPage /> },
  {
    element: <RequireSession />,
    children: [
      {
        element: (
          <AppShell>
            <Outlet />
          </AppShell>
        ),
        children: [
          { path: '/', element: <AddProjectPage /> },
          { path: '/settings', element: <SettingsPage /> },
        ],
      },
      {
        path: '/projects/:projectId',
        element: <ProjectLayout />,
        children: [
          {
            // The drawer and the create dialog open over the board.
            element: <BoardPage />,
            children: [
              { index: true, element: null },
              { path: 'tasks/:taskId', element: <TaskDrawerPage /> },
              { path: 'new/task', element: <NewTaskDialog /> },
            ],
          },
          { path: 'tasks/:taskId/edit', element: <TaskEditPage /> },
          { path: 'automation', element: <AutomationPage /> },
          { path: 'plans', element: <PlansPage /> },
          { path: 'plans/:planId', element: <PlanPage /> },
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
]

export const router = createBrowserRouter(routes)
