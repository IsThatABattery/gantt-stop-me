/**
 * Full-page visits, for API routes that redirect, such as linking with OAuth. An object, so tests can stub `visit`:
 * jsdom can't navigate, and its location.assign can't be spied on.
 */
export const page = {
  visit(url: string) {
    window.location.assign(url)
  },
}
