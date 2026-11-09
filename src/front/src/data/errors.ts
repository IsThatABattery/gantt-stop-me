// Errors the API client rejects with, so pages can check `instanceof` and read
// the API's code.

/** A request the API answered with an error: `{ "error": { "code", "message" } }`. */
export class ApiError extends Error {
  /** HTTP status; 0 when the request never got an answer */
  readonly status: number
  /** The API's error code, e.g. `email_taken` */
  readonly code: string
  /** The whole response body, for errors that carry more, e.g. a 409 with the current task */
  readonly body: unknown

  constructor(status: number, code: string, message: string, body: unknown = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.body = body
  }
}

/** 404: the thing doesn't exist, or the user can't see it. */
export class NotFoundError extends ApiError {
  constructor(message = 'Not found', code = 'not_found', body: unknown = null) {
    super(404, code, message, body)
    this.name = 'NotFoundError'
  }
}

/** 409: rejected because something changed first, e.g. a move against a stale version. */
export class ConflictError extends ApiError {
  constructor(message: string, code = 'conflict', body: unknown = null) {
    super(409, code, message, body)
    this.name = 'ConflictError'
  }
}

/** The API's error code, or null when the error didn't come from the API. */
export const errorCode = (e: unknown) => (e instanceof ApiError ? e.code : null)
