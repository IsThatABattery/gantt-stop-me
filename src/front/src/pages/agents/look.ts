import type { Tone } from '../../components/Chip'
import type { IconName } from '../../components/Icon'
import type { AgentState, TraceKind } from '../../data/types'

/** Color meaning from AGENT_BRIEF §9; every state also has a word and an icon. */
export const stateLook: Record<AgentState, { tone: Tone; icon: IconName }> = {
  idle: { tone: 'neutral', icon: 'hangar' },
  planning: { tone: 'astronaut', icon: 'target' },
  working: { tone: 'astronaut', icon: 'orbit' },
  hail: { tone: 'warning', icon: 'chat' },
  docking: { tone: 'warning', icon: 'dock' },
  blocked: { tone: 'critical', icon: 'alert' },
}

/** "52m", "1h 10m" */
export function timeOnTask(minutes: number) {
  if (minutes < 60) return `${minutes}m`
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return m ? `${h}h ${m}m` : `${h}h`
}

/** Plain lowercase kind for the mono trace, as drawn in ProbeConsole.dc.html. */
export const traceWord: Record<TraceKind, string> = {
  plan: 'plan',
  read: 'read',
  edit: 'edit',
  run: 'run',
  result: 'result',
  hail: 'hail',
  docking: 'docking',
}

export const clockTime = (iso: string) => new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
