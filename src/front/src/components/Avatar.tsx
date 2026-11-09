import type { Agent, Autonomy, User } from '../data/types'
import { autonomyLabels, labels, roleLabels } from '../labels/labels'
import { Icon } from './Icon'

const userSizes = {
  sm: 'size-[26px] text-[11px]',
  md: 'size-7 text-[11px]',
  lg: 'size-10 text-sm',
}

interface UserAvatarProps {
  user: Pick<User, 'initials' | 'role'>
  size?: keyof typeof userSizes
  /** Ring that separates stacked avatars */
  stacked?: boolean
}

export function UserAvatar({ user, size = 'md', stacked }: UserAvatarProps) {
  return (
    <span
      role="img"
      aria-label={`${roleLabels[user.role]} ${user.initials}`}
      className={`inline-flex flex-none items-center justify-center rounded-full bg-line font-semibold text-text ${userSizes[size]} ${
        stacked ? '-mr-1.5 border-2 border-surface' : ''
      }`}
    >
      {user.initials}
    </span>
  )
}

/** 1, 2 or 3 orbit rings for Tethered, Guided, Free flight. */
export function AutonomyRings({ level, size = 14 }: { level: Autonomy; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="var(--astronaut)" strokeWidth="1.2" aria-hidden="true">
      {[0, 1, 2].slice(0, level + 1).map((i) => (
        <ellipse key={i} cx="8" cy="8" rx={3 + i * 2.2} ry={1.4 + i * 1.1} transform="rotate(-25 8 8)" />
      ))}
    </svg>
  )
}

interface AgentAvatarProps {
  agent: Pick<Agent, 'name' | 'autonomy'>
  /** Slow orbit pulse while the agent is working */
  active?: boolean
  /** Amber dot when the agent has raised a hail */
  hail?: boolean
  size?: 'md' | 'lg'
}

export function AgentAvatar({ agent, active, hail, size = 'lg' }: AgentAvatarProps) {
  const box = size === 'lg' ? 'size-10 rounded-[10px]' : 'size-7 rounded-lg'
  return (
    <span
      role="img"
      aria-label={`${labels.agent} ${agent.name}, ${autonomyLabels[agent.autonomy]}${hail ? `, ${labels.hail} raised` : ''}`}
      className={`relative inline-flex flex-none items-center justify-center border border-astronaut bg-astronaut/10 text-astronaut ${box} ${
        active ? 'animate-orbit-pulse' : ''
      }`}
    >
      <Icon name="helmet" size={size === 'lg' ? 20 : 16} />
      <span className="absolute -right-1.5 -bottom-1.5 rounded-full bg-background p-px">
        <AutonomyRings level={agent.autonomy} />
      </span>
      {hail && <span className="absolute -top-1 -right-1 size-2.5 rounded-full border-2 border-background bg-warning" />}
    </span>
  )
}

interface AgentTagProps {
  agent: Pick<Agent, 'name' | 'autonomy'>
  active?: boolean
  hail?: boolean
}

/** Compact helmet + name chip, used as the owner on a card. */
export function AgentTag({ agent, active, hail }: AgentTagProps) {
  return (
    <span
      className={`relative inline-flex h-[26px] items-center gap-1.5 rounded-lg border border-astronaut pr-2 pl-1 text-xs text-astronaut-text ${
        active ? 'animate-orbit-pulse' : ''
      }`}
    >
      <Icon name="helmet" size={16} className="text-astronaut" />
      <span className="sr-only">{labels.agent} </span>
      {agent.name}
      <span className="sr-only">, {autonomyLabels[agent.autonomy]}</span>
      <AutonomyRings level={agent.autonomy} size={12} />
      {hail && (
        <span className="absolute -top-1 -right-1 size-2.5 rounded-full border-2 border-surface bg-warning">
          <span className="sr-only">, {labels.hail} raised</span>
        </span>
      )}
    </span>
  )
}
