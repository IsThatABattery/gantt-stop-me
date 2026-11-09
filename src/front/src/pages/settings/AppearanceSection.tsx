import { RadioCard } from '../../components/RadioCard'
import { settingsLabels } from '../../labels/labels'
import { useTheme } from '../../theme/theme'
import { themes, type ThemeId } from '../../theme/themes'
import { Section } from './Section'
import { sectionIds } from './sections'

/** The color theme for this browser. Picking one applies it at once; there's nothing to save. */
export function AppearanceSection() {
  const [theme, setTheme] = useTheme()
  return (
    <Section id={sectionIds.appearance} title={settingsLabels.appearance} intro={settingsLabels.appearanceHelp}>
      <fieldset className="m-0 border-0 p-0">
        <legend className="mb-2.5 p-0 text-[13px] text-text-soft">{settingsLabels.theme}</legend>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3">
          {themes.map((t) => (
            <RadioCard
              key={t.id}
              name="theme"
              value={t.id}
              checked={theme === t.id}
              onChange={() => setTheme(t.id)}
              label={t.name}
              description={t.description}
              preview={<ThemePreview id={t.id} />}
            />
          ))}
        </div>
      </fieldset>
    </Section>
  )
}

/** A miniature of the app: a rail, a header with a primary button, a card and an astronaut's card. data-theme scopes the colors to it. */
function ThemePreview({ id }: { id: ThemeId }) {
  return (
    <span data-theme={id} aria-hidden="true" className="stars mb-1 flex h-24 overflow-hidden rounded-lg border border-line bg-background">
      <span className="flex w-9 flex-none flex-col gap-1.5 border-r border-line-soft bg-rail p-1.5">
        <span className="h-2.5 rounded-sm bg-nav-active" />
        <span className="h-1.5 w-4/5 rounded-full bg-text-muted" />
        <span className="h-1.5 w-3/5 rounded-full bg-text-muted" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-2 p-2">
        <span className="flex items-center justify-between gap-2">
          <span className="h-2 w-2/5 rounded-full bg-text" />
          <span className="h-3.5 w-9 rounded-sm bg-ion" />
        </span>
        <span className="flex flex-1 gap-1.5">
          <span className="flex flex-1 flex-col gap-1 rounded border border-line bg-surface p-1.5">
            <span className="h-1 rounded-full bg-good" />
            <span className="h-1.5 w-4/5 rounded-full bg-text-soft" />
            <span className="h-1.5 w-1/2 rounded-full bg-text-muted" />
          </span>
          <span className="flex flex-1 flex-col gap-1 rounded border border-astronaut bg-astronaut/10 p-1.5">
            <span className="h-1 rounded-full bg-warning" />
            <span className="h-1.5 w-4/5 rounded-full bg-text-soft" />
            <span className="size-2 rounded-full bg-astronaut" />
          </span>
        </span>
      </span>
    </span>
  )
}
