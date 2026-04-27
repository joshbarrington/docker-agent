package runtime

import (
	"context"
	"log/slog"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/hooks/builtins"
	"github.com/docker/docker-agent/pkg/session"
)

// hooksExec returns the cached [hooks.Executor] for a, building one on
// first lookup. Returns nil when the agent has no user-configured hooks
// and no agent-flag (AddDate / AddEnvironmentInfo / AddPromptFiles) maps
// to a builtin. Callers can then short-circuit without paying for a
// no-op dispatch.
//
// The cache is keyed by agent name. Entries (including the nil sentinel)
// are stable for the lifetime of the runtime, so repeated dispatches
// during a turn don't re-translate agent flags into builtin hook entries
// or rebuild matcher tables.
func (r *LocalRuntime) hooksExec(a *agent.Agent) *hooks.Executor {
	if a == nil {
		return nil
	}
	name := a.Name()

	r.hooksExecMu.RLock()
	if exec, ok := r.hooksExecByAgent[name]; ok {
		r.hooksExecMu.RUnlock()
		return exec
	}
	r.hooksExecMu.RUnlock()

	r.hooksExecMu.Lock()
	defer r.hooksExecMu.Unlock()
	// Re-check under the write lock to avoid double-build under contention.
	if exec, ok := r.hooksExecByAgent[name]; ok {
		return exec
	}

	cfg := builtins.ApplyAgentDefaults(hooks.FromConfig(a.Hooks()), builtins.AgentDefaults{
		AddDate:            a.AddDate(),
		AddEnvironmentInfo: a.AddEnvironmentInfo(),
		AddPromptFiles:     a.AddPromptFiles(),
	})

	var exec *hooks.Executor
	if cfg != nil {
		exec = hooks.NewExecutorWithRegistry(cfg, r.workingDir, r.env, r.hooksRegistry)
	}
	if r.hooksExecByAgent == nil {
		r.hooksExecByAgent = make(map[string]*hooks.Executor)
	}
	r.hooksExecByAgent[name] = exec
	return exec
}

// dispatchHook is the common dispatch path shared by every hook
// callsite: resolve the cached executor, short-circuit if no hook is
// configured for event, then dispatch and emit any [Result.SystemMessage]
// as a Warning event. Errors are logged at warn level and surfaced as
// nil results so callers can use a single nil check to mean "nothing
// useful came back" — covering the not-configured, no-agent, and
// dispatch-failed cases uniformly.
//
// events may be nil for fire-and-forget callsites (notification,
// on_error, on_max_iterations, ...) where there's no Warning channel
// to emit on; the SystemMessage is then dropped by design rather than
// silently logged, since those events are themselves the user-facing
// notification mechanism.
func (r *LocalRuntime) dispatchHook(
	ctx context.Context,
	a *agent.Agent,
	event hooks.EventType,
	input *hooks.Input,
	events chan Event,
) *hooks.Result {
	exec := r.hooksExec(a)
	if exec == nil || !exec.Has(event) {
		return nil
	}

	slog.Debug("Executing hooks", "event", event, "agent", a.Name(), "session_id", input.SessionID)
	result, err := exec.Dispatch(ctx, event, input)
	if err != nil {
		slog.Warn("Hook execution failed", "event", event, "agent", a.Name(), "error", err)
		return nil
	}

	if events != nil && result.SystemMessage != "" {
		events <- Warning(result.SystemMessage, a.Name())
	}
	return result
}

// executeSessionStartHooks executes session_start hooks and persists any
// AdditionalContext as a system message on the session. SystemMessage,
// if any, is emitted as a Warning by [dispatchHook].
func (r *LocalRuntime) executeSessionStartHooks(ctx context.Context, sess *session.Session, a *agent.Agent, events chan Event) {
	result := r.dispatchHook(ctx, a, hooks.EventSessionStart, &hooks.Input{
		SessionID: sess.ID,
		Source:    "startup",
	}, events)
	if result == nil || result.AdditionalContext == "" {
		return
	}
	slog.Debug("Session start hook provided additional context", "context", result.AdditionalContext)
	sess.AddMessage(session.SystemMessage(result.AdditionalContext))
}

// executeTurnStartHooks runs turn_start hooks and returns ephemeral
// system messages to inject into the model call's messages slice.
//
// Unlike session_start, the AdditionalContext from turn_start is NOT
// persisted to the session — it's recomputed every turn. This is the
// right semantics for fast-changing context like "Today's date" or the
// contents of a prompt file the user might be editing during the session.
func (r *LocalRuntime) executeTurnStartHooks(ctx context.Context, sess *session.Session, a *agent.Agent, events chan Event) []chat.Message {
	result := r.dispatchHook(ctx, a, hooks.EventTurnStart, &hooks.Input{
		SessionID: sess.ID,
	}, events)
	if result == nil || result.AdditionalContext == "" {
		return nil
	}
	return []chat.Message{{
		Role:    chat.MessageRoleSystem,
		Content: result.AdditionalContext,
	}}
}

// executeSessionEndHooks fires session_end when the run loop exits
// (stream closed, context done, ...).
func (r *LocalRuntime) executeSessionEndHooks(ctx context.Context, sess *session.Session, a *agent.Agent) {
	r.dispatchHook(ctx, a, hooks.EventSessionEnd, &hooks.Input{
		SessionID: sess.ID,
		Reason:    "stream_ended",
	}, nil)
}

// executeStopHooks fires stop hooks when the model finishes responding,
// passing the final response content as stop_response. SystemMessage is
// surfaced as a Warning by [dispatchHook].
func (r *LocalRuntime) executeStopHooks(ctx context.Context, sess *session.Session, a *agent.Agent, responseContent string, events chan Event) {
	r.dispatchHook(ctx, a, hooks.EventStop, &hooks.Input{
		SessionID:    sess.ID,
		StopResponse: responseContent,
	}, events)
}

// executeNotificationHooks runs notification hooks when the agent emits
// a user-facing notification. Hook output is informational — it does
// not suppress or rewrite the notification.
func (r *LocalRuntime) executeNotificationHooks(ctx context.Context, a *agent.Agent, sessionID, level, message string) {
	if level != "error" && level != "warning" {
		slog.Error("Invalid notification level", "level", level, "expected", "error|warning")
		return
	}
	r.dispatchHook(ctx, a, hooks.EventNotification, &hooks.Input{
		SessionID:           sessionID,
		NotificationLevel:   level,
		NotificationMessage: message,
	}, nil)
}

// executeOnErrorHooks fires on_error when the runtime hits an error
// during a turn (model failures, tool-call loops). Fires alongside the
// broader notification event; on_error is the structured entry point
// for users who want to react only to errors.
func (r *LocalRuntime) executeOnErrorHooks(ctx context.Context, a *agent.Agent, sessionID, message string) {
	r.dispatchHook(ctx, a, hooks.EventOnError, &hooks.Input{
		SessionID:           sessionID,
		NotificationLevel:   "error",
		NotificationMessage: message,
	}, nil)
}

// executeOnMaxIterationsHooks fires on_max_iterations when the runtime
// reaches its configured max_iterations limit. Fires alongside the
// broader notification event; on_max_iterations is the structured entry
// point for users who want to react only to that condition.
func (r *LocalRuntime) executeOnMaxIterationsHooks(ctx context.Context, a *agent.Agent, sessionID, message string) {
	r.dispatchHook(ctx, a, hooks.EventOnMaxIterations, &hooks.Input{
		SessionID:           sessionID,
		NotificationLevel:   "warning",
		NotificationMessage: message,
	}, nil)
}

// executeBeforeLLMCallHooks fires before_llm_call just before each
// model call. The output is informational (not honored as a deny
// verdict yet), making this the right event for cost guardrails,
// auditing, and observability. Hooks that want to contribute system
// messages should use turn_start instead.
func (r *LocalRuntime) executeBeforeLLMCallHooks(ctx context.Context, sess *session.Session, a *agent.Agent) {
	r.dispatchHook(ctx, a, hooks.EventBeforeLLMCall, &hooks.Input{
		SessionID: sess.ID,
	}, nil)
}

// executeAfterLLMCallHooks fires after_llm_call after a successful
// model call, before the response is recorded into the session and
// tool calls are dispatched. The assistant text content is passed via
// stop_response (matching the stop event), so handlers can reuse the
// same parsing logic. Failed model calls fire on_error instead and
// skip this event.
func (r *LocalRuntime) executeAfterLLMCallHooks(ctx context.Context, sess *session.Session, a *agent.Agent, responseContent string) {
	r.dispatchHook(ctx, a, hooks.EventAfterLLMCall, &hooks.Input{
		SessionID:    sess.ID,
		StopResponse: responseContent,
	}, nil)
}

// executeOnUserInputHooks fires on_user_input when the runtime is about
// to wait for the user (tool confirmation, elicitation, max iterations,
// stream stopped). Resolves the agent from r.team itself so callsites
// in code paths without an agent handle (like the elicitation handler)
// stay short.
func (r *LocalRuntime) executeOnUserInputHooks(ctx context.Context, sessionID, logContext string) {
	a, _ := r.team.Agent(r.CurrentAgentName())
	if a == nil {
		return
	}
	slog.Debug("Executing on-user-input hooks", "context", logContext)
	r.dispatchHook(ctx, a, hooks.EventOnUserInput, &hooks.Input{
		SessionID: sessionID,
	}, nil)
}
