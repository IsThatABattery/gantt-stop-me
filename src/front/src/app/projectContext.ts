import { useOutletContext } from 'react-router'
import type { Project } from '../data/api/types'
import type { Project as MockProject } from '../data/types'

/** The project loaded by ProjectLayout, for pages under /projects/:projectId. */
export const useProjectContext = () => useOutletContext<Project>()

/**
 * The mock project the hidden mock-data screens expect. Nothing routes to
 * them now, so this only keeps them compiling until their API exists
 * (docs/implementation/api-gaps.md).
 */
export const useMockProjectContext = () => useOutletContext<MockProject>()
