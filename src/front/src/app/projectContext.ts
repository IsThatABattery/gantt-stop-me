import { useOutletContext } from 'react-router'
import type { Project } from '../data/api/types'

/** The project loaded by ProjectLayout, for pages under /projects/:projectId. */
export const useProjectContext = () => useOutletContext<Project>()
