// Names the API types and the views share.

export type ID = string

/** A board column's key, as the API gives it (docs/api/board.md) */
export type ColumnKey = 'backlog' | 'in-progress' | 'testing' | 'user-check' | 'closed'

/** The columns in board order */
export const COLUMN_ORDER: ColumnKey[] = ['backlog', 'in-progress', 'testing', 'user-check', 'closed']
