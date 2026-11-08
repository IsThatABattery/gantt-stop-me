import { useOutletContext } from 'react-router'
import type { Project } from '../data/types'

/** The project loaded by ProjectLayout, for pages under /projects/:projectId. */
export const useProjectContext = () => useOutletContext<Project>()
