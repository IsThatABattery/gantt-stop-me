import { settingsLabels } from '../../labels/labels'

// In-page sections, reached by hash (e.g. /settings#github) so the router doesn't change.
export const sectionIds = {
  account: 'account',
  appearance: 'appearance',
  github: 'github',
  providers: 'providers',
  agents: 'agents',
} as const

export const sectionNav = [
  { id: sectionIds.account, label: settingsLabels.account },
  { id: sectionIds.appearance, label: settingsLabels.appearance },
  { id: sectionIds.github, label: settingsLabels.github },
  { id: sectionIds.providers, label: settingsLabels.providers },
  { id: sectionIds.agents, label: settingsLabels.agents },
]
