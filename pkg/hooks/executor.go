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

// Executor dispatches configured hooks. Hook types are resolved against
// a [Registry] of [HandlerFactory]s; embedders can register new kinds
// (in-process Go callbacks, HTTP webhooks, ...) without touching the
// executor itself.
type Executor struct {
	workingDir string
	env        []string
	registry   *Registry
	// events maps each event to its compiled matcher list. Flat events
	// (everything except pre/post_tool_use) are stored as a single
	// matcher with an empty pattern, unifying the dispatch path.
	events map[EventType][]matcher
}

// matcher is the compiled form of a [MatcherConfig]: an optional regex
// pattern (nil means "match all") and the hooks to fire when it matches.
type matcher struct {
	raw     string
	pattern *regexp.Regexp
	hooks   []Hook
}

func (m *matcher) matches(toolName string) bool {
	if m.raw == "" || m.raw == "*" {
		return true
	}
	return m.pattern != nil && m.pattern.MatchString(toolName)
}

// hookResult is the outcome of a single hook invocation.
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
func NewExecutorWithRegistry(config *Config, workingDir string, env []string, registry *Registry) *Executor {
	if config == nil {
		config = &Config{}
	}
	if registry == nil {
		registry = DefaultRegistry
	}
	return &Executor{
		workingDir: workingDir,
		env:        env,
		registry:   registry,
		events:     compileEvents(config),
	}
}

// compileEvents builds the per-event matcher lookup. Adding a new event
// is a one-line change here.
func compileEvents(c *Config) map[EventType][]matcher {
	flat := func(hooks []Hook) []matcher {
		if len(hooks) == 0 {
			return nil
		}
		return []matcher{{hooks: hooks}}
	}
	return map[EventType][]matcher{
		EventPreToolUse:      compileMatchers(c.PreToolUse),
		EventPostToolUse:     compileMatchers(c.PostToolUse),
		EventSessionStart:    flat(c.SessionStart),
		EventTurnStart:       flat(c.TurnStart),
		EventBeforeLLMCall:   flat(c.BeforeLLMCall),
		EventAfterLLMCall:    flat(c.AfterLLMCall),
		EventSessionEnd:      flat(c.SessionEnd),
		EventOnUserInput:     flat(c.OnUserInput),
		EventStop:            flat(c.Stop),
		EventNotification:    flat(c.Notification),
		EventOnError:         flat(c.OnError),
		EventOnMaxIterations: flat(c.OnMaxIterations),
	}
}

func compileMatchers(configs []MatcherConfig) []matcher {
	if len(configs) == 0 {
		return nil
	}
	out := make([]matcher, 0, len(configs))
	for _, mc := range configs {
		m := matcher{raw: mc.Matcher, hooks: mc.Hooks}
		if mc.Matcher != "" && mc.Matcher != "*" {
			p, err := regexp.Compile("^(?:" + mc.Matcher + ")$")
			if err != nil {
				slog.Warn("Invalid hook matcher pattern", "pattern", mc.Matcher, "error", err)
				continue
			}
			m.pattern = p
		}
		out = append(out, m)
	}
	return out
}

// Has reports whether any hooks are configured for event.
func (e *Executor) Has(event EventType) bool {
	return len(e.events[event]) > 0
}

// Dispatch runs the hooks registered for event and aggregates their
// verdicts into a single [Result]. Sets input.HookEventName so handlers
// don't have to remember. Defaults [Input.Cwd] to the executor's
// working directory when the caller didn't supply one.
func (e *Executor) Dispatch(ctx context.Context, event EventType, input *Input) (*Result, error) {
	matchers := e.events[event]
	if len(matchers) == 0 {
		return &Result{Allowed: true}, nil
	}
	input.HookEventName = event
	if input.Cwd == "" {
		input.Cwd = e.workingDir
	}

	// Collect, filter by tool name, and dedup by (type, command, args).
	// Dedup catches the common case of an explicit YAML hook overlapping
	// a runtime auto-injected one (e.g. WithAddDate plus a user-authored
	// add_date entry).
	seen := make(map[string]bool)
	var hooks []Hook
	for _, m := range matchers {
		if !m.matches(input.ToolName) {
			continue
		}
		for _, h := range m.hooks {
			key := dedupKey(h)
			if !seen[key] {
				seen[key] = true
				hooks = append(hooks, h)
			}
		}
	}
	if len(hooks) == 0 {
		return &Result{Allowed: true}, nil
	}

	inputJSON, err := input.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize hook input: %w", err)
	}

	results := make([]hookResult, len(hooks))
	var wg sync.WaitGroup
	for i, hook := range hooks {
		wg.Go(func() { results[i] = e.runHook(ctx, hook, inputJSON) })
	}
	wg.Wait()

	return aggregate(results, event), nil
}

// dedupKey returns a deterministic key identifying a hook by (type, command, args).
func dedupKey(h Hook) string {
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

// runHook resolves the hook's [HookType] in the registry, applies its
// timeout, and returns the structured outcome. JSON-on-stdout is parsed
// into [Output] when the handler didn't already provide one.
func (e *Executor) runHook(ctx context.Context, hook Hook, inputJSON []byte) hookResult {
	factory, ok := e.registry.Lookup(hook.Type)
	if !ok {
		return hookResult{err: fmt.Errorf("unsupported hook type: %s", hook.Type)}
	}
	handler, err := factory(HandlerEnv{WorkingDir: e.workingDir, Env: e.env}, hook)
	if err != nil {
		return hookResult{err: err}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, hook.GetTimeout())
	defer cancel()

	res, err := handler.Run(timeoutCtx, inputJSON)
	r := hookResult{stdout: res.Stdout, stderr: res.Stderr, exitCode: res.ExitCode, output: res.Output}

	// Normalize timeout/cancellation: handler error types vary, so we
	// rewrite to a uniform error so PreToolUse fails closed cleanly.
	if ctxErr := timeoutCtx.Err(); ctxErr != nil {
		reason := "cancelled"
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			reason = fmt.Sprintf("timed out after %s", hook.GetTimeout())
		}
		r.err = fmt.Errorf("hook %q %s: %w", hook.Command, reason, ctxErr)
		r.exitCode = -1
		r.output = nil
		return r
	}
	if err != nil {
		r.err = err
		r.exitCode = -1
		r.output = nil
		return r
	}

	// Fall back to the legacy "parse JSON from stdout" protocol.
	if r.output == nil && r.exitCode == 0 && r.stdout != "" {
		s := strings.TrimSpace(r.stdout)
		if strings.HasPrefix(s, "{") {
			var parsed Output
			if jerr := json.Unmarshal([]byte(s), &parsed); jerr == nil {
				r.output = &parsed
			}
		}
	}
	return r
}

// contextEvents are the events whose runtime emit sites consume
// Result.AdditionalContext. Plain stdout from a hook is routed there
// for these events; for observational events it is silently dropped to
// avoid the impression that it mattered.
var contextEvents = map[EventType]bool{
	EventSessionStart: true,
	EventTurnStart:    true,
	EventPostToolUse:  true,
	EventStop:         true,
}

// aggregate combines per-hook results into a single [Result].
func aggregate(results []hookResult, event EventType) *Result {
	final := &Result{Allowed: true}
	var messages, contexts, sysMsgs []string

	for _, r := range results {
		switch {
		case r.err != nil:
			// PreToolUse is a security boundary: an exec failure denies.
			if event == EventPreToolUse {
				slog.Warn("PreToolUse hook failed to execute; denying tool call", "error", r.err)
				final.Allowed = false
				final.ExitCode = -1
				final.Stderr = r.stderr
				messages = append(messages, fmt.Sprintf("PreToolUse hook failed to execute: %v", r.err))
			} else {
				slog.Warn("Hook execution error", "error", r.err)
			}
			continue

		case r.exitCode == 2:
			final.Allowed = false
			final.ExitCode = 2
			if r.stderr != "" {
				final.Stderr = r.stderr
				messages = append(messages, strings.TrimSpace(r.stderr))
			}
			continue

		case r.exitCode != 0:
			slog.Debug("Hook returned non-zero exit code", "exit_code", r.exitCode, "stderr", r.stderr)
			continue

		case r.output == nil:
			// Plain stdout becomes AdditionalContext only for events
			// whose runtime consumes it.
			if r.stdout != "" && contextEvents[event] {
				contexts = append(contexts, strings.TrimSpace(r.stdout))
			}
			continue
		}

		out := r.output
		if !out.ShouldContinue() {
			final.Allowed = false
			if out.StopReason != "" {
				messages = append(messages, out.StopReason)
			}
		}
		if out.IsBlocked() {
			final.Allowed = false
			if out.Reason != "" {
				messages = append(messages, out.Reason)
			}
		}
		if out.SystemMessage != "" {
			sysMsgs = append(sysMsgs, out.SystemMessage)
		}
		if hso := out.HookSpecificOutput; hso != nil {
			if event == EventPreToolUse {
				if hso.PermissionDecision == DecisionDeny {
					final.Allowed = false
					if hso.PermissionDecisionReason != "" {
						messages = append(messages, hso.PermissionDecisionReason)
					}
				}
				if hso.UpdatedInput != nil {
					if final.ModifiedInput == nil {
						final.ModifiedInput = make(map[string]any)
					}
					maps.Copy(final.ModifiedInput, hso.UpdatedInput)
				}
			}
			if hso.AdditionalContext != "" {
				contexts = append(contexts, hso.AdditionalContext)
			}
		}
	}

	final.Message = strings.Join(messages, "\n")
	final.AdditionalContext = strings.Join(contexts, "\n")
	final.SystemMessage = strings.Join(sysMsgs, "\n")
	return final
}
