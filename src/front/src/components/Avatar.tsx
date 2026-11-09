import type { Agent, Autonomy, User } from '../data/types'
import { autonomyLabels, labels, roleLabels } from '../labels/labels'
import { Icon } from './Icon'

const userSizes = {
  sm: 'size-[22px] text-[10px]',
  md: 'size-[26px] text-[11px]',
  lg: 'size-8 text-xs',
}

interface UserAvatarProps {
  user: Pick<User, 'initials' | 'role'>
  size?: keyof typeof userSizes
  /** A paper gap that separates stacked avatars */
  stacked?: boolean
}

/** A person: their initials on a square ink-2 block with a 2px radius. */
export function UserAvatar({ user, size = 'md', stacked }: UserAvatarProps) {
  return (
    <span
      role="img"
      aria-label={`${roleLabels[user.role]} ${user.initials}`}
      className={`inline-flex flex-none items-center justify-center rounded-[2px] bg-ink-2 leading-none font-bold text-on-ink ${userSizes[size]} ${
        stacked ? '-mr-1 outline-2 outline-paper' : ''
      }`}
    >
      {user.initials}
    </span>
  )
}

/** One to three small squares for Supervised, Guided and Autonomous. */
export function AutonomyRings({ level, size = 14 }: { level: Autonomy; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden="true" focusable="false">
      {[0, 1, 2].map((i) => (
        <rect
          key={i}
          x={1 + i * 5}
          y={5}
          width={4}
          height={6}
          fill={i <= level ? 'currentColor' : 'none'}
          stroke="currentColor"
          strokeWidth={1}
        />
      ))}
    </svg>
  )
}

interface AgentAvatarProps {
  agent: Pick<Agent, 'name' | 'autonomy'>
  /** Working right now */
  active?: boolean
  /** It has asked a person a question */
  hail?: boolean
  size?: 'md' | 'lg'
}

/** An agent: its mark on a paper square with a 1.5px ink edge, its autonomy as squares below. */
export function AgentAvatar({ agent, active, hail, size = 'lg' }: AgentAvatarProps) {
  const box = size === 'lg' ? 'size-10' : 'size-7'
  return (
    <span
      role="img"
      aria-label={`${labels.agent} ${agent.name}, ${autonomyLabels[agent.autonomy]}${hail ? `, ${labels.hail} raised` : ''}`}
      className={`relative inline-flex flex-none items-center justify-center rounded-[2px] border-[1.5px] border-ink ${
        active ? 'bg-ink text-on-ink' : 'bg-paper text-ink'
      } ${box}`}
    >
      <Icon name="agent" size={size === 'lg' ? 20 : 16} />
      <span className="absolute -right-1.5 -bottom-1.5 rounded-[2px] bg-paper px-px text-ink">
        <AutonomyRings level={agent.autonomy} />
      </span>
      {hail && <span className="absolute -top-1 -right-1 size-2.5 rounded-[1px] border-2 border-paper bg-approve" />}
    </span>
  )
}

interface AgentTagProps {
  agent: Pick<Agent, 'name' | 'autonomy'>
  active?: boolean
  hail?: boolean
}

/** Compact agent mark + name block, used as the owner on a card. */
export function AgentTag({ agent, active, hail }: AgentTagProps) {
  return (
    <span
      className={`relative inline-flex h-6 items-center gap-1.5 rounded-[2px] border border-ink pr-2 pl-1 text-xs font-semibold ${
        active ? 'bg-ink text-on-ink' : 'bg-paper text-ink'
      }`}
    >
      <Icon name="agent" size={14} />
      <span className="sr-only">{labels.agent} </span>
      {agent.name}
      <span className="sr-only">, {autonomyLabels[agent.autonomy]}</span>
      <AutonomyRings level={agent.autonomy} size={12} />
      {hail && (
        <span className="absolute -top-1 -right-1 size-2.5 rounded-[1px] border-2 border-paper bg-approve">
          <span className="sr-only">, {labels.hail} raised</span>
        </span>
      )}
    </span>
  )
}
