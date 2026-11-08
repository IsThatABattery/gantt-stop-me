import { settingsLabels } from '../../labels/labels'

// In-page sections, reached by hash (e.g. /settings#autonomy) so the router doesn't change.
export const sectionIds = {
  profile: 'profile',
  autonomy: 'autonomy',
  roles: 'roles',
  notifications: 'notifications',
  integrations: 'integrations',
} as const

export const sectionNav = [
  { id: sectionIds.profile, label: settingsLabels.profile },
  { id: sectionIds.autonomy, label: settingsLabels.autonomy },
  { id: sectionIds.roles, label: settingsLabels.roles },
  { id: sectionIds.notifications, label: settingsLabels.notifications },
  { id: sectionIds.integrations, label: settingsLabels.integrations },
]
