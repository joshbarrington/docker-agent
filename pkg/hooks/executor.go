package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strings"
	"sync"
)

// Executor handles the execution of hooks.
//
// An Executor resolves each [Hook]'s [HookType] against a [Registry] of
// [HandlerFactory]s. The default executor uses [DefaultRegistry], which
// only knows about the built-in "command" handler; tests and embedders
// can supply their own registry via [NewExecutorWithRegistry] to add new
// handler kinds (in-process Go callbacks, HTTP webhooks, MCP tools, ...)
// without touching the executor itself.
//
// Public entry points:
//
//   - [Executor.Dispatch] runs the hooks registered for one [EventType]
//     and aggregates their verdicts into a single [Result].
//   - [Executor.Has] reports whether any hooks are configured for an
//     event, so callers can avoid building the [Input] when nothing
//     would run.
type Executor struct {
	config     *Config
	workingDir string
	env        []string

	// registry resolves a HookType to the factory used to build a Handler.
	registry *Registry

	// eventTable is the per-event compiled lookup that backs Dispatch and
	// Has. Tool-matched events (pre/post_tool_use) carry compiled regex
	// matchers in matchers; flat events carry their hook list directly.
	// Each event has at most one of the two populated.
	eventTable map[EventType]eventHooks
}

// eventHooks is the compiled set of hooks for a single event. Exactly one
// of flat / matchers is non-empty: flat for events with no per-tool
// matcher, matchers for pre/post_tool_use.
type eventHooks struct {
	flat     []Hook
	matchers []compiledMatcher
}

func (eh eventHooks) isEmpty() bool {
	return len(eh.flat) == 0 && len(eh.matchers) == 0
}

type compiledMatcher struct {
	config  MatcherConfig
	pattern *regexp.Regexp
}

// hookResult represents the result of executing a single hook
type hookResult struct {
	output   *Output
	stdout   string
	stderr   string
	exitCode int
	err      error
}

// NewExecutor creates a new hook executor backed by [DefaultRegistry].
func NewExecutor(config *Config, workingDir string, env []string) *Executor {
	return NewExecutorWithRegistry(config, workingDir, env, DefaultRegistry)
}

// NewExecutorWithRegistry creates a new hook executor that resolves hook
// types against the supplied registry.
//
// This is the seam used to extend the hook system: callers register
// additional [HandlerFactory]s for new [HookType] values on a private
// [Registry] and pass it here.
func NewExecutorWithRegistry(config *Config, workingDir string, env []string, registry *Registry) *Executor {
	if config == nil {
		config = &Config{}
	}
	if registry == nil {
		registry = DefaultRegistry
	}

	e := &Executor{
		config:     config,
		workingDir: workingDir,
		env:        env,
		registry:   registry,
	}
	e.compileEventTable()
	return e
}

// compileEventTable builds the per-event lookup that Dispatch and Has
// consult. Adding a new event is a one-line change here; no per-event
// methods or fields need to be added on the Executor.
func (e *Executor) compileEventTable() {
	e.eventTable = map[EventType]eventHooks{
		EventPreToolUse:      {matchers: e.compileMatcherList(e.config.PreToolUse)},
		EventPostToolUse:     {matchers: e.compileMatcherList(e.config.PostToolUse)},
		EventSessionStart:    {flat: e.config.SessionStart},
		EventTurnStart:       {flat: e.config.TurnStart},
		EventBeforeLLMCall:   {flat: e.config.BeforeLLMCall},
		EventAfterLLMCall:    {flat: e.config.AfterLLMCall},
		EventSessionEnd:      {flat: e.config.SessionEnd},
		EventOnUserInput:     {flat: e.config.OnUserInput},
		EventStop:            {flat: e.config.Stop},
		EventNotification:    {flat: e.config.Notification},
		EventOnError:         {flat: e.config.OnError},
		EventOnMaxIterations: {flat: e.config.OnMaxIterations},
	}
}

func (e *Executor) compileMatcherList(configs []MatcherConfig) []compiledMatcher {
	var result []compiledMatcher
	for _, mc := range configs {
		var pattern *regexp.Regexp
		if mc.Matcher != "" && mc.Matcher != "*" {
			// Compile as regex, case-sensitive
			p, err := regexp.Compile("^(?:" + mc.Matcher + ")$")
			if err != nil {
				slog.Warn("Invalid hook matcher pattern", "pattern", mc.Matcher, "error", err)
				continue
			}
			pattern = p
		}
		result = append(result, compiledMatcher{
			config:  mc,
			pattern: pattern,
		})
	}
	return result
}

// matchTool checks if a tool name matches the given pattern
func (cm *compiledMatcher) matchTool(toolName string) bool {
	// "*" or empty matcher matches all
	if cm.config.Matcher == "" || cm.config.Matcher == "*" {
		return true
	}
	if cm.pattern == nil {
		return false
	}
	return cm.pattern.MatchString(toolName)
}

// Has reports whether any hooks are configured for event. Callers can
// use this to skip building the [Input] when nothing would run.
func (e *Executor) Has(event EventType) bool {
	eh, ok := e.eventTable[event]
	return ok && !eh.isEmpty()
}

// Dispatch runs the hooks registered for event and aggregates their
// verdicts into a single [Result].
//
// For tool-matched events (pre/post_tool_use), only hooks whose matcher
// matches input.ToolName are run. For all other events the hook list is
// flat. When no hooks would run, a permissive Result{Allowed: true} is
// returned without serializing the input.
//
// Dispatch sets input.HookEventName to event so handlers can rely on it
// without the caller having to remember.
func (e *Executor) Dispatch(ctx context.Context, event EventType, input *Input) (*Result, error) {
	eh, ok := e.eventTable[event]
	if !ok || eh.isEmpty() {
		return &Result{Allowed: true}, nil
	}

	input.HookEventName = event

	hooksToRun := e.selectHooks(eh, input.ToolName)
	if len(hooksToRun) == 0 {
		return &Result{Allowed: true}, nil
	}

	return e.executeHooks(ctx, hooksToRun, input, event)
}

// selectHooks returns the subset of eh that should fire for the given
// tool name. Flat events ignore toolName; tool-matched events filter
// their matchers and concatenate the matching hook lists.
func (e *Executor) selectHooks(eh eventHooks, toolName string) []Hook {
	if len(eh.matchers) == 0 {
		return eh.flat
	}
	var hooks []Hook
	for _, cm := range eh.matchers {
		if cm.matchTool(toolName) {
			hooks = append(hooks, cm.config.Hooks...)
		}
	}
	return hooks
}

// executeHooks runs a list of hooks in parallel and aggregates results
func (e *Executor) executeHooks(ctx context.Context, hooks []Hook, input *Input, eventType EventType) (*Result, error) {
	// Deduplicate hooks by structural identity (type + command + args).
	// This catches the common case of an explicit YAML hook overlapping a
	// runtime auto-injected one (e.g. WithAddDate plus a user-authored
	// {type: builtin, command: add_date}); two add_prompt_files hooks
	// with different Args remain distinct and both fire.
	seen := make(map[string]bool)
	var uniqueHooks []Hook
	for _, h := range hooks {
		key := hookDedupKey(h)
		if !seen[key] {
			seen[key] = true
			uniqueHooks = append(uniqueHooks, h)
		}
	}

	if len(uniqueHooks) == 0 {
		return &Result{Allowed: true}, nil
	}

	// Serialize input to JSON
	inputJSON, err := input.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize hook input: %w", err)
	}

	// Execute hooks in parallel
	results := make([]hookResult, len(uniqueHooks))
	var wg sync.WaitGroup

	for i, hook := range uniqueHooks {
		wg.Go(func() {
			output, stdout, stderr, exitCode, err := e.executeHook(ctx, hook, inputJSON)
			results[i] = hookResult{
				output:   output,
				stdout:   stdout,
				stderr:   stderr,
				exitCode: exitCode,
				err:      err,
			}
		})
	}

	wg.Wait()

	// Aggregate results
	return e.aggregateResults(results, eventType)
}

// hookDedupKey produces a deterministic key for hook deduplication. Two
// hooks with the same type, command, and ordered args are considered
// equivalent.
func hookDedupKey(h Hook) string {
	var b strings.Builder
	b.WriteString(string(h.Type))
	b.WriteByte(0)
	b.WriteString(h.Command)
	for _, a := range h.Args {
		b.WriteByte(0)
		b.WriteString(a)
	}
	return b.String()
}

// executeHook runs a single hook by dispatching to the [Handler] registered
// for its [HookType]. The hook timeout is applied here (so every handler
// kind is bounded uniformly) and successful Stdout is parsed as JSON into
// an [Output] when the handler did not return a pre-parsed one.
func (e *Executor) executeHook(ctx context.Context, hook Hook, inputJSON []byte) (*Output, string, string, int, error) {
	factory, ok := e.registry.Lookup(hook.Type)
	if !ok {
		return nil, "", "", 0, fmt.Errorf("unsupported hook type: %s", hook.Type)
	}

	handler, err := factory(HandlerEnv{WorkingDir: e.workingDir, Env: e.env}, hook)
	if err != nil {
		return nil, "", "", 0, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, hook.GetTimeout())
	defer cancel()

	res, err := handler.Run(timeoutCtx, inputJSON)

	// A fired timeout or parent-context cancellation surfaces as a non-nil
	// error whose Go type varies across handler kinds. Normalize to a plain
	// execution error so PreToolUse gates can fail closed rather than look
	// at a meaningless exit code.
	if ctxErr := timeoutCtx.Err(); ctxErr != nil {
		reason := "cancelled"
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			reason = fmt.Sprintf("timed out after %s", hook.GetTimeout())
		}
		return nil, res.Stdout, res.Stderr, -1,
			fmt.Errorf("hook %q %s: %w", hook.Command, reason, ctxErr)
	}
	if err != nil {
		return nil, res.Stdout, res.Stderr, -1, err
	}

	// If the handler produced a pre-parsed Output, honor it as-is. Otherwise
	// fall back to the legacy "parse JSON from stdout" protocol that command
	// hooks rely on.
	output := res.Output
	if output == nil && res.ExitCode == 0 && res.Stdout != "" {
		stdoutStr := strings.TrimSpace(res.Stdout)
		if strings.HasPrefix(stdoutStr, "{") {
			var parsed Output
			if jerr := json.Unmarshal([]byte(stdoutStr), &parsed); jerr == nil {
				output = &parsed
			}
		}
	}

	return output, res.Stdout, res.Stderr, res.ExitCode, nil
}

// aggregateResults combines results from multiple hooks
func (e *Executor) aggregateResults(results []hookResult, eventType EventType) (*Result, error) {
	finalResult := &Result{
		Allowed: true,
	}

	var messages []string
	var additionalContexts []string
	var systemMessages []string

	for _, r := range results {
		if r.err != nil {
			// PreToolUse is a security boundary: if a hook fails to
			// produce a verdict (timeout, spawn failure, missing binary),
			// deny the tool call rather than silently letting it through.
			if eventType == EventPreToolUse {
				slog.Warn("PreToolUse hook failed to execute; denying tool call", "error", r.err)
				finalResult.Allowed = false
				finalResult.ExitCode = -1
				finalResult.Stderr = r.stderr
				messages = append(messages, fmt.Sprintf("PreToolUse hook failed to execute: %v", r.err))
			} else {
				slog.Warn("Hook execution error", "error", r.err)
			}
			continue
		}

		// Exit code 2 is a blocking error
		if r.exitCode == 2 {
			finalResult.Allowed = false
			finalResult.ExitCode = 2
			if r.stderr != "" {
				finalResult.Stderr = r.stderr
				messages = append(messages, strings.TrimSpace(r.stderr))
			}
			continue
		}

		// Non-zero, non-2 exit codes are non-blocking errors
		if r.exitCode != 0 {
			slog.Debug("Hook returned non-zero exit code", "exit_code", r.exitCode, "stderr", r.stderr)
			continue
		}

		// Process successful output
		if r.output != nil {
			// Check continue flag
			if !r.output.ShouldContinue() {
				finalResult.Allowed = false
				if r.output.StopReason != "" {
					messages = append(messages, r.output.StopReason)
				}
			}

			// Check decision
			if r.output.IsBlocked() {
				finalResult.Allowed = false
				if r.output.Reason != "" {
					messages = append(messages, r.output.Reason)
				}
			}

			// Collect system messages
			if r.output.SystemMessage != "" {
				systemMessages = append(systemMessages, r.output.SystemMessage)
			}

			// Process hook-specific output
			if r.output.HookSpecificOutput != nil {
				hso := r.output.HookSpecificOutput

				// PreToolUse permission decision
				if eventType == EventPreToolUse {
					switch hso.PermissionDecision {
					case DecisionDeny:
						finalResult.Allowed = false
						if hso.PermissionDecisionReason != "" {
							messages = append(messages, hso.PermissionDecisionReason)
						}
					case DecisionAsk:
						// Ask leaves it up to the normal approval flow
						// Don't change Allowed
					}

					// Merge updated input
					if hso.UpdatedInput != nil {
						if finalResult.ModifiedInput == nil {
							finalResult.ModifiedInput = make(map[string]any)
						}
						maps.Copy(finalResult.ModifiedInput, hso.UpdatedInput)
					}
				}

				// Additional context
				if hso.AdditionalContext != "" {
					additionalContexts = append(additionalContexts, hso.AdditionalContext)
				}
			}
		} else if r.stdout != "" {
			// Plain text stdout is added as context for events whose runtime
			// emit site reads Result.AdditionalContext: session_start (and
			// turn_start, which has the same context-injection semantics),
			// post_tool_use, and stop. Observational events (before/after
			// LLM call, on_error, on_max_iterations, notification) do not
			// surface AdditionalContext to the model, so plain stdout there
			// would silently disappear — those hooks should structure their
			// output as JSON if they want to communicate with the executor.
			switch eventType {
			case EventSessionStart, EventTurnStart, EventPostToolUse, EventStop:
				additionalContexts = append(additionalContexts, strings.TrimSpace(r.stdout))
			}
		}
	}

	// Combine messages
	if len(messages) > 0 {
		finalResult.Message = strings.Join(messages, "\n")
	}
	if len(additionalContexts) > 0 {
		finalResult.AdditionalContext = strings.Join(additionalContexts, "\n")
	}
	if len(systemMessages) > 0 {
		finalResult.SystemMessage = strings.Join(systemMessages, "\n")
	}

	return finalResult, nil
}
