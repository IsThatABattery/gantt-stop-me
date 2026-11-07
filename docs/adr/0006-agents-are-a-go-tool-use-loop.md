# Agents are a tool-use loop inside the Go binary, not a third-party harness

The scoping, test-writing and browser agents need a tool-using loop, and the team wants any OpenAI-compatible endpoint (Anthropic, OpenAI, Qwen, OpenRouter, Ollama, vLLM) selectable per agent. We write the loop ourselves in Go against the chat completions API with tool calling. Scoping and test-writing get four tools: read file, list directory, write file, run command. The browser agent gets navigate, click, type, page snapshot and screenshot, backed by a Go Playwright driver, with the Recording made through Playwright's own video option. Every agent is a few hundred lines we own, and the runner needs only the gantt binary and the browser binaries: no Node, no MCP server, no dependency on another tool's flags or output format.

## Considered options

- Shelling out to a coding-agent CLI such as OpenCode or Claude Code: less to own, but Node in the runner, a provider list dictated by the CLI, and a config format of its own leaking into every Project.
- Microsoft's Playwright MCP server through a Go MCP client: a fuller browser toolset, but Node is back in the runner.

## Consequences

- `gantt.yaml` carries the Providers (name, base URL, chosen model ids) and an `agents` section with a Project default model and optional per-agent models for scoping, test-writing and browser. Keys never enter the repo: a git-ignored `.env` locally and repo secrets in CI, both written by the settings page and by `gantt init`.
- Only OpenAI-compatible endpoints exist as a Provider type. Models are discovered from the endpoint's model listing, with a manual model id for endpoints that do not list them.
- Loop quality (retries, context limits, malformed tool calls) is ours to fix. If it proves weak, the shell-out path is the fallback, behind the same agent interface.
