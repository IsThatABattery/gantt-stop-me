/**
 * A tile's place in a row of count tiles, as the --tile-step the tile utility reads: 0 for the first, 1 for the last.
 * The board's stage tiles step darker left to right however many columns there are. Pass it to Tile's step, or set it
 * on any element using the tile utility: style={{ '--tile-step': tileStep(i, n) } as CSSProperties}.
 */
export function tileStep(index: number, count: number): number {
  return count > 1 ? index / (count - 1) : 0
}
