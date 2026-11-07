# Agents run by shelling out to a multi-provider coding-agent CLI

The scoping, test-writing and browser agents need a tool-using agent loop, and the team wants to try cheaper or local models (Qwen) later without rewriting anything. We do not write an agent loop. The Go binary writes a prompt and a project-local config (model, MCP servers such as Playwright), runs OpenCode non-interactively (`opencode run`, with its model selection and machine-readable output options, confirmed against the installed version) inside the Project checkout, and parses the result. The browser agent gets Playwright's MCP server and is told to start video recording through its `browser_start_video` tool, which writes a `.webm` into the server's output directory. Providers are OpenAI-compatible endpoints the Project configures explicitly (Anthropic, OpenAI, Qwen, OpenRouter, Ollama and vLLM all expose one), each agent is assigned a model from them, and the tool writes them into OpenCode's config as custom providers, so model choice is configuration, not code. The cost is Node in the runner and a dependency on a fast-moving CLI's flags and JSON shape, kept behind one Go package so it can be swapped.

## Considered options

- Claude Code CLI: the strongest harness today, but Anthropic models only (plus Bedrock and Vertex), so routing later would mean a second harness.
- Our own Go tool-use loop: roughly three hundred lines for file tools and double that with browser tools over MCP; full control, and all of its quality to own.

## Consequences

- `gantt.yaml` carries the Providers (name, base URL, chosen model ids) and an `agents` section with a Project default model and optional per-agent models for scoping, test-writing and browser. Keys never enter the repo: a git-ignored `.env` locally and repo secrets in CI, both written by the settings page and by `gantt init`.
- Only OpenAI-compatible endpoints exist as a Provider type. Models are discovered from the endpoint's model listing, with a manual model id for endpoints that do not list them. A native Anthropic type is a later addition if some feature needs it.
- The tool generates OpenCode's config per run from these values, so OpenCode's own config format never appears in a Project.
