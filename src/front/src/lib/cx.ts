// Joins a component's base classes with caller overrides. When an override
// sets one of these properties, the base class for it is dropped, so the
// result doesn't depend on Tailwind's stylesheet order.
const GROUPS = ['min-h-', 'px-', 'py-', 'bg-', 'w-']

const groupOf = (cls: string) => GROUPS.find((g) => cls.startsWith(g))

export function cx(base: string, overrides = ''): string {
  const extra = overrides.split(/\s+/).filter(Boolean)
  const taken = new Set(extra.map(groupOf).filter(Boolean))
  const kept = base.split(/\s+/).filter((c) => c && !taken.has(groupOf(c)))
  return [...kept, ...extra].join(' ')
}
