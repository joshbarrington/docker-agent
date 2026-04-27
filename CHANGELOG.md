# Changelog

All notable changes to this project will be documented in this file.


## [v1.51.0] - 2026-04-27

This release improves Anthropic model support on Vertex AI, enhances the model picker interface, and includes several bug fixes.

## What's New
- Adds pricing and capabilities information to the /model picker interface with a detailed comparison table

## Improvements
- Routes Anthropic models on Vertex AI through the native endpoint instead of OpenAI-compatible endpoint to fix compatibility issues

## Bug Fixes
- Fixes race condition in session cleanup that could cause spurious "session busy" errors
- Fixes OTLP endpoint URL handling to properly support http/https schemes

## Technical Changes
- Enables noctx linter and adds context threading through HTTP, SQL, exec and net APIs

### Pull Requests

- [#2476](https://github.com/docker/docker-agent/pull/2476) - Route Anthropic models on Vertex AI through the native endpoint
- [#2489](https://github.com/docker/docker-agent/pull/2489) - ci: bump golangci-lint from v2.9 to v2.11
- [#2499](https://github.com/docker/docker-agent/pull/2499) - docs: update CHANGELOG.md for v1.50.0
- [#2503](https://github.com/docker/docker-agent/pull/2503) - fix(session): prevent race condition in session cleanup
- [#2504](https://github.com/docker/docker-agent/pull/2504) - fix(otel): support http/https scheme in OTLP endpoint URL
- [#2508](https://github.com/docker/docker-agent/pull/2508) - lint: enable noctx and deduplicate touched code
- [#2511](https://github.com/docker/docker-agent/pull/2511) - feat(tui): show pricing & capabilities in /model picker


## [v1.50.0] - 2026-04-23

This release fixes several runtime issues with message steering and sandbox argument handling, along with TUI improvements for user prompts and speech commands.

## What's New

- Adds support for custom OAuth callback redirect URLs for remote MCP toolsets, allowing public-facing proxies for authentication

## Improvements

- Adds custom component for user_prompt tool calls in TUI that shows only status and name without exposing internal details

## Bug Fixes

- Fixes sandbox mode incorrectly interpreting agent file path as first chat message due to duplicate argument handling
- Fixes runtime race conditions where steer messages could be silently dropped during idle windows or first turns
- Fixes /speak slash command not dispatching immediately in TUI

## Technical Changes

- Updates Go to version 1.26.2
- Refactors runtime steer message injection to remove system-reminder envelope

### Pull Requests

- [#2486](https://github.com/docker/docker-agent/pull/2486) - docs: update CHANGELOG.md for v1.49.2
- [#2487](https://github.com/docker/docker-agent/pull/2487) - fix(sandbox): don't duplicate agent file and --config-dir args
- [#2488](https://github.com/docker/docker-agent/pull/2488) - chore: bump Go to 1.26.2
- [#2492](https://github.com/docker/docker-agent/pull/2492) - fix(runtime): drain steerQueue at top of RunStream loop to close idle-window race
- [#2494](https://github.com/docker/docker-agent/pull/2494) - feat(mcp): support custom OAuth callbackRedirectURL for remote toolsets
- [#2496](https://github.com/docker/docker-agent/pull/2496) - fix(tui): make /speak slash command dispatch immediately
- [#2497](https://github.com/docker/docker-agent/pull/2497) - tui: add custom component for user_prompt tool calls


## [v1.49.2] - 2026-04-21

This release fixes an issue with the --pull-interval flag when using URL gordon references.

## Bug Fixes
- Fixes blocking of --pull-interval flag when using URL gordon reference

## Technical Changes
- Updates CHANGELOG.md for v1.49.1

### Pull Requests

- [#2484](https://github.com/docker/docker-agent/pull/2484) - docs: update CHANGELOG.md for v1.49.1
- [#2485](https://github.com/docker/docker-agent/pull/2485) - Do not block --pull-interval flag when using URL gordon ref


## [v1.49.1] - 2026-04-21

This release improves the shell tool's command handling and fixes documentation inconsistencies.

## Improvements
- Accepts "command" as an alias for "cmd" in shell tool calls to improve compatibility with different AI models
- Improves error messaging when shell commands are empty or blank

## Bug Fixes
- Fixes documentation and code divergences reported in issue #2464 with 36 targeted corrections
- Prevents blank "cmd" parameters from interfering with "command" alias functionality

## Technical Changes
- Updates configuration schema version to 8 in documentation
- Updates CHANGELOG.md for v1.49.0 release

### Pull Requests

- [#2464](https://github.com/docker/docker-agent/pull/2464) - docs: fix doc-code divergences reported in issue #2464
- [#2479](https://github.com/docker/docker-agent/pull/2479) - docs: fix doc-code divergences reported in #2464
- [#2481](https://github.com/docker/docker-agent/pull/2481) - shell: accept `command` as alias for `cmd` and improve empty-arg error
- [#2483](https://github.com/docker/docker-agent/pull/2483) - docs: update CHANGELOG.md for v1.49.0


## [v1.49.0] - 2026-04-21

This release improves DMR support, adds skill filtering capabilities, and includes several bug fixes for OpenTelemetry and security hardening.

## What's New
- Adds support for filtering skills by name in agent YAML configuration
- Improves DMR support with better context size handling and structured configuration

## Bug Fixes
- Fixes OpenTelemetry service resource schema alignment
- Fixes path traversal vulnerability and other security issues in artifact store, skills loader, hooks, shell and agent warnings
- Fixes OpenTelemetry import ordering in tests

## Technical Changes
- Encodes agent source URL when using it as agent name and key for proper conversation handling in `serve api`
- Moves localhost helper comment in OpenTelemetry code

### Pull Requests

- [#2351](https://github.com/docker/docker-agent/pull/2351) - Improve DMR support
- [#2404](https://github.com/docker/docker-agent/pull/2404) - Merge pull request #2474 from dgageot/board/support-boolean-or-array-skills-in-yaml-f97b09f6
- [#2442](https://github.com/docker/docker-agent/pull/2442) - fix(otel): align service resource schema
- [#2470](https://github.com/docker/docker-agent/pull/2470) - docs: update CHANGELOG.md for v1.48.0
- [#2472](https://github.com/docker/docker-agent/pull/2472) - bump github.com/docker/cli from v29.4.0+incompatible to v29.4.1+incompatible
- [#2473](https://github.com/docker/docker-agent/pull/2473) - Encode agent source URL when using it as agent name and key, so that it can be used properly in conversations when using `serve api`
- [#2474](https://github.com/docker/docker-agent/pull/2474) - Support filtering skills by name in agent YAML (#2404)
- [#2480](https://github.com/docker/docker-agent/pull/2480) - fix: harden artifact store, skills loader, hooks, shell and agent warnings


## [v1.48.0] - 2026-04-20

This release adds working directory configuration for MCP and LSP toolsets and improves toolset reliability with better retry handling.

## What's New
- Adds optional `working_dir` field to MCP and LSP toolset configurations to launch processes from a specific directory

## Bug Fixes
- Fixes retry behavior for MCP toolsets after tool calls within the same turn
- Stops retrying SQLITE_CANTOPEN (14) errors that cannot be resolved
- Fixes filepath handling to satisfy gocritic filepathJoin lint rule
- Returns explicit error when ref-based MCP resolves to remote server with working_dir

## Technical Changes
- Documents working_dir field for MCP and LSP toolsets in configuration

### Pull Requests

- [#2457](https://github.com/docker/docker-agent/pull/2457) - fix(#2457): retry MCP toolsets after tool calls within the same turn
- [#2458](https://github.com/docker/docker-agent/pull/2458) - fix: retry LSP/MCP toolsets after tool calls, covering env-wrapped commands (fixes #2457)
- [#2460](https://github.com/docker/docker-agent/pull/2460) - feat: add optional working_dir to MCP and LSP toolset configs
- [#2466](https://github.com/docker/docker-agent/pull/2466) - Don't retry SQLITE_CANTOPEN (14) errors
- [#2468](https://github.com/docker/docker-agent/pull/2468) - docs: update CHANGELOG.md for v1.47.0


## [v1.47.0] - 2026-04-20

This release fixes several issues with AI model interactions, including title generation failures with reasoning models and shell command hangs.

## Bug Fixes
- Fixes title generation failures with OpenAI reasoning models by using low reasoning effort instead of omitting it
- Fixes shell command hangs when a tool command backgrounds a child process
- Repairs malformed JSON in edit_file tool call arguments that was causing parsing failures
- Moves reasoning token budget floor to OpenAI provider for better token management

## Improvements
- Increases title generation token budget for reasoning models to ensure adequate output space
- Adds thinking_display provider option for Anthropic models to control visibility of thinking blocks

## Technical Changes
- Adds test assertion for non-empty title in end-to-end title generation tests

### Pull Requests

- [#2412](https://github.com/docker/docker-agent/pull/2412) - fix: title generation fails with OpenAI reasoning models
- [#2451](https://github.com/docker/docker-agent/pull/2451) - Add thinking_display provider_opt for Anthropic models
- [#2452](https://github.com/docker/docker-agent/pull/2452) - fix: repair malformed JSON in edit_file tool call arguments
- [#2455](https://github.com/docker/docker-agent/pull/2455) - docs: update CHANGELOG.md for v1.46.0
- [#2462](https://github.com/docker/docker-agent/pull/2462) - shell: fix hang when a tool command backgrounds a child process
- [#2463](https://github.com/docker/docker-agent/pull/2463) - bump direct Go dependencies


## [v1.46.0] - 2026-04-16

This release adds OAuth credential configuration for MCP servers, evaluation testing improvements, and numerous stability fixes.

## What's New
- Adds support for explicit OAuth credentials configuration for remote MCP servers that don't support Dynamic Client Registration
- Adds `--repeat` flag to eval command for running evaluations multiple times
- Adds support for `xhigh` effort level in Anthropic adaptive thinking (Claude Opus 4.7+)
- Adds `task_budget` configuration field for Claude Opus 4.7 to cap total tokens across multi-step tasks
- Adds markdown rendering support in user_prompt dialog messages

## Improvements
- Improves image attachment handling by inlining as base64 data URLs for cross-provider compatibility
- Improves robots.txt caching to store parsed data per host instead of boolean results
- Improves session database version detection with clear upgrade messages for newer databases

## Bug Fixes
- Fixes `--attach` flag being silently ignored when used without a message argument
- Fixes data race in AddMessageUsageRecord by adding mutex lock
- Fixes data race in rule-based router by protecting lastSelectedID with mutex
- Fixes panic in extractSystemBlocks when system message is empty with CacheControl
- Fixes empty messages slice handling in SendUserMessage path
- Fixes symlink-based path traversal vulnerability in ACP filesystem toolset
- Fixes OAuth callback CSRF vulnerability by rejecting when expected state is not set
- Fixes MCP tryRestart to use context-aware select instead of time.Sleep
- Fixes assistant text being discarded when tool calls are present in Responses API conversion
- Fixes MCP OAuth token refresh by remembering the discovered auth server

## Technical Changes
- Updates mutex handling for MCP Toolset.Instructions() method
- Updates Go dependencies including Anthropic SDK and various UI libraries

### Pull Requests

- [#2394](https://github.com/docker/docker-agent/pull/2394) - Support explicit OAuth credentials for remote MCP servers
- [#2427](https://github.com/docker/docker-agent/pull/2427) - docs: update CHANGELOG.md for v1.45.0
- [#2428](https://github.com/docker/docker-agent/pull/2428) - fix: add mutex lock to AddMessageUsageRecord to prevent data race
- [#2429](https://github.com/docker/docker-agent/pull/2429) - fix: add mutex to protect lastSelectedID in rule-based router
- [#2430](https://github.com/docker/docker-agent/pull/2430) - fix: hold mutex for instructions read in MCP Toolset.Instructions()
- [#2431](https://github.com/docker/docker-agent/pull/2431) - fix: prevent panic in extractSystemBlocks on empty system message wit…
- [#2432](https://github.com/docker/docker-agent/pull/2432) - fix: guard against empty messages slice in SendUserMessage path
- [#2433](https://github.com/docker/docker-agent/pull/2433) - fix: prevent symlink-based path traversal in ACP filesystem toolset
- [#2434](https://github.com/docker/docker-agent/pull/2434) - fix: reject OAuth callback when expected state has not been set (CSRF)
- [#2436](https://github.com/docker/docker-agent/pull/2436) - fix: replace time.Sleep with context-aware select in MCP tryRestart
- [#2437](https://github.com/docker/docker-agent/pull/2437) - fix: cache parsed robots.txt per host instead of boolean result
- [#2438](https://github.com/docker/docker-agent/pull/2438) - fix: preserve assistant text when tool calls present in Responses API conversion
- [#2440](https://github.com/docker/docker-agent/pull/2440) - Add --repeat flag to eval command for running evaluations multiple times
- [#2441](https://github.com/docker/docker-agent/pull/2441) - fix: detect newer session database and show clear upgrade message
- [#2444](https://github.com/docker/docker-agent/pull/2444) - bump direct Go dependencies
- [#2445](https://github.com/docker/docker-agent/pull/2445) - Add a pokemon example
- [#2446](https://github.com/docker/docker-agent/pull/2446) - Render markdown in user_prompt dialog messages
- [#2447](https://github.com/docker/docker-agent/pull/2447) - Add an advanced coder example
- [#2448](https://github.com/docker/docker-agent/pull/2448) - fix(mcp): reuse discovered auth server for token refresh
- [#2449](https://github.com/docker/docker-agent/pull/2449) - Fix --attach flag
- [#2450](https://github.com/docker/docker-agent/pull/2450) - Support xhigh effort for Anthropic adaptive thinking (Opus 4.7+)
- [#2453](https://github.com/docker/docker-agent/pull/2453) - feat(anthropic): add task_budget for Claude Opus 4.7
- [#2454](https://github.com/docker/docker-agent/pull/2454) - chore: update cagent-action to v1.4.1


## [v1.45.0] - 2026-04-15

This release improves template expression handling, adds circular navigation to completions, and fixes issues with skills and MCP toolset loading.

## Bug Fixes
- Fixes evaluation of JavaScript template expressions to handle failures independently - when one expression fails, other valid expressions in the same template are still expanded
- Fixes skills loading functionality
- Fixes retry behavior for MCP toolset startup when server is unavailable
- Fixes MCP toolset creation to proceed even when command binary is unavailable

## Improvements
- Adds circular navigation wrapping to completion component, allowing users to cycle through completion options

### Pull Requests

- [#2400](https://github.com/docker/docker-agent/pull/2400) - fix: evaluate JS template expressions independently on failure
- [#2403](https://github.com/docker/docker-agent/pull/2403) - docs: update CHANGELOG.md for v1.44.0
- [#2407](https://github.com/docker/docker-agent/pull/2407) - add circular navigation wrapping to completion component
- [#2413](https://github.com/docker/docker-agent/pull/2413) - fix: retry stdio MCP toolset when binary is unavailable at startup
- [#2414](https://github.com/docker/docker-agent/pull/2414) - Fix skills loading


## [v1.44.0] - 2026-04-13

This release introduces TUI customization capabilities, session management improvements, and OAuth security enhancements, along with numerous bug fixes and stability improvements.

## What's New

- Adds support for extending and customizing TUI with additional commands through new `Immediate` flag and `Parser` struct
- Adds session delete functionality to session browser
- Adds click-to-select support for agents in the sidebar
- Adds `/fork` slash command to duplicate current session into a new tab
- Adds mid-turn message steering for running agent sessions with new `/steer` and `/followup` API endpoints
- Adds OAuth token storage in OS keychain with silent refresh token support
- Adds debug OAuth commands: list, remove, and login
- Adds support for shell expansions (~, env vars) in config paths
- Adds total session count display in session browser dialog title

## Improvements

- Improves TUI rendering to match sandbox template
- Makes Ctrl+W context-aware to preserve word deletion in editor when focused
- Makes `/exit` close only the current tab when multiple tabs are open

## Bug Fixes

- Fixes crash when opening empty websocket frames in OpenAI provider
- Fixes Gemini thinking tokens not included in output token count for cost calculation
- Fixes tool calls getting stuck as running when moved out of active reasoning block
- Fixes missing type in schema and orphaned function calls in Responses API
- Fixes spurious blank line appearing in every assistant message
- Fixes layout shift when hovering over assistant messages to reveal copy button
- Fixes concurrent RunSession calls causing tool_use/tool_result mismatch
- Fixes panic in code mode when tool handler is nil
- Fixes suggestion ghost text remaining when completion dialog closes on backspace
- Fixes skill frontmatter parsing when description contains a colon
- Fixes sidebar agent click zones mapping all lines to first agent
- Fixes OAuth token security vulnerabilities and infinite recursion issues
- Fixes auto-detect tool install failures being treated as fatal
- Fixes background agent context being cancelled with parent message lifecycle

## Technical Changes

- Stores OAuth tokens in OS keychain with graceful fallback to in-memory storage
- Serializes concurrent RunSession calls to prevent race conditions
- Sanitizes message history to ensure all tool calls have results
- Adds regression tests for SSE comment lines from OpenRouter
- Uses in-memory store in keyring tests to avoid macOS keychain permission dialog
- Separates steer and follow-up into distinct queues with lock/confirm semantics
- Adds documentation for OpenAPI toolset
- Optimizes PR CI build process

### Pull Requests

- [#2346](https://github.com/docker/docker-agent/pull/2346) - Allow to extend and customize TUI with additional commands
- [#2347](https://github.com/docker/docker-agent/pull/2347) - docs: update CHANGELOG.md for v1.43.0
- [#2348](https://github.com/docker/docker-agent/pull/2348) - Better sandbox
- [#2349](https://github.com/docker/docker-agent/pull/2349) - Add regression tests for SSE comment lines from OpenRouter (#2349)
- [#2350](https://github.com/docker/docker-agent/pull/2350) - fix(openai): ignore empty websocket frames
- [#2352](https://github.com/docker/docker-agent/pull/2352) - support session delete to session browser
- [#2355](https://github.com/docker/docker-agent/pull/2355) - Store OAuth tokens in OS keychain and add silent refresh token support
- [#2356](https://github.com/docker/docker-agent/pull/2356) - feat: click on agent in sidebar to switch to it
- [#2358](https://github.com/docker/docker-agent/pull/2358) - bump direct Go dependencies
- [#2359](https://github.com/docker/docker-agent/pull/2359) - Add regression tests for SSE comment lines from OpenRouter
- [#2360](https://github.com/docker/docker-agent/pull/2360) - Fix tool call stuck as running when moved out of active reasoning block
- [#2362](https://github.com/docker/docker-agent/pull/2362) - fix: handle missing type in schema and orphaned function calls in Responses API
- [#2363](https://github.com/docker/docker-agent/pull/2363) - Add mid-turn message steering for running agent sessions
- [#2365](https://github.com/docker/docker-agent/pull/2365) - Debug oauth
- [#2366](https://github.com/docker/docker-agent/pull/2366) - optional title and app name
- [#2367](https://github.com/docker/docker-agent/pull/2367) - fix: use in-memory store in keyring tests to avoid macOS keychain permission dialog
- [#2369](https://github.com/docker/docker-agent/pull/2369) - fix(tui): remove spurious blank line from every assistant message
- [#2371](https://github.com/docker/docker-agent/pull/2371) - docs: add documentation for OpenAPI toolset
- [#2374](https://github.com/docker/docker-agent/pull/2374) - fix(tui): reserve stable top row for copy icon to prevent layout shift
- [#2375](https://github.com/docker/docker-agent/pull/2375) - fix: serialize concurrent RunSession calls to prevent tool_use/tool_result mismatch
- [#2377](https://github.com/docker/docker-agent/pull/2377) - Sanitize message history
- [#2378](https://github.com/docker/docker-agent/pull/2378) - Faster PR CI
- [#2385](https://github.com/docker/docker-agent/pull/2385) - Add /fork slash command to duplicate current session into a new tab
- [#2386](https://github.com/docker/docker-agent/pull/2386) - fix(toolinstall): soft-fail auto-detect installs
- [#2387](https://github.com/docker/docker-agent/pull/2387) - fix: /exit closes only the current tab when multiple tabs are open
- [#2388](https://github.com/docker/docker-agent/pull/2388) - fix: prevent panic in code mode when tool handler is nil
- [#2389](https://github.com/docker/docker-agent/pull/2389) - Add support for shell expansions (~, env vars) in config paths
- [#2390](https://github.com/docker/docker-agent/pull/2390) - fix: make Ctrl+W context-aware to preserve word deletion in editor
- [#2391](https://github.com/docker/docker-agent/pull/2391) - Show total session count in session browser dialog title
- [#2392](https://github.com/docker/docker-agent/pull/2392) - fix: decouple background agent context from parent message lifecycle
- [#2395](https://github.com/docker/docker-agent/pull/2395) - fix: OAuth token security and bug fixes
- [#2398](https://github.com/docker/docker-agent/pull/2398) - Bump direct Go dependencies
- [#2399](https://github.com/docker/docker-agent/pull/2399) - fix: clear suggestion ghost text when completion dialog closes on backspace
- [#2401](https://github.com/docker/docker-agent/pull/2401) - Fix skill frontmatter parsing when description contains a colon
- [#2402](https://github.com/docker/docker-agent/pull/2402) - Fix sidebar agent click zones mapping all lines to first agent


## [v1.43.0] - 2026-04-08

This release adds non-interactive mode capabilities, improves TUI interactions with mouse support, and includes several bug fixes for RAG tools and streaming responses.

## What's New

- Adds auto-stop for max iterations in non-interactive mode to prevent hanging when tools are approved
- Adds non-interactive mode flag to distinguish from tools approval scenarios
- Adds mouse drag-to-move support for TUI dialogs, allowing repositioning by clicking and dragging the title area
- Adds custom session ID support through WithID option instead of relying on UUID generation
- Adds support for custom providers in RAG embedding and reranking models
- Adds underline styling for URLs on mouse hover

## Improvements

- Evolves providers config to support any provider type with shared model defaults
- Improves mise build output to show go build command and resulting binary
- Exempts background-agent polling from loop-termination detection to prevent false positives

## Bug Fixes

- Fixes agent accent color application for working spinners in sidebar
- Fixes duplicate RAG tool names and nil pointer panic in file watcher
- Fixes toolset startup triggering from emitToolsChanged callback to avoid spurious timeout warnings
- Fixes missing Models map in RAG ManagersBuildConfig for model alias resolution
- Fixes nil pointer dereference in BM25Strategy.watchLoop during session teardown
- Fixes extraction of reasoning_content from DMR streaming responses
- Fixes scrollbar rendering in web terminals by replacing problematic characters

## Technical Changes

- Adds nocgo build support for rag/treesitter
- Updates error message display when only one model is available

### Pull Requests

- [#2208](https://github.com/docker/docker-agent/pull/2208) - feat(runtime): add auto-stop for max iterations in non-interactive mode
- [#2315](https://github.com/docker/docker-agent/pull/2315) - fix: use agent accent color for working spinners in sidebar
- [#2316](https://github.com/docker/docker-agent/pull/2316) - Underline URLs on mouse hover
- [#2317](https://github.com/docker/docker-agent/pull/2317) - docs: update CHANGELOG.md for v1.42.0
- [#2319](https://github.com/docker/docker-agent/pull/2319) - Exempt background-agent polling from loop-termination detection
- [#2322](https://github.com/docker/docker-agent/pull/2322) - fix: resolve duplicate RAG tool names and nil pointer panic in file watcher
- [#2323](https://github.com/docker/docker-agent/pull/2323) - fix: avoid triggering toolset startup from emitToolsChanged callback
- [#2324](https://github.com/docker/docker-agent/pull/2324) - fix: pass Models map to RAG ManagersBuildConfig for model alias resolution
- [#2331](https://github.com/docker/docker-agent/pull/2331) - session: add WithID option for custom session IDs
- [#2334](https://github.com/docker/docker-agent/pull/2334) - Fix nil pointer dereference in BM25Strategy.watchLoop during session teardown
- [#2335](https://github.com/docker/docker-agent/pull/2335) - fix: extract reasoning_content from DMR streaming responses
- [#2338](https://github.com/docker/docker-agent/pull/2338) - Improve mise build output to show go build command and resulting binary
- [#2339](https://github.com/docker/docker-agent/pull/2339) - feat: add mouse drag-to-move support for TUI dialogs
- [#2340](https://github.com/docker/docker-agent/pull/2340) - Fix scrollbar rendering in web terminals
- [#2343](https://github.com/docker/docker-agent/pull/2343) - Evolve providers to support any provider type with shared model defaults
- [#2344](https://github.com/docker/docker-agent/pull/2344) - feat: support custom providers in RAG embedding and reranking models
- [#2345](https://github.com/docker/docker-agent/pull/2345) - Nicer message


## [v1.42.0] - 2026-04-03

This release improves evaluation output with structured JSON results and fixes several Windows compatibility issues.

## What's New
- Adds URL click detection for terminals with mouse tracking support
- Includes structured results, run configuration, and summary in evaluation JSON output
- Includes judge reasons for passed relevance criteria in evaluation results

## Bug Fixes
- Fixes Windows OS detection typo in session environment (corrects "window" to "windows")
- Replaces removed claude-3-7-sonnet-latest alias with explicit model ID in examples
- Uses platform-aware shell detection for Windows compatibility in skill expansion, script_shell, post-edit hooks, and bang commands

## Technical Changes
- Pre-populates criterion names in CheckRelevance results
- Fixes lint issues including gci formatting and testifylint float comparisons

### Pull Requests

- [#2307](https://github.com/docker/docker-agent/pull/2307) - docs: update CHANGELOG.md for v1.41.0
- [#2308](https://github.com/docker/docker-agent/pull/2308) - tui/messages: Add URL click detection for terminals with mouse tracking
- [#2309](https://github.com/docker/docker-agent/pull/2309) - eval: include structured results, run config, and summary in JSON output
- [#2312](https://github.com/docker/docker-agent/pull/2312) - fix: correct Windows OS detection typo in session environment
- [#2313](https://github.com/docker/docker-agent/pull/2313) - fix: replace removed claude-3-7-sonnet-latest alias in examples
- [#2314](https://github.com/docker/docker-agent/pull/2314) - fix: use platform-aware shell for skill expansion, script_shell, post-edit hooks, and bang command


## [v1.41.0] - 2026-04-01

This release introduces a new models discovery command, contextual help system, and several TUI improvements including persistent warnings and simplified lean mode.

## What's New
- Adds `docker agent models` command to list available models for the `--model` flag
- Adds contextual help dialog accessible via Ctrl+H (or F1/Ctrl+?) showing all keyboard shortcuts
- Adds `--lean` flag for simplified TUI mode with minimal interface (just message stream and editor)
- Adds copy button on hover for assistant messages to copy content to clipboard
- Adds Vertex AI Model Garden support for non-Gemini models (Claude, Llama) hosted on Google Cloud

## Improvements
- Makes TUI warnings persist until manually dismissed instead of auto-dismissing after 3 seconds
- Preserves recent messages during session compaction to maintain conversational context
- Shows elapsed time and warning for long-running tool calls in the TUI
- Adds desktop_uuid in telemetry alongside user_uuid for better tracking

## Bug Fixes
- Fixes markdown rendering in callout notes by adding markdown="1" attribute
- Fixes panic on closed channel by making chanSend non-blocking
- Fixes recursive run_skill loop in context:fork skill sub-sessions
- Fixes docker run --sandbox functionality
- Fixes eval tool_call_response to use correct event field names
- Fixes guard against nil tool_definition in buildTranscript

## Technical Changes
- Replaces kin-openapi with pb33f/libopenapi for OpenAPI parsing
- Removes trailing headers handling for rate limit headers
- Tracks command errors with success=false and error details in telemetry
- Ports build system to mise
- Updates Go module dependencies

### Pull Requests

- [#2252](https://github.com/docker/docker-agent/pull/2252) - Make TUI warnings persist until manually dismissed
- [#2253](https://github.com/docker/docker-agent/pull/2253) - Add --lean flag for simplified TUI mode
- [#2259](https://github.com/docker/docker-agent/pull/2259) - Preserve recent messages during session compaction
- [#2279](https://github.com/docker/docker-agent/pull/2279) - Add desktop_uuid in telemetry (next to user_uuid)
- [#2281](https://github.com/docker/docker-agent/pull/2281) - docs: update CHANGELOG.md for v1.40.0
- [#2283](https://github.com/docker/docker-agent/pull/2283) - Track command errors with success=false and error details
- [#2284](https://github.com/docker/docker-agent/pull/2284) - Bump direct Go module dependencies
- [#2285](https://github.com/docker/docker-agent/pull/2285) - Fix markdown rendering in documentation callout notes
- [#2286](https://github.com/docker/docker-agent/pull/2286) - fix: make chanSend non-blocking to prevent panic on closed channel
- [#2287](https://github.com/docker/docker-agent/pull/2287) - Add Vertex AI Model Garden support for non-Gemini models
- [#2288](https://github.com/docker/docker-agent/pull/2288) - Add copy button on hover for assistant messages
- [#2289](https://github.com/docker/docker-agent/pull/2289) - fix: prevent recursive run_skill loop in context:fork skill sub-sessions
- [#2290](https://github.com/docker/docker-agent/pull/2290) - docs: add Vertex AI Model Garden section to Google provider docs
- [#2291](https://github.com/docker/docker-agent/pull/2291) - tui: show elapsed time and warning for long-running tool calls
- [#2292](https://github.com/docker/docker-agent/pull/2292) - go mod tidy
- [#2293](https://github.com/docker/docker-agent/pull/2293) - Port to mise
- [#2294](https://github.com/docker/docker-agent/pull/2294) - Fix TUI stuck in Working state after failed sub-agent transfer_task
- [#2298](https://github.com/docker/docker-agent/pull/2298) - Remove trailing headers handling for rate limit headers
- [#2299](https://github.com/docker/docker-agent/pull/2299) - Replace kin-openapi with pb33f/libopenapi for OpenAPI parsing
- [#2301](https://github.com/docker/docker-agent/pull/2301) - Fix `docker run --sandbox`
- [#2302](https://github.com/docker/docker-agent/pull/2302) - fix: eval tool_call_response uses correct event field names
- [#2304](https://github.com/docker/docker-agent/pull/2304) - feat: add `docker agent models` command
- [#2305](https://github.com/docker/docker-agent/pull/2305) - Add contextual help dialog (Ctrl+H)
- [#2306](https://github.com/docker/docker-agent/pull/2306) - use DD proxy when available, also from WSL


## [v1.40.0] - 2026-03-30

This release improves AI assistant capabilities with better response tracking and Google integration, plus fixes a critical exit hang issue.

## What's New
- Adds Google Search, Google Maps, and code execution capabilities for Gemini models
- Surfaces finish_reason information on assistant messages and token usage events to track why the AI stopped generating responses

## Bug Fixes
- Fixes process hang when using `/exit` command due to bubbletea renderer deadlock

## Technical Changes
- Adds tests reproducing bubbletea renderer deadlock on exit
- Adds safety-net exit mechanism for bubbletea renderer deadlock prevention

### Pull Requests

- [#2254](https://github.com/docker/docker-agent/pull/2254) - Surface finish_reason on assistant messages and token usage events
- [#2265](https://github.com/docker/docker-agent/pull/2265) - docs: update CHANGELOG.md for v1.39.0
- [#2269](https://github.com/docker/docker-agent/pull/2269) - Fix process hang on /exit due to bubbletea renderer deadlock
- [#2276](https://github.com/docker/docker-agent/pull/2276) - Google grounding
- [#2277](https://github.com/docker/docker-agent/pull/2277) - Fix url


## [v1.39.0] - 2026-03-27

This release adds new color themes for the terminal interface and includes internal version management updates.

## What's New
- Adds Calm Roots theme with warm white accents, sage green info messages, and charcoal background
- Adds Neon Pink theme with vibrant pink tones and high-contrast white accents for readability

## Technical Changes
- Freezes v7 version
- Updates CHANGELOG.md for v1.38.0

### Pull Requests

- [#2256](https://github.com/docker/docker-agent/pull/2256) - docs: update CHANGELOG.md for v1.38.0
- [#2260](https://github.com/docker/docker-agent/pull/2260) - Add Calm Roots and Neon Pink themes
- [#2264](https://github.com/docker/docker-agent/pull/2264) - Freeze v7


## [v1.38.0] - 2026-03-26

This release improves OAuth configuration and fixes tool caching issues with remote MCP server reconnections.

## Improvements

- Changes OAuth client name to "docker-agent" for better identification
- Reworks compaction logic to prevent infinite loops when context overflow errors occur repeatedly

## Bug Fixes

- Fixes tool cache not refreshing after remote MCP server reconnects, ensuring updated tools are available after server restarts

## Technical Changes

- Updates CHANGELOG.md for v1.37.0 release documentation

### Pull Requests

- [#2242](https://github.com/docker/docker-agent/pull/2242) - Refactor compaction
- [#2243](https://github.com/docker/docker-agent/pull/2243) - docs: update CHANGELOG.md for v1.37.0
- [#2245](https://github.com/docker/docker-agent/pull/2245) - Change the oauth client name to docker-agent
- [#2246](https://github.com/docker/docker-agent/pull/2246) - fix: refresh tool and prompt caches after remote MCP server reconnect


## [v1.37.0] - 2026-03-25

This release adds support for forwarding sampling parameters to provider APIs, introduces global user-level permissions, and includes several bug fixes and improvements.

## What's New

- Adds support for forwarding sampling provider options (top_k, repetition_penalty, etc.) to provider APIs
- Adds global-level permissions from user config that apply across all sessions and agents
- Adds a welcome message to the interface
- Adds custom linter to enforce config version import chain

## Improvements

- Refactors RAG from agent-level config to standard toolset type for consistency with other toolsets
- Restores RAG indexing event forwarding to TUI after toolset refactor
- Simplifies RAG event forwarding and cleans up RAGTool

## Bug Fixes

- Fixes Bedrock interleaved_thinking defaults to true and adds logging for provider_opts mismatches
- Fixes issue where CacheControl markers were preserved during message compaction, exceeding Anthropic's limit
- Fixes tool loop detector by resetting it after degenerate loop error
- Fixes desktop proxy socket name on WSL where http-proxy socket is not allowed for users

## Technical Changes

- Documents max_old_tool_call_tokens and max_consecutive_tool_calls in agent config reference
- Documents global permissions from user config in permissions reference and guides
- Pins GitHub actions for improved security
- Updates cagent-action to latest version with better permissions

### Pull Requests

- [#2210](https://github.com/docker/docker-agent/pull/2210) - Refactor RAG from agent-level config to standard toolset type
- [#2225](https://github.com/docker/docker-agent/pull/2225) - Add custom linter to enforce config version import chain
- [#2226](https://github.com/docker/docker-agent/pull/2226) - feat: forward sampling provider_opts (top_k, repetition_penalty) to provider APIs
- [#2227](https://github.com/docker/docker-agent/pull/2227) - docs: update CHANGELOG.md for v1.36.1
- [#2229](https://github.com/docker/docker-agent/pull/2229) - docs: add max_old_tool_call_tokens and max_consecutive_tool_calls to agent config reference
- [#2230](https://github.com/docker/docker-agent/pull/2230) - Add global-level permissions from user config
- [#2231](https://github.com/docker/docker-agent/pull/2231) - Pin GitHub actions
- [#2233](https://github.com/docker/docker-agent/pull/2233) - update cagent-action to latest (with better permissions)
- [#2236](https://github.com/docker/docker-agent/pull/2236) - fix: strip CacheControl from messages during compaction
- [#2237](https://github.com/docker/docker-agent/pull/2237) - Reset tool loop detector after degenerate loop error
- [#2238](https://github.com/docker/docker-agent/pull/2238) - Bump direct Go module dependencies
- [#2240](https://github.com/docker/docker-agent/pull/2240) - Fix desktop proxy socket name on WSL
- [#2241](https://github.com/docker/docker-agent/pull/2241) - docs: document global permissions from user config


## [v1.36.1] - 2026-03-23

This release improves OCI reference handling, adds a tools command, and enhances MCP server reliability with better error recovery.

## What's New
- Adds `/tools` command to show available tools in a TUI dialog
- Adds support for serving digest-pinned OCI references directly from cache

## Improvements
- Uses Docker Desktop proxy for all HTTP operations when Docker Desktop is running
- Improves MCP server reconnection by retrying tool calls on any connection error, not just session errors
- Normalizes OCI reference handling in store lookups to match Pull() key format

## Bug Fixes
- Fixes `/clear` command to properly re-initialize the TUI
- Fixes tools/permissions dialog height instability when scrolling
- Fixes empty lines in tools dialog from multiline descriptions
- Fixes relative path resolution when parentDir is empty by falling back to current working directory

## Technical Changes
- Extracts RAG code for better organization
- Removes model alias resolution for inline agent model references
- Sets missing category on MCP and script shell tools
- Removes dead code and unused agent event handling
- Enables additional linters (bodyclose, makezero, sqlclosecheck) with corresponding fixes
- Adds comprehensive Managing Secrets documentation guide

### Pull Requests

- [#2201](https://github.com/docker/docker-agent/pull/2201) - docs: update CHANGELOG.md for v1.36.0
- [#2204](https://github.com/docker/docker-agent/pull/2204) - Better oci refs
- [#2205](https://github.com/docker/docker-agent/pull/2205) - Simplify the runtime related RAG code a bit
- [#2206](https://github.com/docker/docker-agent/pull/2206) - Remove model alias resolution for inline agent model references
- [#2207](https://github.com/docker/docker-agent/pull/2207) - Fix /clear
- [#2209](https://github.com/docker/docker-agent/pull/2209) - Add /tools command to show the available tools
- [#2212](https://github.com/docker/docker-agent/pull/2212) - fix: recover from ErrSessionMissing when remote MCP server restarts
- [#2213](https://github.com/docker/docker-agent/pull/2213) - docs: clarify :agent and :name parameters in API server endpoints
- [#2215](https://github.com/docker/docker-agent/pull/2215) - fix: retry MCP callTool on any connection error, not just ErrSessionMissing
- [#2217](https://github.com/docker/docker-agent/pull/2217) - docs: add Managing Secrets guide
- [#2218](https://github.com/docker/docker-agent/pull/2218) - Bump Go dependencies
- [#2219](https://github.com/docker/docker-agent/pull/2219) - Enable bodyclose, makezero, and sqlclosecheck linters
- [#2221](https://github.com/docker/docker-agent/pull/2221) - fix: resolve relative paths against CWD when parentDir is empty
- [#2222](https://github.com/docker/docker-agent/pull/2222) - Use Docker Desktop proxy when available
- [#2224](https://github.com/docker/docker-agent/pull/2224) - Make run.go easier to read


## [v1.36.0] - 2026-03-20

This release adds WebSocket transport support for OpenAI streaming, introduces configurable tool call token limits, and improves the command-line interface with new session management capabilities.

## What's New

- Adds WebSocket transport option for OpenAI Responses API streaming as an alternative to SSE
- Adds `/clear` command to reset current tab with a new session
- Adds configurable `max_old_tool_call_tokens` setting in agent YAML to control historical tool call content retention

## Improvements

- Hides agent name header when stdout is not a TTY for cleaner piped output
- Sorts all slash commands by label and hides `/q` alias from dialogs, showing only `/exit` and `/quit`
- Injects `lastResponseID` as `previous_response_id` in WebSocket requests for better continuity

## Bug Fixes

- Fixes data race on WebSocket pool lazy initialization
- Fixes panic in WebSocket handling

## Technical Changes

- Removes legacy `syncMessagesColumn` and messages JSON column from database schema
- Simplifies WebSocket pool code structure
- Documents external OCI registry agents usage as sub-agents

### Pull Requests

- [#2186](https://github.com/docker/docker-agent/pull/2186) - Add WebSocket transport for OpenAI Responses API streaming
- [#2192](https://github.com/docker/docker-agent/pull/2192) - feat: make maxOldToolCallTokens configurable in agent YAML
- [#2195](https://github.com/docker/docker-agent/pull/2195) - docs: document external OCI registry agents as sub-agents
- [#2196](https://github.com/docker/docker-agent/pull/2196) - Remove syncMessagesColumn and legacy messages JSON column
- [#2197](https://github.com/docker/docker-agent/pull/2197) - Support `echo "hello" | docker agent | cat`
- [#2199](https://github.com/docker/docker-agent/pull/2199) - Add /clear command to reset current tab with a new session
- [#2200](https://github.com/docker/docker-agent/pull/2200) - Hide /q from dialogs and sort all commands by label


## [v1.34.0] - 2026-03-19

This release improves tool call handling and evaluation functionality with several technical fixes and optimizations.

## Improvements

- Optimizes partial tool call streaming by sending only delta arguments instead of accumulated arguments
- Reduces evaluation summary display width for better terminal formatting
- Includes tool definition only on the first partial tool call to reduce redundancy

## Bug Fixes

- Fixes schema conversion for OpenAI Responses API strict mode, resolving issues with gpt-4.1-nano
- Removes duplicate tool call data from tool call response events to reduce payload size

## Technical Changes

- Updates evaluation system to not provide all API keys when using models gateway
- Removes redundant tool call information from response events while preserving tool call IDs for client reference

### Pull Requests

- [#2105](https://github.com/docker/docker-agent/pull/2105) - Only send the delta on the partial tool call
- [#2159](https://github.com/docker/docker-agent/pull/2159) - docs: update CHANGELOG.md for v1.33.0
- [#2160](https://github.com/docker/docker-agent/pull/2160) - Fix (reduce) evals summary width
- [#2162](https://github.com/docker/docker-agent/pull/2162) - Evals: don't provide all API keys when using models gateway
- [#2163](https://github.com/docker/docker-agent/pull/2163) - Remove the tool call from the tool call response event
- [#2164](https://github.com/docker/docker-agent/pull/2164) - build(deps): bump google.golang.org/grpc from 1.79.2 to 1.79.3 in the go_modules group across 1 directory
- [#2168](https://github.com/docker/docker-agent/pull/2168) - Fix schema conversion for OpenAI Responses API strict mode - Fixes tool calls with gpt-4.1-nano


## [v1.33.0] - 2026-03-18

This release improves file editing reliability, adds session exit keywords, and fixes several issues with sub-sessions and evaluation handling.

## What's New
- Adds support for "exit", "quit", and ":q" keywords to quit sessions immediately
- Adds per-eval Docker image override via evals.image property in evaluation configurations
- Adds run instructions to creator agent prompt for proper agent execution guidance

## Bug Fixes
- Fixes handling of double-serialized edits argument in edit_file tool when LLMs send JSON strings instead of arrays
- Fixes sub-session thinking state being incorrectly derived from parent session instead of child agent
- Fixes --sandbox flag when running in CLI plugin mode
- Fixes cross-model Gemini function calls by using dummy thought_signature
- Fixes event timestamps for user messages in SessionFromEvents to prevent duration calculation issues

## Improvements
- Displays breakdown of failure types in evaluation summary for better debugging
- Declines elicitations in run --exec --json mode
- Validates path field consistently in edit file operations

## Technical Changes
- Removes unused fileWriteTracker from creator package
- Simplifies UnmarshalJSON implementation for better path validation
- Updates evaluation image build cache to handle different images per working directory

### Pull Requests

- [#2144](https://github.com/docker/docker-agent/pull/2144) - fix: handle double-serialized edits argument in edit_file tool
- [#2146](https://github.com/docker/docker-agent/pull/2146) - Better rendering in tmux and ghostty
- [#2147](https://github.com/docker/docker-agent/pull/2147) - docs: update CHANGELOG.md for v1.32.5
- [#2149](https://github.com/docker/docker-agent/pull/2149) - fix: sub-session thinking state derived from child agent, not parent session
- [#2150](https://github.com/docker/docker-agent/pull/2150) - Display breakdown of types of failures in eval summary
- [#2151](https://github.com/docker/docker-agent/pull/2151) - Fix --sandbox when running cli plugin mode
- [#2152](https://github.com/docker/docker-agent/pull/2152) - feat: support "exit" as a keyword to quit the session
- [#2153](https://github.com/docker/docker-agent/pull/2153) - Add per-eval Docker image override via evals.image property
- [#2154](https://github.com/docker/docker-agent/pull/2154) - Add run instructions to creator agent prompt
- [#2155](https://github.com/docker/docker-agent/pull/2155) - fix: use dummy thought_signature for cross-model Gemini function calls
- [#2156](https://github.com/docker/docker-agent/pull/2156) - Decline elicitations in run --exec --json mode
- [#2157](https://github.com/docker/docker-agent/pull/2157) - Remove unused fileWriteTracker from creator package
- [#2158](https://github.com/docker/docker-agent/pull/2158) - fix: use event timestamps for user messages in SessionFromEvents


## [v1.32.5] - 2026-03-17

This release improves agent reliability and performance with better tool loop detection, enhanced MCP handling, and various bug fixes.

## What's New

- Adds framework-level tool loop detection to prevent degenerate agent loops when the same tool is called repeatedly
- Adds support for dynamic command expansion in skills using `!\`command\`` syntax
- Adds support for running skills as isolated sub-agents via `context: fork` frontmatter
- Adds CLI flags (`--hook-pre-tool-use`, `--hook-post-tool-use`, etc.) to override agent hooks from command line
- Adds stop and notification hooks with session lifecycle integration

## Improvements

- Reworks thinking budget system to be opt-in by default with adaptive thinking and effort levels
- Caches syntax highlighting results for code blocks to improve markdown rendering performance
- Optimizes MCP catalog loading with single fetch per run and ETag caching
- Derives meaningful names for external sub-agents instead of using generic 'root' name
- Optimizes filesystem tool performance by avoiding duplicate string allocations
- Speeds up history loading with ReadFile and strconv.Unquote optimizations

## Bug Fixes

- Fixes context cancelling during RAG initialization and query operations
- Fixes frozen spinner during MCP tool loading
- Fixes model name display in TUI sidebar for all model types
- Fixes two data races in shell tool execution
- Fixes character handling issues in tmux integration
- Fixes binary download URLs in documentation to match release artifact naming
- Validates thinking_budget effort levels at parse time and rejects unknown values

## Technical Changes

- Removes unused methods from codebase
- Hardens and simplifies MCP gateway code
- Adds logging for selected model in Agent.Model() for better observability
- Fixes pool_size reporting to reflect actual selection pool
- Reverts timeout changes for remote MCP initialization and tool calls

### Pull Requests

- [#2112](https://github.com/docker/docker-agent/pull/2112) - docs: update CHANGELOG.md for v1.32.4
- [#2113](https://github.com/docker/docker-agent/pull/2113) - Bump dependencies
- [#2114](https://github.com/docker/docker-agent/pull/2114) - Fix rag init context cancel
- [#2115](https://github.com/docker/docker-agent/pull/2115) - Fix frozen spinner during MCP tool loading
- [#2116](https://github.com/docker/docker-agent/pull/2116) - Support dynamic command expansion in skills (\!`command` syntax)
- [#2118](https://github.com/docker/docker-agent/pull/2118) - Fix model name display in TUI sidebar for all model types
- [#2119](https://github.com/docker/docker-agent/pull/2119) - perf(markdown): cache syntax highlighting results for code blocks
- [#2121](https://github.com/docker/docker-agent/pull/2121) - Rework thinking budget: opt-in by default, adaptive thinking, effort levels
- [#2123](https://github.com/docker/docker-agent/pull/2123) - feat: framework-level tool loop detection
- [#2124](https://github.com/docker/docker-agent/pull/2124) - Simplify MCP catalog loading: single fetch per run with ETag caching
- [#2125](https://github.com/docker/docker-agent/pull/2125) - Fix issues on builtin filesystem tools
- [#2127](https://github.com/docker/docker-agent/pull/2127) - Fix two data races in shell tool
- [#2128](https://github.com/docker/docker-agent/pull/2128) - Fix a few characters for tmux
- [#2129](https://github.com/docker/docker-agent/pull/2129) - docs: fix binary download URLs to match release artifact naming
- [#2130](https://github.com/docker/docker-agent/pull/2130) - More doc fixing with "agent serve mcp"
- [#2131](https://github.com/docker/docker-agent/pull/2131) - Add timeouts to remote MCP initialization and tool calls
- [#2132](https://github.com/docker/docker-agent/pull/2132) - Derive meaningful names for external sub-agents instead of using 'root'
- [#2133](https://github.com/docker/docker-agent/pull/2133) - gateway: harden and simplify MCP gateway code
- [#2134](https://github.com/docker/docker-agent/pull/2134) - Log selected model in Agent.Model() for alloy observability
- [#2135](https://github.com/docker/docker-agent/pull/2135) - Add --hook-* CLI flags to override agent hooks from the command line
- [#2136](https://github.com/docker/docker-agent/pull/2136) - Add stop and notification hooks, wire up session lifecycle hooks
- [#2137](https://github.com/docker/docker-agent/pull/2137) - feat: support running skills as isolated sub-agents via context: fork
- [#2138](https://github.com/docker/docker-agent/pull/2138) - Optimize start time
- [#2141](https://github.com/docker/docker-agent/pull/2141) - Revert "Add timeouts to remote MCP initialization and tool calls"
- [#2142](https://github.com/docker/docker-agent/pull/2142) - Reject unknown thinking_budget effort levels at parse time


## [v1.32.4] - 2026-03-16

This release optimizes tool instructions, removes unused session metadata, and includes several bug fixes and improvements.

## Improvements

- Optimizes builtin tool instructions for conciseness by applying Claude 4 prompt engineering best practices
- Removes unused branch metadata and split_diff_view from sessions to clean up data storage

## Bug Fixes

- Fixes emoji rendering issues in iTerm2
- Reverts keyboard enhancement changes that caused incorrect behavior in VSCode with AZERTY layout

## Technical Changes

- Extracts compaction logic into dedicated pkg/compaction package for better code organization
- Updates skill configuration
- Improves evaluation system by validating LLM judge, disabling thinking for LLM as judge, and removing handoffs scoring
- Disallows unknown fields in configuration validation

### Pull Requests

- [#2078](https://github.com/docker/docker-agent/pull/2078) - Remove unused branch metadata and split_diff_view from sessions
- [#2091](https://github.com/docker/docker-agent/pull/2091) - Optimize builtin tool instructions for conciseness
- [#2094](https://github.com/docker/docker-agent/pull/2094) - Bump dependencies
- [#2097](https://github.com/docker/docker-agent/pull/2097) - docs: update CHANGELOG.md for v1.32.3
- [#2098](https://github.com/docker/docker-agent/pull/2098) - Revert "tui: improve tmux experience and simplify keyboard enhancements"
- [#2099](https://github.com/docker/docker-agent/pull/2099) - Fix 2089 - emoji rendering in iTerm2
- [#2100](https://github.com/docker/docker-agent/pull/2100) - Improve evals
- [#2101](https://github.com/docker/docker-agent/pull/2101) - Extract compaction into a dedicated pkg/compaction package


## [v1.32.3] - 2026-03-13

This release removes an experimental feature and improves error handling for rate-limited API requests.

## Improvements
- Makes HTTP 429 (Too Many Requests) errors retryable when no fallback model is available, respecting the Retry-After header

## Bug Fixes
- Gates 429 retry behavior behind WithRetryOnRateLimit() opt-in option to prevent unexpected retry behavior

## Technical Changes
- Removes experimental feature from the codebase
- Adds optional gateway usage for LLM evaluation as a judge
- Refactors to use typed StatusError for retry metadata, with providers wrapping errors at Recv()

### Pull Requests

- [#2087](https://github.com/docker/docker-agent/pull/2087) - Remove experimental feature
- [#2090](https://github.com/docker/docker-agent/pull/2090) - docs: update CHANGELOG.md for v1.32.2
- [#2092](https://github.com/docker/docker-agent/pull/2092) - [eval] Optionnally use the gateway for the llm as a judge
- [#2093](https://github.com/docker/docker-agent/pull/2093) - This can be retried
- [#2096](https://github.com/docker/docker-agent/pull/2096) - fix: make HTTP 429 retryable when no fallback model, respect Retry-After header


## [v1.32.2] - 2026-03-12

This release focuses on security improvements and bug fixes, including prevention of PATH hijacking vulnerabilities and fixes to environment file support.

## Bug Fixes
- Fixes prevention of PATH hijacking and TOCTOU (Time-of-Check-Time-of-Use) vulnerabilities in shell/binary resolution (CWE-426)
- Fixes --env-file support for the gateway

## Technical Changes
- Removes debug code from codebase
- Reverts user prompt options feature that was previously added

### Pull Requests

- [#2071](https://github.com/docker/docker-agent/pull/2071) - Add options-based selection to user_prompt tool
- [#2083](https://github.com/docker/docker-agent/pull/2083) - fix: prevent PATH hijacking and TOCTOU in shell/binary resolution
- [#2084](https://github.com/docker/docker-agent/pull/2084) - docs: update CHANGELOG.md for v1.32.1
- [#2085](https://github.com/docker/docker-agent/pull/2085) - Fix --env-file support for the gateway
- [#2086](https://github.com/docker/docker-agent/pull/2086) - Remove debug code
- [#2088](https://github.com/docker/docker-agent/pull/2088) - Revert "Add options-based selection to user_prompt tool"


## [v1.32.1] - 2026-03-12

This release fixes several issues with session handling, tool elicitation, and MCP environment variable validation.

## Bug Fixes
- Fixes corrupted session history by filtering sub-agent streaming events from parent session persistence
- Fixes elicitation requests failing in sessions with ToolsApproved=true by decoupling elicitation channel from ToolsApproved flag
- Fixes MCP environment variable validation being skipped when any gateway preflight errors occur

## Improvements
- Prevents sidebar from scrolling to top when clicking navigation links in documentation

## Technical Changes
- Adds end-to-end test for tool result block validation
- Updates CHANGELOG.md for v1.32.0 release

### Pull Requests

- [#2053](https://github.com/docker/docker-agent/pull/2053) - fix(#2053): filter sub-agent streaming events from parent session persistence
- [#2072](https://github.com/docker/docker-agent/pull/2072) - docs: update CHANGELOG.md for v1.32.0
- [#2076](https://github.com/docker/docker-agent/pull/2076) - Don't scroll sidebar to the top
- [#2077](https://github.com/docker/docker-agent/pull/2077) - Fix corrupted session history
- [#2080](https://github.com/docker/docker-agent/pull/2080) - fix: decouple elicitation channel from ToolsApproved flag
- [#2081](https://github.com/docker/docker-agent/pull/2081) - Fix MCP env var check skipped when any gateway preflight errors


## [v1.32.0] - 2026-03-12

This release adds support for newer Gemini models, improves toolset documentation, and enhances user interaction capabilities.

## What's New

- Adds options-based selection to user_prompt tool, allowing the agent to present users with labeled choices instead of free-form input
- Documents {ORIGINAL_INSTRUCTIONS} placeholder for enriching toolset instructions rather than replacing them

## Bug Fixes

- Fixes support for Gemini 3.x versioned models (e.g., gemini-3.1-pro-preview) to ensure proper model recognition and thinking configuration
- Fixes gateway handling when using docker agent without a command
- Fixes broken links in documentation

## Technical Changes

- Adds check for broken links in CI
- Updates .gitignore to exclude cagent-* binaries from being committed

### Pull Requests

- [#2054](https://github.com/docker/docker-agent/pull/2054) - fix: support Gemini 3.x versioned models (e.g., gemini-3.1-pro-preview)
- [#2062](https://github.com/docker/docker-agent/pull/2062) - doc: document {ORIGINAL_INSTRUCTIONS} placeholder for toolset instructions
- [#2063](https://github.com/docker/docker-agent/pull/2063) - docs: update CHANGELOG.md for v1.31.0
- [#2064](https://github.com/docker/docker-agent/pull/2064) - Fix gateway handling with docker agent without command
- [#2067](https://github.com/docker/docker-agent/pull/2067) - Fix broken links
- [#2068](https://github.com/docker/docker-agent/pull/2068) - Check for broken links
- [#2069](https://github.com/docker/docker-agent/pull/2069) - gitignore cagent-* binaries
- [#2071](https://github.com/docker/docker-agent/pull/2071) - Add options-based selection to user_prompt tool


## [v1.31.0] - 2026-03-11

This release enhances the cost dialog with detailed session statistics and improves todo tool reliability for better task completion tracking.

## What's New
- Adds total token count, session duration, and message count to cost dialog
- Adds reasoning tokens display for supported models (e.g. o1)
- Adds average cost per 1K tokens and per message metrics to cost analysis
- Adds cost percentage breakdown per model and per message
- Adds cache hit rate and per-entry cached token count display

## Improvements
- Improves todo tool reliability by reminding LLM of incomplete items and including full state in all responses

## Bug Fixes
- Fixes Sonnet model name
- Fixes various edge-case bugs in cost dialog formatting

## Technical Changes
- Adds cache to building hub image in CI
- Optimizes CI by building and testing Go on the same runner to avoid duplicate compilation
- Freezes config to v6
- Deduplicates tool documentation into individual pages
- Adds docs-serve task for local Jekyll preview via Docker

### Pull Requests

- [#2037](https://github.com/docker/docker-agent/pull/2037) - Add cache to building hub image in CI
- [#2046](https://github.com/docker/docker-agent/pull/2046) - cost dialog: enrich with session stats, per-model percentages, and formatting fixes
- [#2048](https://github.com/docker/docker-agent/pull/2048) - fix: improve todo completion reliability
- [#2050](https://github.com/docker/docker-agent/pull/2050) - docs: update CHANGELOG.md for v1.30.1
- [#2052](https://github.com/docker/docker-agent/pull/2052) - Fix sonnet model name
- [#2056](https://github.com/docker/docker-agent/pull/2056) - Improve the toolsets documentation
- [#2059](https://github.com/docker/docker-agent/pull/2059) - Freeze config v6


## [v1.30.1] - 2026-03-11

This release improves command history handling, adds sound notifications, and includes various bug fixes and performance optimizations.

## What's New

- Adds sound notifications for long-running tasks and errors (opt-in feature, disabled by default)
- Adds LSP multiplexer to support multiple LSP toolsets simultaneously
- Adds per-toolset model routing via model field on toolsets configuration
- Adds click-to-copy functionality for working directory in TUI sidebar
- Makes background_agents a standalone toolset that can be enabled independently

## Improvements

- Improves tmux experience with better keyboard enhancements and focus handling
- Optimizes BM25 scoring strategy for better performance
- Reduces redundant work during evaluation runs
- Fixes animated spinners inside terminal multiplexers
- Repaints terminal on focus to fix broken display after tab switch in Docker Desktop

## Bug Fixes

- Fixes loading very long lines in command history that previously caused crashes
- Fixes LSP server being killed by context cancellation and restart failures
- Fixes session-pinned agent usage in RunStream instead of shared currentAgent
- Fixes sidebar context percentage flickering during sub-agent transfers
- Fixes concurrent map writes by moving registerDefaultTools to constructor
- Returns clear error when OPENAI_API_KEY is missing for speech-to-text

## Technical Changes

- Splits monolithic runtime.go into focused files by concern
- Refactors code to use slices and maps stdlib functions instead of manual implementations
- Enables modernize and perfsprint linters with all findings resolved
- Migrates tool output to structured JSON schemas for todo tools
- Replaces json.MarshalIndent with json.Marshal in builtin tools
- Uses errors.AsType consistently instead of errors.As with pre-declared variables

### Pull Requests

- [#1870](https://github.com/docker/docker-agent/pull/1870) - feat: add sound notifications for task completion and errors
- [#1940](https://github.com/docker/docker-agent/pull/1940) - history: Fix loading very long lines
- [#1970](https://github.com/docker/docker-agent/pull/1970) - Add LSP multiplexer to support multiple LSP toolsets
- [#2002](https://github.com/docker/docker-agent/pull/2002) - Don't ignore GITHUB_TOKEN
- [#2003](https://github.com/docker/docker-agent/pull/2003) - docs: update CHANGELOG.md for v1.30.0
- [#2005](https://github.com/docker/docker-agent/pull/2005) - Fix broken links to pages subsections
- [#2007](https://github.com/docker/docker-agent/pull/2007) - codemode: fix Start() fail-fast and use tools.As for wrapper unwrapping
- [#2008](https://github.com/docker/docker-agent/pull/2008) - Fix LSP server killed by context cancellation and restart failures
- [#2009](https://github.com/docker/docker-agent/pull/2009) - fix: use session-pinned agent in RunStream instead of shared currentAgent
- [#2010](https://github.com/docker/docker-agent/pull/2010) - refactor: split runtime.go and extract pkg/modelerrors
- [#2011](https://github.com/docker/docker-agent/pull/2011) - Bump direct Go dependencies
- [#2012](https://github.com/docker/docker-agent/pull/2012) - fix(#2012): Return clear error when OPENAI_API_KEY is missing for speech-to-text
- [#2013](https://github.com/docker/docker-agent/pull/2013) - fix(#2012): Return clear error when OPENAI_API_KEY is missing for speech-to-text
- [#2014](https://github.com/docker/docker-agent/pull/2014) - Replace duplicated mockEnvProvider test types with shared environment providers
- [#2015](https://github.com/docker/docker-agent/pull/2015) - feat: add per-toolset model routing via model field on toolsets
- [#2016](https://github.com/docker/docker-agent/pull/2016) - Simplify rulebased router: remove redundant types and score aggregation
- [#2017](https://github.com/docker/docker-agent/pull/2017) - tui: improve tmux experience and simplify keyboard enhancements
- [#2018](https://github.com/docker/docker-agent/pull/2018) - Unify streamAdapter/betaStreamAdapter retry logic into generic retryableStream
- [#2019](https://github.com/docker/docker-agent/pull/2019) - refactor(anthropic): deduplicate sequencing, media-type, and test helpers
- [#2020](https://github.com/docker/docker-agent/pull/2020) - docs: fix hallucinated CLI flags, commands, and config formats
- [#2021](https://github.com/docker/docker-agent/pull/2021) - refactor: use slices and maps stdlib functions instead of manual implementations
- [#2024](https://github.com/docker/docker-agent/pull/2024) - Fix task deploy-local
- [#2025](https://github.com/docker/docker-agent/pull/2025) - fix: default sound notifications to off (opt-in)
- [#2026](https://github.com/docker/docker-agent/pull/2026) - tui: repaint terminal on focus to fix broken display after tab switch
- [#2027](https://github.com/docker/docker-agent/pull/2027) - Enable modernize and perfsprint linters, fix all findings
- [#2028](https://github.com/docker/docker-agent/pull/2028) - refactor: use errors.AsType consistently instead of errors.As with pre-declared variables
- [#2029](https://github.com/docker/docker-agent/pull/2029) - refactor(dmr): split client.go into focused files by concern
- [#2030](https://github.com/docker/docker-agent/pull/2030) - refactor(runtime): split monolithic runtime.go into focused files
- [#2031](https://github.com/docker/docker-agent/pull/2031) - Replace json.MarshalIndent with json.Marshal in builtin tools
- [#2032](https://github.com/docker/docker-agent/pull/2032) - update Slack link in readme
- [#2033](https://github.com/docker/docker-agent/pull/2033) - feat: make background_agents a standalone toolset
- [#2034](https://github.com/docker/docker-agent/pull/2034) - Fix last brew install cagent mention
- [#2035](https://github.com/docker/docker-agent/pull/2035) - tui: fix animated spinners inside terminal multiplexers
- [#2036](https://github.com/docker/docker-agent/pull/2036) - feat: click to copy working directory in TUI sidebar
- [#2038](https://github.com/docker/docker-agent/pull/2038) - refactor: remove duplication in model resolution, thinking budget, and message construction
- [#2040](https://github.com/docker/docker-agent/pull/2040) - Use ResultSuccess/ResultError helpers in tasks and user_prompt tools
- [#2041](https://github.com/docker/docker-agent/pull/2041) - fix: move registerDefaultTools to constructor to prevent concurrent map writes
- [#2042](https://github.com/docker/docker-agent/pull/2042) - Fix sidebar context % flickering during sub-agent transfers
- [#2043](https://github.com/docker/docker-agent/pull/2043) - perf: optimize BM25 scoring strategy
- [#2045](https://github.com/docker/docker-agent/pull/2045) - todo: migrate tool output to structured JSON schemas
- [#2047](https://github.com/docker/docker-agent/pull/2047) - eval: reduce redundant work during evaluation runs


## [v1.30.0] - 2026-03-09

This release introduces file drag-and-drop support, background agent tasks, and completes the transition from "cagent" to "docker-agent" branding throughout the codebase.

## What's New

- Adds file drag-and-drop support for images and PDFs with visual file type indicators and 5MB size limit per file
- Adds background agent task tools (`run_background_agent`, `list_background_agents`, `view_background_agent`, `stop_background_agent`) for concurrent sub-agent dispatch
- Adds `--sandbox` flag to run command for Docker sandbox isolation
- Adds model_picker toolset for dynamic model switching between LLM models mid-conversation
- Adds search, update, categories, and default path functionality to memory tool
- Adds MiniMax as a built-in provider alias with `MINIMAX_API_KEY` support
- Adds top-level `mcps` section for reusable MCP server definitions in agent configs
- Adds support for OCI/catalog and URL references as sub-agents and handoffs

## Improvements

- Auto-continues max iterations in `--yolo` mode instead of prompting
- Improves toolset error reporting to show specific toolset information
- Improves user_prompt TUI dialog with title, free-form input, and navigation
- Auto-pulls DMR models in non-interactive mode
- Animates window title while working for tmux activity detection
- Supports comma-separated string format for allowed-tools in skills

## Bug Fixes

- Fixes thread blocking when attachment file is deleted
- Fixes max iterations handling in JSON output mode
- Fixes text to speech on macOS
- Fixes context window overflow with auto-recovery and proactive compaction
- Fixes data races in Session Messages slice and test functions
- Fixes SSE streaming by disabling automatic gzip compression
- Applies ModifiedInput from pre-tool hooks to tool call arguments

## Technical Changes

- Completes rename from "cagent" to "docker-agent" throughout codebase, documentation, and repository URLs
- Supports both `DOCKER_AGENT_*` and legacy `CAGENT_*` environment variables
- Removes `--exit-on-stdin-eof` flag and ConnectRPC code
- Adds timeouts to shutdown contexts to prevent goroutine leaks
- Extracts TodoStorage interface with in-memory implementation
- Refactors listener lifecycle to return cleanup functions
- Updates Dockerfile to use docker-agent binary with cagent as compatible symlink

### Pull Requests

- [#863](https://github.com/docker/docker-agent/pull/863) - Add background agent task tools for concurrent sub-agent dispatch (#863)
- [#1658](https://github.com/docker/docker-agent/pull/1658) - feat: add file drag-and-drop support for images and PDFs
- [#1736](https://github.com/docker/docker-agent/pull/1736) - fix(editor): prevent thread block when attachment file is deleted
- [#1737](https://github.com/docker/docker-agent/pull/1737) - fix(cli): auto-continue max iterations in --yolo mode
- [#1904](https://github.com/docker/docker-agent/pull/1904) - cagent run --sandbox
- [#1908](https://github.com/docker/docker-agent/pull/1908) - Add background agent task tools for concurrent sub-agent dispatch (#863)
- [#1909](https://github.com/docker/docker-agent/pull/1909) - docs: update CHANGELOG.md for v1.29.0
- [#1911](https://github.com/docker/docker-agent/pull/1911) - Fix #1911
- [#1913](https://github.com/docker/docker-agent/pull/1913) - Bump Go dependencies
- [#1914](https://github.com/docker/docker-agent/pull/1914) - agent: Improve toolset error reporting
- [#1915](https://github.com/docker/docker-agent/pull/1915) - Update docs and samples to rename docker-agent, change usage samples to `docker agent`
- [#1916](https://github.com/docker/docker-agent/pull/1916) - update taskfile to build both images docker/cagent and docker/docker-agent
- [#1917](https://github.com/docker/docker-agent/pull/1917) - Rename env vars CAGENT_ to DOCKER_AGENT_ (keep support for old env vars) 
- [#1918](https://github.com/docker/docker-agent/pull/1918) - Remove --exit-on-stdin-eof
- [#1921](https://github.com/docker/docker-agent/pull/1921) - Nightly scanner should be less nit-picky about docs
- [#1922](https://github.com/docker/docker-agent/pull/1922) - Fix speech to text on macOS
- [#1923](https://github.com/docker/docker-agent/pull/1923) - Simplify the AGENTS.md a LOT
- [#1924](https://github.com/docker/docker-agent/pull/1924) - Fix a few issues in the docs
- [#1925](https://github.com/docker/docker-agent/pull/1925) - Support auto-downloading tools
- [#1926](https://github.com/docker/docker-agent/pull/1926) - Rename CAGENT_HIDE_TELEMETRY & CAGENT_EXP_DEBUG_LAYOUT. Still support old env vars
- [#1927](https://github.com/docker/docker-agent/pull/1927) - docs: remove generated pages/ from git tracking
- [#1928](https://github.com/docker/docker-agent/pull/1928) - More docs rename (in / docs), fix remaining `docker agent serve a2a/acp/mcp` 
- [#1929](https://github.com/docker/docker-agent/pull/1929) - Fix test
- [#1930](https://github.com/docker/docker-agent/pull/1930) - Fix a few race conditions seen in tests
- [#1931](https://github.com/docker/docker-agent/pull/1931) - Fix #1911
- [#1932](https://github.com/docker/docker-agent/pull/1932) - Validate yaml in doc
- [#1933](https://github.com/docker/docker-agent/pull/1933) - Improve pkg/js
- [#1936](https://github.com/docker/docker-agent/pull/1936) - Improve README
- [#1937](https://github.com/docker/docker-agent/pull/1937) - Add model_picker toolset for dynamic model switching
- [#1938](https://github.com/docker/docker-agent/pull/1938) - Teach the agent to work with our config versions
- [#1939](https://github.com/docker/docker-agent/pull/1939) - Fix broken links in docs pages, were not using relative urls
- [#1941](https://github.com/docker/docker-agent/pull/1941) - Improve sub-sessions usage
- [#1942](https://github.com/docker/docker-agent/pull/1942) - Show the new TUI
- [#1943](https://github.com/docker/docker-agent/pull/1943) - Improve user_prompt TUI dialog: title, free-form input, and navigation
- [#1944](https://github.com/docker/docker-agent/pull/1944) - Auto-pull DMR models in non-interactive mode
- [#1945](https://github.com/docker/docker-agent/pull/1945) - Fix listener resource leaks in serve commands
- [#1946](https://github.com/docker/docker-agent/pull/1946) - Support OCI/catalog and URL references as sub-agents and handoffs
- [#1947](https://github.com/docker/docker-agent/pull/1947) - Add top-level mcps section for reusable MCP server definitions
- [#1948](https://github.com/docker/docker-agent/pull/1948) - Add MiniMax as a built-in provider alias
- [#1949](https://github.com/docker/docker-agent/pull/1949) - Animate window title while working for tmux activity detection
- [#1950](https://github.com/docker/docker-agent/pull/1950) - fix(hooks): apply ModifiedInput from pre-tool hooks to tool call arguments
- [#1953](https://github.com/docker/docker-agent/pull/1953) - Bump go dependencies
- [#1954](https://github.com/docker/docker-agent/pull/1954) - bump google.golang.org/adk from v0.4.0 to v0.5.0
- [#1955](https://github.com/docker/docker-agent/pull/1955) - Leverage latest MCP spec features from go-sdk v1.4.0
- [#1957](https://github.com/docker/docker-agent/pull/1957) - Rename repo URL and pages URL
- [#1958](https://github.com/docker/docker-agent/pull/1958) - Use docker agent command
- [#1959](https://github.com/docker/docker-agent/pull/1959) - Improve docs search
- [#1960](https://github.com/docker/docker-agent/pull/1960) - todo: extract storage interface with in-memory implementation
- [#1961](https://github.com/docker/docker-agent/pull/1961) - docker-agent is primary binary in taskfile
- [#1962](https://github.com/docker/docker-agent/pull/1962) - A few more renames from cagent
- [#1964](https://github.com/docker/docker-agent/pull/1964) - Some more cagent urls
- [#1965](https://github.com/docker/docker-agent/pull/1965) - Add timeouts to shutdown contexts to prevent goroutine leaks
- [#1967](https://github.com/docker/docker-agent/pull/1967) - Disable automatic gzip compression to fix SSE streaming
- [#1968](https://github.com/docker/docker-agent/pull/1968) - Fix main branch
- [#1971](https://github.com/docker/docker-agent/pull/1971) - Add search, update, categories, and default path to memory tool
- [#1972](https://github.com/docker/docker-agent/pull/1972) - Update winget workflow to modify Docker.Agent package, with the new GH repo name
- [#1973](https://github.com/docker/docker-agent/pull/1973) - Fix context window overflow: auto-recovery and proactive compaction
- [#1974](https://github.com/docker/docker-agent/pull/1974) - updated GHA with new checks:write permission
- [#1979](https://github.com/docker/docker-agent/pull/1979) - Fix cobra command and rename more things from cagent to docker agent
- [#1983](https://github.com/docker/docker-agent/pull/1983) - Fix documentation
- [#1984](https://github.com/docker/docker-agent/pull/1984) - Support comma-separated string for allowed-tools in skills
- [#1988](https://github.com/docker/docker-agent/pull/1988) - Fix gopls versions
- [#1989](https://github.com/docker/docker-agent/pull/1989) - auto-complete tests
- [#1990](https://github.com/docker/docker-agent/pull/1990) - Daily fixes
- [#1991](https://github.com/docker/docker-agent/pull/1991) - Fix model name
- [#1992](https://github.com/docker/docker-agent/pull/1992) - Dockerfile with docker-agent binary, keeping cagent only as compatible symlink
- [#1993](https://github.com/docker/docker-agent/pull/1993) - Rename cagent in eval
- [#1994](https://github.com/docker/docker-agent/pull/1994) - More renames from cagent to docker-agent
- [#1995](https://github.com/docker/docker-agent/pull/1995) - Fix documentation
- [#1996](https://github.com/docker/docker-agent/pull/1996) - Remove ConnectRPC code
- [#1997](https://github.com/docker/docker-agent/pull/1997) - Rename e2e test files
- [#1998](https://github.com/docker/docker-agent/pull/1998) - Remove useless documentation
- [#1999](https://github.com/docker/docker-agent/pull/1999) - More renames
- [#2000](https://github.com/docker/docker-agent/pull/2000) - Remove package to github.com/docker/docker-agent
- [#2001](https://github.com/docker/docker-agent/pull/2001) - Remove my name :-)


## [v1.29.0] - 2026-03-03

This release adds automated issue triage capabilities and new CLI configuration options for directory overrides.

## What's New
- Adds auto issue triage workflow that automatically evaluates bug reports and can create fix PRs
- Adds `--config-dir`, `--data-dir`, and `--cache-dir` global CLI flags to override default paths

## Bug Fixes
- Fixes result marker parsing in auto-issue-triage workflow to handle LLM output with trailing empty lines
- Fixes GitHub Pages deployment issues

## Technical Changes
- Updates nightly scanner documentation and configuration
- Removes draft status from PR creation workflow steps
- Adds tip about the default agent in documentation

### Pull Requests

- [#1888](https://github.com/docker/docker-agent/pull/1888) - feat: add auto issue triage workflow
- [#1901](https://github.com/docker/docker-agent/pull/1901) - Fix GitHub pages deployment
- [#1902](https://github.com/docker/docker-agent/pull/1902) - docs: update CHANGELOG.md for v1.28.1
- [#1903](https://github.com/docker/docker-agent/pull/1903) - Fix the github pages?
- [#1905](https://github.com/docker/docker-agent/pull/1905) - Replace the brittle tail -n 1 parsing with something that searches for the marker
- [#1906](https://github.com/docker/docker-agent/pull/1906) - Add tip about the default agent
- [#1907](https://github.com/docker/docker-agent/pull/1907) - Add --config-dir and --data-dir global CLI flags to override default paths


## [v1.28.1] - 2026-03-03

This release adds image support for AI agents, improves cross-platform compatibility, and includes various stability fixes.

## What's New
- Adds image support to read_file tool and MCP tool results, allowing agents to view and describe images
- Adds content-based MIME detection and automatic image resizing for vision capabilities
- Strips image content for text-only models using model capabilities detection

## Improvements
- Reduces builtin tool prompt lengths while preserving key examples for better performance
- Skips hidden directories in recursive skill loading to avoid walking large trees like .git and .node_modules
- Only uses insecure TLS for localhost OTLP endpoints for better security

## Bug Fixes
- Fixes Esc key not interrupting sub-agents in multi-agent sessions
- Fixes slice bounds out of range panic for short JWT tokens
- Fixes goroutine tight loop in LSP readNotifications
- Fixes race condition with elicitation events channel
- Avoids looping forever on symlinks during skill loading
- Handles json.Marshal errors for tool Parameters and OutputSchema

## Technical Changes
- Replaces syscall.Rmdir with golang.org/x/sys for cross-platform directory removal
- Removes per-chunk UpdateMessage debug log from SQLite store to reduce log noise
- Stops tool sets for team loaded in GetAgentToolCount
- Migrates GitHub pages to markdown with Jekyll

### Pull Requests

- [#1875](https://github.com/docker/docker-agent/pull/1875) - Skip hidden directories in recursive skill loading
- [#1879](https://github.com/docker/docker-agent/pull/1879) - Reduce builtin tool prompt lengths while preserving key examples
- [#1885](https://github.com/docker/docker-agent/pull/1885) - Replace syscall.Rmdir with golang.org/x/sys for cross-platform directory removal
- [#1889](https://github.com/docker/docker-agent/pull/1889) - :eyes: Vision :eyes:
- [#1892](https://github.com/docker/docker-agent/pull/1892) - docs: update CHANGELOG.md for v1.28.0
- [#1893](https://github.com/docker/docker-agent/pull/1893) - Fixes to the documentation
- [#1895](https://github.com/docker/docker-agent/pull/1895) - Daily fixes of the bot-detected issues
- [#1896](https://github.com/docker/docker-agent/pull/1896) - Remove per-chunk UpdateMessage debug log
- [#1897](https://github.com/docker/docker-agent/pull/1897) - Pushes docker/docker-agent next to docker/cagent hub image
- [#1899](https://github.com/docker/docker-agent/pull/1899) - fix: Esc key not interrupting sub-agents in multi-agent sessions
- [#1900](https://github.com/docker/docker-agent/pull/1900) - Migrate our GitHub pages to markdown, with Jekyll


## [v1.28.0] - 2026-03-03

This release improves authentication debugging, session management, and MCP server reliability, along with UI enhancements to the command palette.

## What's New
- Adds 'debug auth' command to inspect Docker Desktop JWT with optional JSON output
- Adds automatic retry functionality for all models, including those without fallbacks

## Improvements
- Improves MCP server lifecycle with caching and auto-restart capabilities using exponential backoff
- Sorts command palette actions alphabetically within each group
- Uses tea.View.ProgressBar instead of raw escape codes for better display

## Bug Fixes
- Fixes session derailment by preserving user messages during conversation trimming
- Fixes duplicate Session header in command palette on macOS
- Fixes mcp/notion not working with OpenAI models by properly walking additionalProperties in schemas
- Defaults to string type for script tool arguments when type is not specified

## Technical Changes
- Updates tool filtering documentation
- Updates CHANGELOG.md for v1.27.1
- Updates Charm libraries to stable v2.0.0 releases (bubbletea, bubbles, lipgloss)

### Pull Requests

- [#1859](https://github.com/docker/docker-agent/pull/1859) - Fix script args with DMR
- [#1861](https://github.com/docker/docker-agent/pull/1861) - Add 'debug auth' command to inspect Docker Desktop JWT
- [#1862](https://github.com/docker/docker-agent/pull/1862) - docs: update CHANGELOG.md for v1.27.1
- [#1863](https://github.com/docker/docker-agent/pull/1863) - fix(#1863): preserve user messages in trimMessages to prevent session derailment
- [#1864](https://github.com/docker/docker-agent/pull/1864) - fix(#1863): preserve user messages in trimMessages to prevent session derailment
- [#1871](https://github.com/docker/docker-agent/pull/1871) - Fix `mcp/notion` not working with OpenAI models
- [#1872](https://github.com/docker/docker-agent/pull/1872) - Improve MCP server lifecycle: caching and auto-restart
- [#1874](https://github.com/docker/docker-agent/pull/1874) - Improve tool filtering doc
- [#1876](https://github.com/docker/docker-agent/pull/1876) - Bump dependencies
- [#1877](https://github.com/docker/docker-agent/pull/1877) - Improve Commands dialog
- [#1886](https://github.com/docker/docker-agent/pull/1886) - Add retries even for models without fallbacks


## [v1.27.1] - 2026-02-26

This release improves the user interface experience with better message editing capabilities and fixes several issues with token usage tracking and session loading.

## What's New
- Adds `on_user_input` hook that triggers when the agent is waiting for user input or tool confirmation

## Improvements
- Improves multi-line editing of past user messages
- Adds clipboard paste support during inline message editing
- Makes loading past sessions faster
- Updates TUI display when the current agent changes

## Bug Fixes
- Fixes token usage being recorded multiple times per stream, preventing inflated telemetry counts
- Fixes empty inline edit textarea expanding to full height
- Fixes docker ai shellout to cagent for standalone invocations

## Technical Changes
- Updates schema tests to only run for latest version
- Fixes documentation issues

### Pull Requests

- [#1845](https://github.com/docker/docker-agent/pull/1845) - Repaint the TUI when the current agent changes
- [#1846](https://github.com/docker/docker-agent/pull/1846) - docs: update CHANGELOG.md for v1.27.0
- [#1847](https://github.com/docker/docker-agent/pull/1847) - feat(hooks): add on_user_input
- [#1850](https://github.com/docker/docker-agent/pull/1850) - Improve editing past user messages
- [#1854](https://github.com/docker/docker-agent/pull/1854) - Make loading past sessions faster
- [#1855](https://github.com/docker/docker-agent/pull/1855) - fix: record token usage once per stream to prevent inflated telemetry
- [#1857](https://github.com/docker/docker-agent/pull/1857) - Schema tests should be only for latest version
- [#1858](https://github.com/docker/docker-agent/pull/1858) - Fix doc
- [#1860](https://github.com/docker/docker-agent/pull/1860) - Fix docker ai shellout to cagent


## [v1.27.0] - 2026-02-25

This release introduces dynamic agent color styling for multi-agent teams, adds new filesystem tools, and includes several bug fixes and security improvements.

## What's New

- Adds dynamic agent color styling system that assigns unique, deterministic colors to each agent in multi-agent teams for visual distinction across the TUI
- Adds hue-based agent color generation with theme integration that adapts saturation and lightness based on theme background
- Adds mkdir and rmdir filesystem tools so agents can create and remove directories without using shell commands
- Allows .github and .gitlab directories in WalkFiles traversal for better CI workflow support

## Bug Fixes

- Fixes race condition in agent color style lookups
- Fixes path traversal vulnerability in ACP filesystem operations
- Fixes YAML marshalling issues that could produce corrupted configuration files
- Handles case-insensitive filesystems properly
- Logs errors when persisting session title in TUI

## Technical Changes

- Consolidates color utilities into styles/colorutil.go
- Unexports internal color helpers and deduplicates fallbacks
- Fixes cassettes functionality

### Pull Requests

- [#1756](https://github.com/docker/docker-agent/pull/1756) - feat(#1756): Add dynamic agent color styling system
- [#1757](https://github.com/docker/docker-agent/pull/1757) - feat(#1756): Add dynamic agent color styling system
- [#1781](https://github.com/docker/docker-agent/pull/1781) - tools/fs: Add mkdir and rmdir
- [#1832](https://github.com/docker/docker-agent/pull/1832) - Daily fixes
- [#1833](https://github.com/docker/docker-agent/pull/1833) - allow .github and .gitlab directories in WalkFiles traversal
- [#1841](https://github.com/docker/docker-agent/pull/1841) - docs: update CHANGELOG.md for v1.26.0
- [#1844](https://github.com/docker/docker-agent/pull/1844) - Fix yaml marshalling


## [v1.26.0] - 2026-02-24

This is a maintenance release with dependency updates and internal improvements.

## Technical Changes
- Maintenance release with dependency updates



## [v1.24.0] - 2026-02-24

This release introduces remote skills discovery capabilities and improves file reading tools with pagination support.

## What's New
- Adds remote skills discovery with disk cache and dedicated tools, supporting the well-known skills discovery specification
- Adds offset and line_count pagination parameters to read_file and read_multiple_files tools for incremental reading of large files

## Improvements
- Limits output size for read_file and read_multiple_files tools to prevent excessive token usage
- Removes pagination instructions from tool descriptions for cleaner interface

## Bug Fixes
- Fixes LineCount metadata on truncated read_multiple_files results

## Technical Changes
- Freezes configuration version v5 and bumps to v6
- Updates test cassettes to match schema changes for file reading tools

### Pull Requests

- [#1810](https://github.com/docker/docker-agent/pull/1810) - Freeze v5 (and a few refactoring)
- [#1822](https://github.com/docker/docker-agent/pull/1822) - Implement remote skills discovery with disk cache and dedicated tools
- [#1828](https://github.com/docker/docker-agent/pull/1828) - builtin: add offset and line_count pagination to read_file and read_multiple_files
- [#1829](https://github.com/docker/docker-agent/pull/1829) - docs: update CHANGELOG.md for v1.23.6


## [v1.23.6] - 2026-02-23

This release improves cost tracking accuracy, enhances session management, and fixes several UI and functionality issues.

## What's New

- Adds tab completion for /commands dialog
- Adds mouse support for selecting and opening sessions in the sessions dialog

## Improvements

- Computes session cost from messages instead of accumulating on session for better accuracy
- Includes compaction cost in /cost dialog
- Displays original YAML model names in sidebar instead of resolved aliases
- Improves emoji copying support by reversing clipboard copy order (OSC52 first, then pbcopy fallback)

## Bug Fixes

- Fixes token usage percentage display during and after agent transfers
- Fixes session forking and costs calculation
- Fixes actual provider display for alloy models in sidebar (was showing wrong provider)
- Restores ctrl-1, ctrl-2... shortcuts for quick agent selection
- Fixes NewHandler panic on parameterless tool calls

## Technical Changes

- Consolidates TokenUsage event constructors
- Removes dead UpdateLastAssistantMessageUsage method
- Emits TokenUsageEvent on session restore for context percentage display
- Emits TokenUsageEvent after compaction so sidebar cost updates
- Adds e2e tests on binaries for CLI plugin execution
- Creates ~/.docker/cli-plugins directory if it doesn't exist

### Pull Requests

- [#1795](https://github.com/docker/docker-agent/pull/1795) - Fix multiple cost/tokens related issues
- [#1803](https://github.com/docker/docker-agent/pull/1803) - docs: update CHANGELOG.md for v1.23.5
- [#1804](https://github.com/docker/docker-agent/pull/1804) - Better support copying emojis
- [#1806](https://github.com/docker/docker-agent/pull/1806) - Tab completion for /commands dialog
- [#1807](https://github.com/docker/docker-agent/pull/1807) - fix: use actual provider for alloy models in sidebar
- [#1808](https://github.com/docker/docker-agent/pull/1808) - Update winget workflow
- [#1811](https://github.com/docker/docker-agent/pull/1811) - Improve sessions dialog
- [#1812](https://github.com/docker/docker-agent/pull/1812) - Binary e2e tests
- [#1813](https://github.com/docker/docker-agent/pull/1813) - feat: use docker read write bot
- [#1816](https://github.com/docker/docker-agent/pull/1816) - fix: restore ctrl-1, ctrl-2... shortcuts for quick agent selection
- [#1817](https://github.com/docker/docker-agent/pull/1817) - Bump Go dependencies
- [#1826](https://github.com/docker/docker-agent/pull/1826) - Refactor winget workflow to use wingetcreate CLI
- [#1827](https://github.com/docker/docker-agent/pull/1827) - get_memories errors on new memories


## [v1.23.5] - 2026-02-20

This release improves the session browser interface and fixes several issues with the docker-agent standalone binary.

## Improvements
- Shows message count in session browser dialog for better session overview

## Bug Fixes
- Fixes recognition of cobra internal completion commands as subcommands
- Fixes help text display for docker-agent standalone binary exec
- Fixes version output for docker-agent CLI plugin and standalone exec

## Technical Changes
- Renames internal schema structure

### Pull Requests

- [#1792](https://github.com/docker/docker-agent/pull/1792) - docs: update CHANGELOG.md for v1.23.4
- [#1796](https://github.com/docker/docker-agent/pull/1796) - Fix help for docker-agent standalone binary exec
- [#1802](https://github.com/docker/docker-agent/pull/1802) - Fix docker-agent version for cli plugin & standalone exec


## [v1.23.4] - 2026-02-19

This release introduces parallel session support with tab management, major command restructuring, and enhanced UI interactions.

## What's New

- Adds parallel session support with a new tab view to switch between sessions
- Adds drag and drop functionality for reordering tabs
- Adds mouse click support to elicitation, prompt input, and tool confirmation dialogs
- Adds `X-Cagent-Model-Name` header to models gateway requests
- Adds Ask list to permissions config to force confirmation for read-only tools
- Defaults to running the default agent when no subcommand is given

## Improvements

- Restores ctrl-r binding for searching prompt history
- Updates Claude Sonnet model version to 4.6
- Prevents closing the last remaining tab with Ctrl+W
- Makes fetch tool not read-only
- Handles Claude overloaded_error with retry logic

## Bug Fixes

- Fixes ctrl-c in docker agent and `docker agent` defaulting to `docker agent run`
- Fixes completion command
- Fixes cagent-action to expect a prompt
- Fixes gemini use of vertexai environment variables
- Fixes CPU profile file handling and error handling in isFirstRun

## Technical Changes

- Removes `cagent config` commands (breaking change)
- Removes `cagent feedback` command (breaking change)
- Removes `cagent build` command (breaking change)
- Removes `cagent catalog` command (breaking change)
- Moves a2a, acp, mcp and api commands under `cagent serve` (breaking change)
- Replaces `cagent exec` with `cagent run --exec` (breaking change)
- Moves pull and push under `cagent share` (breaking change)
- Hides `cagent debug`
- Adds skills to the default agent
- Defaults restore_tabs to false

### Pull Requests

- [#1751](https://github.com/docker/docker-agent/pull/1751) - feat: add `X-Cagent-Model-Name` header to models gateway requests
- [#1753](https://github.com/docker/docker-agent/pull/1753) - docs: update CHANGELOG.md for v1.23.3
- [#1755](https://github.com/docker/docker-agent/pull/1755) - Review cagent commands
- [#1759](https://github.com/docker/docker-agent/pull/1759) - Restore ctrl-r binding for searching prompt history
- [#1761](https://github.com/docker/docker-agent/pull/1761) - Fix completion command
- [#1762](https://github.com/docker/docker-agent/pull/1762) - fix: cagent-action expects a prompt
- [#1763](https://github.com/docker/docker-agent/pull/1763) - fix: gemini use of vertexai environment variables 
- [#1766](https://github.com/docker/docker-agent/pull/1766) - Add mouse click support to elicitation, prompt input, and tool confirmation dialogs
- [#1768](https://github.com/docker/docker-agent/pull/1768) - chore(config): Update Claude Sonnet model version to 4.6
- [#1772](https://github.com/docker/docker-agent/pull/1772) - drag 'n drop tabs
- [#1773](https://github.com/docker/docker-agent/pull/1773) - temp home dir to avoid issues in some environments
- [#1777](https://github.com/docker/docker-agent/pull/1777) - Bump Go dependencies
- [#1780](https://github.com/docker/docker-agent/pull/1780) - fallback: Handle overloaded_error
- [#1782](https://github.com/docker/docker-agent/pull/1782) - Fix ctrl-c in `docker agent serve api` and fix `docker agent` defaulting to `docker agent run`
- [#1785](https://github.com/docker/docker-agent/pull/1785) - permissions: add Ask list to force confirmation for tools
- [#1786](https://github.com/docker/docker-agent/pull/1786) - Make fetch tool not read-only
- [#1787](https://github.com/docker/docker-agent/pull/1787) - Daily fixes for the Nightly issue detector
- [#1788](https://github.com/docker/docker-agent/pull/1788) - Fix path and typo
- [#1789](https://github.com/docker/docker-agent/pull/1789) - Keep same error handling for main cli plugin execution
- [#1790](https://github.com/docker/docker-agent/pull/1790) - tui/tabbar: Prevent closing the last remaining tab


## [v1.23.3] - 2026-02-16

This release adds Docker CLI plugin support and improves TUI performance by making model reasoning checks asynchronous.

## What's New
- Adds support for using cagent as a Docker CLI plugin with `docker agent` command (no functional changes to existing `cagent` command)
- Handles Windows .exe binary suffix for CLI plugin compatibility

## Improvements
- Makes model reasoning support checks asynchronous to prevent TUI freezing (previously could block for up to 30 seconds)
- Threads context.Context through modelsdev store API to allow proper cancellation and deadline propagation

## Technical Changes
- Renames cagent OCI annotation to `io.docker.agent.version` while maintaining backward compatibility with the old annotation
- Updates config media type to use `docker.agent`
- Adds TUI general guidelines to AGENTS.md documentation

### Pull Requests

- [#1745](https://github.com/docker/docker-agent/pull/1745) - Rename cagent OCI annotation, keep old one
- [#1746](https://github.com/docker/docker-agent/pull/1746) - docs: update CHANGELOG.md for v1.23.2
- [#1747](https://github.com/docker/docker-agent/pull/1747) - Thread context.Context through modelsdev store API
- [#1748](https://github.com/docker/docker-agent/pull/1748) - Allow to use cagent binary as a docker cli plugin docker-agent. No functional change for cagent command.
- [#1749](https://github.com/docker/docker-agent/pull/1749) - Move ModelSupportsReasoning calls to async bubbletea commands


## [v1.23.2] - 2026-02-16

This release adds header forwarding capabilities for toolsets and includes several bug fixes and code improvements.

## What's New
- Adds support for `${headers.NAME}` syntax to forward upstream API headers to toolsets, allowing toolset configurations to reference incoming HTTP request headers

## Bug Fixes
- Fixes race condition in isFirstRun using atomic file creation
- Fixes nil pointer dereference when RateLimit is present without Usage
- Fixes double-counting of session costs with cumulative usage providers
- Fixes Ctrl+K key binding conflict in session browser by reassigning CopyID to Ctrl+Y
- Fixes model selection functionality

## Improvements
- Adds input validation and audit logging to shell tool
- Adds input validation and error handling to RunBangCommand

## Technical Changes
- Extracts shared helpers for command-based providers to reduce code duplication
- Removes duplication from config.Resolv
- Moves GetUserSettings() from pkg/config to pkg/userconfig as Get()
- Removes redundant Reader interface from pkg/config
- Fixes leaked os.Root handle in fileSource.Read
- Makes small improvements to cmd/root

### Pull Requests

- [#1725](https://github.com/docker/docker-agent/pull/1725) - Support ${headers.NAME} syntax to forward upstream API headers to toolsets
- [#1727](https://github.com/docker/docker-agent/pull/1727) - docs: update CHANGELOG.md for v1.23.1
- [#1729](https://github.com/docker/docker-agent/pull/1729) - Cleanup config code
- [#1730](https://github.com/docker/docker-agent/pull/1730) - refactor(environment): extract shared helpers for command-based providers
- [#1731](https://github.com/docker/docker-agent/pull/1731) - Daily fixes
- [#1732](https://github.com/docker/docker-agent/pull/1732) - Fix two issues with costs
- [#1734](https://github.com/docker/docker-agent/pull/1734) - Small improvements to cmd/root
- [#1740](https://github.com/docker/docker-agent/pull/1740) - Fix model switcher
- [#1741](https://github.com/docker/docker-agent/pull/1741) - fix(#1741): resolve Ctrl+K key binding conflict in session browser
- [#1742](https://github.com/docker/docker-agent/pull/1742) - fix(#1741): resolve Ctrl+K key binding conflict in session browser


## [v1.23.1] - 2026-02-13

This release introduces a new OpenAPI toolset for automatic API integration, task management capabilities, and several improvements to message handling and testing infrastructure.

## What's New

- Adds Tasks toolset with support for priorities and dependencies
- Adds OpenAPI built-in toolset type that automatically converts OpenAPI specifications into usable tools
- Adds support for custom telemetry tags via `TELEMETRY_TAGS` environment variable

## Improvements

- Preserves line breaks and indentation in welcome messages for better formatting
- Updates documentation links to point to GitHub Pages instead of code repository

## Bug Fixes

- Fixes recursive enforcement of required properties in OpenAI tool schemas (resolves Chrome MCP compatibility with OpenAI 5.2)
- Returns error when no messages are available after conversion instead of sending invalid requests

## Technical Changes

- Replaces time.Sleep in tests with deterministic synchronization for faster, more reliable testing
- Refactors models store implementation
- Adds .idea/ directory to gitignore
- Removes fake models.dev and unused code

### Pull Requests

- [#1704](https://github.com/docker/docker-agent/pull/1704) - Tasks toolset
- [#1710](https://github.com/docker/docker-agent/pull/1710) - fix: recursively enforce required properties in OpenAI tool schemas
- [#1714](https://github.com/docker/docker-agent/pull/1714) - docs: update CHANGELOG.md for v1.23.0
- [#1718](https://github.com/docker/docker-agent/pull/1718) - preserve line breaks and indentation in welcome messages
- [#1719](https://github.com/docker/docker-agent/pull/1719) - Add openapi built-in toolset type
- [#1720](https://github.com/docker/docker-agent/pull/1720) - return error if no messages are available after conversion
- [#1721](https://github.com/docker/docker-agent/pull/1721) - Refactor models store
- [#1722](https://github.com/docker/docker-agent/pull/1722) - Replace time.Sleep in tests with deterministic synchronization
- [#1723](https://github.com/docker/docker-agent/pull/1723) - Allow passing in custom tags to telemetry
- [#1724](https://github.com/docker/docker-agent/pull/1724) - Speed up fallback tests
- [#1726](https://github.com/docker/docker-agent/pull/1726) - Update documentation links to GitHub Pages


## [v1.23.0] - 2026-02-12

This release improves TUI display accuracy, enhances API security defaults, and fixes several memory leaks and session handling issues.

## What's New

- Adds optional setup script support for evaluation sessions to prepare container environments before agent execution
- Adds user_prompt tools to the planner for interactive user questions

## Improvements

- Makes session compaction non-blocking with spinner feedback instead of blocking the TUI render thread
- Returns error responses for unknown tool calls instead of silently skipping them
- Strips null values from MCP tool call arguments to fix compatibility with models like GPT-5.2
- Improves error handling and logging in evaluation judge with better error propagation and structured logging

## Bug Fixes

- Fixes incorrect tool count display in TUI when running in --remote mode
- Fixes tick leak that caused ~10% CPU usage when assistant finished answering
- Fixes session store leak and removes redundant session store methods
- Fixes A2A agent card advertising unroutable wildcard address by using localhost
- Fixes potential goroutine leak in monitorStdin
- Fixes Agents.UnmarshalYAML to properly reject unknown fields in agent configurations
- Persists tool call error state in session messages so failed tool calls maintain error status when sessions are reloaded

## Technical Changes

- Removes CORS middleware from 'cagent api' command
- Changes default binding from 0.0.0.0 to 127.0.0.1:8080 for 'cagent api', 'cagent a2a' and 'cagent mcp' commands
- Uses different default ports for better security
- Lists valid versions in unsupported config version error messages
- Adds the summary message as a user message during session compaction
- Propagates cleanup errors from fakeCleanup and recordCleanup functions
- Logs errors on log file close instead of discarding them

### Pull Requests

- [#1648](https://github.com/docker/docker-agent/pull/1648) - fix: show correct tool count in TUI when running in --remote mode
- [#1657](https://github.com/docker/docker-agent/pull/1657) - Better default security for cagent api|mcp|a2a
- [#1663](https://github.com/docker/docker-agent/pull/1663) - docs: update CHANGELOG.md for v1.22.0
- [#1668](https://github.com/docker/docker-agent/pull/1668) - Session store cleanup
- [#1669](https://github.com/docker/docker-agent/pull/1669) - Fix tick leak
- [#1673](https://github.com/docker/docker-agent/pull/1673) - eval: add optional setup script support for eval sessions
- [#1684](https://github.com/docker/docker-agent/pull/1684) - Fix Agents.UnmarshalYAML to reject unknown fields
- [#1685](https://github.com/docker/docker-agent/pull/1685) - Fix A2A agent card advertising unroutable wildcard address
- [#1686](https://github.com/docker/docker-agent/pull/1686) - Close the session
- [#1687](https://github.com/docker/docker-agent/pull/1687) - Make /compact non-blocking with spinner feedback
- [#1688](https://github.com/docker/docker-agent/pull/1688) - Remove redundant stdin nil check in api command
- [#1689](https://github.com/docker/docker-agent/pull/1689) - Return error response for unknown tool calls instead of silently skipping
- [#1692](https://github.com/docker/docker-agent/pull/1692) - Add documentation gh-pages
- [#1693](https://github.com/docker/docker-agent/pull/1693) - Add the summary message as a user message
- [#1694](https://github.com/docker/docker-agent/pull/1694) - Add more documentation
- [#1696](https://github.com/docker/docker-agent/pull/1696) - Fix MCP tool calls with gpt 5.2
- [#1697](https://github.com/docker/docker-agent/pull/1697) - Bump Go to 1.26.0
- [#1699](https://github.com/docker/docker-agent/pull/1699) - Fix issues found by the review agent
- [#1700](https://github.com/docker/docker-agent/pull/1700) - List valid versions in unsupported config version error
- [#1703](https://github.com/docker/docker-agent/pull/1703) - Bump direct Go dependencies
- [#1705](https://github.com/docker/docker-agent/pull/1705) - Improve the Planner
- [#1706](https://github.com/docker/docker-agent/pull/1706) - Improve error handling and logging in evaluation judge
- [#1711](https://github.com/docker/docker-agent/pull/1711) - Persist tool call error state in session messages


## [v1.22.0] - 2026-02-09

This release enhances the chat experience with history search functionality and improves file attachment handling, along with multi-turn conversation support for command-line operations.

## What's New

- Adds Ctrl+R reverse history search to the chat editor for quickly finding previous conversations
- Adds support for multi-turn conversations in `cagent exec`, `cagent run`, and `cagent eval` commands
- Adds support for queueing multiple messages with `cagent run question1 question2 ...`

## Improvements

- Improves file attachment handling by inlining text-based files and fixing placeholder stripping
- Refactors scrollbar into a reusable scrollview component for more consistent scrolling behavior across the interface

## Bug Fixes

- Fixes pasted attachments functionality
- Fixes persistence of multi_content for user messages to ensure attachment data is properly saved
- Fixes session browser shortcuts (star, filter, copy-id) to use Ctrl modifier, preventing conflicts with search input
- Fixes title generation spinner that could spin forever
- Fixes scrollview height issues when used with dialogs
- Fixes double @@ symbols when using file picker for @ attachments

## Technical Changes

- Updates OpenAI schema format handling to improve compatibility

### Pull Requests

- [#1630](https://github.com/docker/docker-agent/pull/1630) - feat: add Ctrl+R reverse history search
- [#1640](https://github.com/docker/docker-agent/pull/1640) - better file attachments
- [#1645](https://github.com/docker/docker-agent/pull/1645) - Prevent title generation spinner to spin forever
- [#1649](https://github.com/docker/docker-agent/pull/1649) - docs: update CHANGELOG.md for v1.21.0
- [#1650](https://github.com/docker/docker-agent/pull/1650) - OpenAI doesn't like those format indications on the schema
- [#1652](https://github.com/docker/docker-agent/pull/1652) - Fix: persist multi_content for user messages
- [#1654](https://github.com/docker/docker-agent/pull/1654) - Refactor scrollbar into more reusable `scrollview` component
- [#1656](https://github.com/docker/docker-agent/pull/1656) - fix: use ctrl modifier for session browser shortcuts to avoid search conflict
- [#1659](https://github.com/docker/docker-agent/pull/1659) - Fix pasted attachments
- [#1661](https://github.com/docker/docker-agent/pull/1661) - deleting version 2 so i can use permissions
- [#1662](https://github.com/docker/docker-agent/pull/1662) - Multi turn (cagent exec|run|eval)


## [v1.21.0] - 2026-02-09

This release adds a new generalist coding agent, improves agent configuration handling, and includes several bug fixes and UI improvements.

## What's New
- Adds a generalist coding agent for enhanced coding assistance
- Adds OCI artifact wrapper for spec-compliant manifest with artifactType

## Improvements
- Supports recursive ~/.agents/skills directory structure
- Wraps todo descriptions at word boundaries in sidebar for better display
- Preserves 429 error details on OpenAI for better error handling

## Bug Fixes
- Fixes subagent delegation and validates model outputs when transfer_task is called
- Fixes YAML parsing issue with unquoted strings containing special characters like colons

## Technical Changes
- Freezes config version v4 and bumps to v5

### Pull Requests

- [#1419](https://github.com/docker/docker-agent/pull/1419) - Help fix #1419
- [#1625](https://github.com/docker/docker-agent/pull/1625) - Add a generalist coding agent
- [#1631](https://github.com/docker/docker-agent/pull/1631) - Support recursive ~/.agents/skills
- [#1632](https://github.com/docker/docker-agent/pull/1632) - Help fix #1419
- [#1633](https://github.com/docker/docker-agent/pull/1633) - Add OCI artifact wrapper for spec-compliant manifest with artifactType
- [#1634](https://github.com/docker/docker-agent/pull/1634) - docs: update CHANGELOG.md for v1.20.6
- [#1635](https://github.com/docker/docker-agent/pull/1635) - Freeze v4 and bump config version to v5
- [#1637](https://github.com/docker/docker-agent/pull/1637) - Fix subagent logic
- [#1641](https://github.com/docker/docker-agent/pull/1641) - unquoted strings are fine until they contain special characters like :
- [#1643](https://github.com/docker/docker-agent/pull/1643) - Wrap todo descriptions at word boundaries in sidebar
- [#1646](https://github.com/docker/docker-agent/pull/1646) - Bump Go dependencies
- [#1647](https://github.com/docker/docker-agent/pull/1647) - Preserve 429 error details on OpenAI


## [v1.20.6] - 2026-02-07

This release introduces branching sessions, model fallbacks, and automated code quality scanning, along with performance improvements and enhanced file handling capabilities.

## What's New

- Adds branching sessions feature that allows editing previous messages to create new session branches without losing original conversation history
- Adds automated nightly codebase scanner with multi-agent architecture for detecting code quality issues and creating GitHub issues
- Adds model fallback system that automatically retries with alternative models when inference providers fail
- Adds skill invocation via slash commands for enhanced workflow automation
- Adds `--prompt-file` CLI flag for including file contents as system context
- Adds debug title command for troubleshooting session title generation

## Improvements

- Improves @ attachment performance to prevent UI hanging in large or deeply nested directories
- Switches to Anthropic Files API for file uploads instead of embedding content directly, dramatically reducing token usage
- Enhances scanner resilience and adds persistent memory system for learning from previous runs

## Bug Fixes

- Fixes tool calls score rendering in evaluations
- Fixes title generation for OpenAI and Gemini models
- Fixes GitHub Actions directory creation issues

## Technical Changes

- Refactors to use cagent's built-in memory system and text format for sub-agent output
- Enables additional golangci-lint linters and fixes code quality issues
- Simplifies PR review workflow by adopting reusable workflow from cagent-action
- Updates Model Context Protocol SDK and other dependencies

### Pull Requests

- [#1573](https://github.com/docker/docker-agent/pull/1573) - Automated nightly codebase scanner
- [#1578](https://github.com/docker/docker-agent/pull/1578) - Branching sessions on message edit
- [#1589](https://github.com/docker/docker-agent/pull/1589) - Model fallbacks
- [#1595](https://github.com/docker/docker-agent/pull/1595) - Simplifies PR review workflow by adopting the new reusable workflow from cagent-action
- [#1610](https://github.com/docker/docker-agent/pull/1610) - docs: update CHANGELOG.md for v1.20.5
- [#1611](https://github.com/docker/docker-agent/pull/1611) - Improve @ attachments perf 
- [#1612](https://github.com/docker/docker-agent/pull/1612) - Only create a new modelstore if none is given
- [#1613](https://github.com/docker/docker-agent/pull/1613) - [evals] Fix tool calls score rendering
- [#1614](https://github.com/docker/docker-agent/pull/1614) - Added space between release links
- [#1617](https://github.com/docker/docker-agent/pull/1617) - Opus 4.6
- [#1618](https://github.com/docker/docker-agent/pull/1618) - feat: add --prompt-file CLI flag for including file contents as system context
- [#1619](https://github.com/docker/docker-agent/pull/1619) - Update Nightly Scan Workflow
- [#1620](https://github.com/docker/docker-agent/pull/1620) - /attach use file upload instead of embedding in the context
- [#1621](https://github.com/docker/docker-agent/pull/1621) - Update Go deps
- [#1622](https://github.com/docker/docker-agent/pull/1622) - Add debug title command for session title generation
- [#1623](https://github.com/docker/docker-agent/pull/1623) - Add skill invocation via slash commands 
- [#1624](https://github.com/docker/docker-agent/pull/1624) - Fix schema and add drift test
- [#1627](https://github.com/docker/docker-agent/pull/1627) - Enable more linters and fix existing issues


## [v1.20.5] - 2026-02-05

This release improves stability for non-interactive sessions, updates the default Anthropic model to Claude Sonnet 4.5, and adds support for private GitHub repositories and standard agent directories.

## What's New

- Adds support for using agent YAML files from private GitHub repositories
- Adds support for standard `.agents/skills` directory structure
- Adds deepwiki integration to the librarian
- Adds timestamp tracking to runtime events
- Allows users to define their own default model in global configuration

## Improvements

- Updates default Anthropic model to Claude Sonnet 4.5
- Adds reason explanations when relevance checks fail during evaluations
- Persists ACP sessions to default SQLite database unless specified with `--session-db` flag
- Makes aliased agent paths absolute for better path resolution
- Produces session database for evaluations to enable investigation of results

## Bug Fixes

- Prevents panic when elicitation is requested in non-interactive sessions
- Fixes title generation hanging with Gemini 3 models by properly disabling thinking
- Fixes current agent display in TUI interface
- Prevents TUI dimensions from going negative when sidebar is collapsed
- Fixes flaky test issues

## Technical Changes

- Simplifies ElicitationRequestEvent check to reduce code duplication
- Allows passing additional environment variables to Docker when running evaluations
- Passes LLM as judge on full transcript for better evaluation accuracy


## [v1.20.4] - 2026-02-03

This release improves session handling with relative references and tool permissions, along with better table rendering in the TUI.

## What's New
- Adds support for relative session references in --session flag (e.g., `-1` for last session, `-2` for second to last)
- Adds "always allow this tool" option to permanently approve specific tools or commands for the session
- Adds granular permission patterns for shell commands that auto-approve specific commands while requiring confirmation for others

## Improvements
- Updates shell command selection to work with the new tool permission system
- Wraps tables properly in the TUI's experimental renderer to fit terminal width with smart column sizing

## Bug Fixes
- Fixes reading of legacy sessions
- Fixes getting sub-session errors where session was not found

## Technical Changes
- Adds test databases for better testing coverage
- Automatically runs PR reviewer for Docker organization members
- Exposes new approve-tool confirmation type via HTTP and ConnectRPC APIs


## [v1.20.3] - 2026-02-02

This release migrates PR review workflows to packaged actions and includes visual improvements to the Nord theme.

## Improvements
- Migrates PR review to packaged cagent-action sub-actions, reducing workflow complexity
- Changes code fences to blue color in Nord theme for better visual consistency

## Technical Changes
- Adds task rebuild when themes change to ensure proper theme updates
- Removes local development configuration that was accidentally committed


## [v1.20.2] - 2026-02-02

This release improves the tools system architecture and enhances TUI scrolling performance.

## Improvements
- Improves render and mouse scroll performance in the TUI interface

## Technical Changes
- Adds StartableToolSet and As[T] generic helper to tools package
- Adds capability interfaces for optional toolset features
- Adds ConfigureHandlers convenience function for tools
- Migrates StartableToolSet to tools package and cleans up ToolSet interface
- Removes BaseToolSet and DescriptionToolSet wrapper
- Reorganizes tool-related code structure


## [v1.20.1] - 2026-02-02

This release includes UI improvements, better error handling, and internal code organization enhancements.

## Improvements

- Changes audio listening shortcut from ctrl-k to ctrl-l (ctrl-k is now reserved for line editing)
- Improves title editing by allowing double-click anywhere on the title instead of requiring precise icon clicks
- Keeps footer unchanged when using /session or /new commands unless something actually changes
- Shows better error messages when using "auto" model with no available providers or when dmr is not available

## Bug Fixes

- Fixes flaky test that was causing CI failures
- Fixes `cagent new` command functionality
- Fixes title edit hitbox issues when title wraps to multiple lines

## Technical Changes

- Organizes TUI messages by domain concern
- Introduces SessionStateReader interface for read-only access
- Introduces Subscription type for cleaner animation lifecycle management
- Improves tool registry API with declarative RegisterAll method
- Introduces HitTest for centralized mouse target detection in chat
- Makes sidebar View() function pure by moving SetWidth to SetSize
- Introduces cmdbatch package for fluent command batching
- Organizes chat runtime event handlers by category
- Introduces subscription package for external event sources
- Separates CollapsedViewModel from rendering in sidebar
- Improves provider handling and error messaging


## [v1.20.0] - 2026-01-30

This release introduces editable session titles, custom TUI themes, and improved evaluation capabilities, along with database improvements and bug fixes.

## What's New
- Adds editable session titles with `/title` command and TUI support for renaming sessions
- Adds custom TUI theme support with built-in themes and hot-reloading capabilities
- Adds permissions view dialog for better visibility into agent permissions
- Adds concurrent LLM-as-a-judge relevance checks for faster evaluations
- Adds image cache to cagent eval for improved performance

## Improvements
- Makes slash commands searchable in the command palette
- Improves command palette with scrolling, mouse support, and dynamic resizing
- Adds validation error display in elicitation dialogs when Enter is pressed
- Adds Ctrl+z support for suspending TUI application to background
- Adds `--exit-on-stdin-eof` flag for better integration control
- Adds `--keep-containers` flag to cagent eval for debugging

## Bug Fixes
- Fixes auto-heal corrupted OCI local store by forcing re-pull when corruption is detected
- Fixes input token counting with Gemini models
- Fixes space key not working in elicitation text input fields
- Fixes session compaction issues
- Fixes stdin EOF checking to prevent cagent api from terminating unexpectedly in containers

## Technical Changes
- Extracts messages from sessions table into normalized session_items table
- Adds database backup and recovery on migration failure
- Maintains backward/forward compatibility for session data
- Removes ESC key from main status bar (now shown in spinner)
- Removes progress bar from cagent eval logs
- Sends mouse events to dialogs only when open


## [v1.19.7] - 2026-01-26

This release improves the user experience with better error handling and enhanced output formatting.

## Improvements
- Improves error handling and user feedback throughout the application
- Enhances output formatting for better readability and user experience

## Technical Changes
- Updates internal dependencies and build configurations
- Refactors code structure for improved maintainability
- Updates development and testing infrastructure


## [v1.19.6] - 2026-01-26

This release improves the user experience with better error handling and enhanced output formatting.

## Improvements
- Improves error handling and user feedback throughout the application
- Enhances output formatting for better readability and user experience

## Technical Changes
- Updates internal dependencies and build configurations
- Refactors code structure for better maintainability
- Updates development and testing infrastructure


## [v1.19.5] - 2026-01-22

This release improves the terminal user interface with better error handling and visual feedback, along with concurrency fixes and enhanced Docker authentication options.

## What's New

- Adds external command support for providing Docker access tokens
- Adds MCP Toolkit example for better integration guidance
- Adds realistic benchmark for markdown rendering performance testing

## Improvements

- Improves edit_file tool error rendering with consistent styling and single-line display
- Improves PR reviewer agent with Go-specific patterns and feedback learning capabilities
- Enhances collapsed reasoning blocks with fade-out animation for completed tool calls
- Makes dialog value changes clearer by indicating space key usage
- Adds dedicated pending response spinner with improved rendering performance

## Bug Fixes

- Fixes edit_file tool to skip diff rendering when tool execution fails
- Fixes concurrent access issues in user configuration aliases map
- Fixes style restoration after inline code blocks in markdown text
- Fixes model defaults when using the "router" provider to prevent erroneous thinking mode
- Fixes paste events incorrectly going to editor when dialog is open
- Fixes cassette recording functionality

## Technical Changes

- Adds clarifying comments for configuration and data directory paths
- Hides tools configuration interface
- Protects aliases map with mutex for thread safety


[v1.19.5]: https://github.com/docker/docker-agent/releases/tag/v1.19.5

[v1.19.6]: https://github.com/docker/docker-agent/releases/tag/v1.19.6

[v1.19.7]: https://github.com/docker/docker-agent/releases/tag/v1.19.7

[v1.20.0]: https://github.com/docker/docker-agent/releases/tag/v1.20.0

[v1.20.1]: https://github.com/docker/docker-agent/releases/tag/v1.20.1

[v1.20.2]: https://github.com/docker/docker-agent/releases/tag/v1.20.2

[v1.20.3]: https://github.com/docker/docker-agent/releases/tag/v1.20.3

[v1.20.4]: https://github.com/docker/docker-agent/releases/tag/v1.20.4

[v1.20.5]: https://github.com/docker/docker-agent/releases/tag/v1.20.5

[v1.20.6]: https://github.com/docker/docker-agent/releases/tag/v1.20.6

[v1.21.0]: https://github.com/docker/docker-agent/releases/tag/v1.21.0

[v1.22.0]: https://github.com/docker/docker-agent/releases/tag/v1.22.0

[v1.23.0]: https://github.com/docker/docker-agent/releases/tag/v1.23.0

[v1.23.1]: https://github.com/docker/docker-agent/releases/tag/v1.23.1

[v1.23.2]: https://github.com/docker/docker-agent/releases/tag/v1.23.2

[v1.23.3]: https://github.com/docker/docker-agent/releases/tag/v1.23.3

[v1.23.4]: https://github.com/docker/docker-agent/releases/tag/v1.23.4

[v1.23.5]: https://github.com/docker/docker-agent/releases/tag/v1.23.5

[v1.23.6]: https://github.com/docker/docker-agent/releases/tag/v1.23.6

[v1.24.0]: https://github.com/docker/docker-agent/releases/tag/v1.24.0

[v1.26.0]: https://github.com/docker/docker-agent/releases/tag/v1.26.0

[v1.27.0]: https://github.com/docker/docker-agent/releases/tag/v1.27.0

[v1.27.1]: https://github.com/docker/docker-agent/releases/tag/v1.27.1

[v1.28.0]: https://github.com/docker/docker-agent/releases/tag/v1.28.0

[v1.28.1]: https://github.com/docker/docker-agent/releases/tag/v1.28.1

[v1.29.0]: https://github.com/docker/docker-agent/releases/tag/v1.29.0

[v1.30.0]: https://github.com/docker/docker-agent/releases/tag/v1.30.0

[v1.30.1]: https://github.com/docker/docker-agent/releases/tag/v1.30.1

[v1.31.0]: https://github.com/docker/docker-agent/releases/tag/v1.31.0

[v1.32.0]: https://github.com/docker/docker-agent/releases/tag/v1.32.0

[v1.32.1]: https://github.com/docker/docker-agent/releases/tag/v1.32.1

[v1.32.2]: https://github.com/docker/docker-agent/releases/tag/v1.32.2

[v1.32.3]: https://github.com/docker/docker-agent/releases/tag/v1.32.3

[v1.32.4]: https://github.com/docker/docker-agent/releases/tag/v1.32.4

[v1.32.5]: https://github.com/docker/docker-agent/releases/tag/v1.32.5

[v1.33.0]: https://github.com/docker/docker-agent/releases/tag/v1.33.0

[v1.34.0]: https://github.com/docker/docker-agent/releases/tag/v1.34.0

[v1.36.0]: https://github.com/docker/docker-agent/releases/tag/v1.36.0

[v1.36.1]: https://github.com/docker/docker-agent/releases/tag/v1.36.1

[v1.37.0]: https://github.com/docker/docker-agent/releases/tag/v1.37.0

[v1.38.0]: https://github.com/docker/docker-agent/releases/tag/v1.38.0

[v1.39.0]: https://github.com/docker/docker-agent/releases/tag/v1.39.0

[v1.40.0]: https://github.com/docker/docker-agent/releases/tag/v1.40.0

[v1.41.0]: https://github.com/docker/docker-agent/releases/tag/v1.41.0

[v1.42.0]: https://github.com/docker/docker-agent/releases/tag/v1.42.0

[v1.43.0]: https://github.com/docker/docker-agent/releases/tag/v1.43.0

[v1.44.0]: https://github.com/docker/docker-agent/releases/tag/v1.44.0

[v1.45.0]: https://github.com/docker/docker-agent/releases/tag/v1.45.0

[v1.46.0]: https://github.com/docker/docker-agent/releases/tag/v1.46.0

[v1.47.0]: https://github.com/docker/docker-agent/releases/tag/v1.47.0

[v1.48.0]: https://github.com/docker/docker-agent/releases/tag/v1.48.0

[v1.49.0]: https://github.com/docker/docker-agent/releases/tag/v1.49.0

[v1.49.1]: https://github.com/docker/docker-agent/releases/tag/v1.49.1

[v1.49.2]: https://github.com/docker/docker-agent/releases/tag/v1.49.2

[v1.50.0]: https://github.com/docker/docker-agent/releases/tag/v1.50.0

[v1.51.0]: https://github.com/docker/docker-agent/releases/tag/v1.51.0
