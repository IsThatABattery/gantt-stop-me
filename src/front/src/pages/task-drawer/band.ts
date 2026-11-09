// The approve band: a yellow strip wherever a person approves (a task in User Check, a plan doc to review).
// Everything on it is drawn in on-approve, the band's own ink, which stays near-black in every theme, so the
// buttons and focus ring keep their contrast on yellow whatever ink is elsewhere.

export const band = 'bg-approve text-on-approve'

const bandButton =
  'inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-control border-[1.5px] border-on-approve px-4 font-[inherit] text-sm leading-tight no-underline transition-colors duration-150 ease-out-quint focus-visible:outline-on-approve disabled:cursor-not-allowed aria-busy:cursor-progress'

/** The band's primary action: an ink button with an approve-yellow label. Until it can be used it is only an outline. */
export const bandPrimary = `${bandButton} bg-on-approve font-bold text-approve enabled:hover:bg-on-approve/85 enabled:active:bg-on-approve disabled:border-on-approve/35 disabled:bg-transparent disabled:text-on-approve/65`

/** Every other action on the band: an ink outline. */
export const bandSecondary = `${bandButton} bg-transparent font-semibold text-on-approve enabled:hover:bg-on-approve/10 enabled:active:bg-on-approve/20 disabled:opacity-45`

/** The band's mark: an ink square holding the person icon, as the board's tabs light for approval. */
export const bandMark = 'grid size-8 flex-none place-items-center rounded-[2px] bg-on-approve text-approve'
