/**
 * A test as the issue's Tests section writes it (a run result's `text`): its line, then any block under it.
 * A line that is one code span is a command, shown in mono without its backticks. A fenced block is code.
 */
export function splitTestText(text: string): { head: string; command: boolean; body: string; code: boolean } {
  const [first = '', ...rest] = text.split('\n')
  const command = /^`[^`]+`$/.test(first.trim())
  const block = rest.join('\n').trim()
  const fenced = /^```/.test(block)
  return {
    head: command ? first.trim().slice(1, -1) : first,
    command,
    body: fenced ? block.replace(/^```[^\n]*\n?/, '').replace(/\n?```$/, '') : block,
    code: fenced,
  }
}
