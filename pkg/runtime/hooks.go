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

// buildHooksExecutors builds a [hooks.Executor] for every agent in the
// team that has user-configured hooks or an agent-flag that maps to a
// builtin (AddDate / AddEnvironmentInfo / AddPromptFiles). Agents with
// no hooks have no entry; lookups fall through to nil so callers can
// short-circuit cheaply.
//
// Called once from [NewLocalRuntime] after r.workingDir, r.env and
// r.hooksRegistry are finalized; the resulting map is read-only for
// the lifetime of the runtime, so per-dispatch lookups don't need to
// lock.
func (r *LocalRuntime) buildHooksExecutors() {
	r.hooksExecByAgent = make(map[string]*hooks.Executor)
	for _, name := range r.team.AgentNames() {
		a, err := r.team.Agent(name)
		if err != nil {
			continue
		}
		cfg := builtins.ApplyAgentDefaults(a.Hooks(), builtins.AgentDefaults{
			AddDate:            a.AddDate(),
			AddEnvironmentInfo: a.AddEnvironmentInfo(),
			AddPromptFiles:     a.AddPromptFiles(),
		})
		if cfg == nil {
			continue
		}
		r.hooksExecByAgent[name] = hooks.NewExecutorWithRegistry(cfg, r.workingDir, r.env, r.hooksRegistry)
	}
}

// hooksExec returns the pre-built [hooks.Executor] for a, or nil when
// the agent has no hooks (see [buildHooksExecutors]).
func (r *LocalRuntime) hooksExec(a *agent.Agent) *hooks.Executor {
	if a == nil {
		return nil
	}
	return r.hooksExecByAgent[a.Name()]
}

// dispatchHook is the common dispatch path shared by every hook
// callsite: resolve the pre-built executor, dispatch, and emit any
// [Result.SystemMessage] as a Warning event. Errors are logged at warn
// level and surfaced as nil results so callers can use a single nil
// check to mean "nothing useful came back" — covering the
// not-configured, no-agent, and dispatch-failed cases uniformly.
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
	if exec == nil {
		return nil
	}

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

// executeSessionStartHooks fires session_start once at the top of
// RunStream and returns its AdditionalContext as transient system
// messages. The result is NOT persisted to the session: persisting
// would pollute the visible transcript and (because session_start
// fires after the user message has been added) shift the message the
// runtime relays as the [UserMessageEvent]. Callers thread the
// returned slice through [session.Session.GetMessages] on every
// iteration so cwd / OS / arch context reaches the model without ever
// being stored.
func (r *LocalRuntime) executeSessionStartHooks(ctx context.Context, sess *session.Session, a *agent.Agent, events chan Event) []chat.Message {
	return contextMessages(r.dispatchHook(ctx, a, hooks.EventSessionStart, &hooks.Input{
		SessionID: sess.ID,
		Source:    "startup",
	}, events))
}

// executeTurnStartHooks fires turn_start before each model call and
// returns its AdditionalContext as transient system messages. Like
// session_start the result is never persisted, but turn_start runs
// every iteration so its content is recomputed each turn — the right
// semantics for fast-changing context like the current date or the
// contents of a prompt file the user might be editing mid-session.
func (r *LocalRuntime) executeTurnStartHooks(ctx context.Context, sess *session.Session, a *agent.Agent, events chan Event) []chat.Message {
	return contextMessages(r.dispatchHook(ctx, a, hooks.EventTurnStart, &hooks.Input{
		SessionID: sess.ID,
	}, events))
}

// contextMessages converts a context-providing hook's AdditionalContext
// into a one-element transient system-message slice ready to thread
// through [session.Session.GetMessages]. Returns nil for empty results
// so callers can pass it straight to [slices.Concat] without a guard.
func contextMessages(result *hooks.Result) []chat.Message {
	if result == nil || result.AdditionalContext == "" {
		return nil
	}
	return []chat.Message{{
		Role:    chat.MessageRoleSystem,
		Content: result.AdditionalContext,
	}}
}

// executeSessionEndHooks fires session_end when the run loop exits
// and clears any per-session state held by stateful builtins so a
// long-running runtime stays bounded.
func (r *LocalRuntime) executeSessionEndHooks(ctx context.Context, sess *session.Session, a *agent.Agent) {
	r.dispatchHook(ctx, a, hooks.EventSessionEnd, &hooks.Input{
		SessionID: sess.ID,
		Reason:    "stream_ended",
	}, nil)
	r.builtinsState.ClearSession(sess.ID)
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

// notifyError fires both notification(level=error) and on_error in one
// call. They're always emitted together (an error is always also a
// user-facing notification), so collapsing them into one call expresses
// intent more directly than firing two events at every callsite.
func (r *LocalRuntime) notifyError(ctx context.Context, a *agent.Agent, sessionID, message string) {
	r.notify(ctx, a, hooks.EventNotification, sessionID, "error", message)
	r.notify(ctx, a, hooks.EventOnError, sessionID, "error", message)
}

// notifyMaxIterations fires both notification(level=warning) and
// on_max_iterations. Same rationale as [notifyError]: the two are
// always emitted together when the iteration limit is reached.
func (r *LocalRuntime) notifyMaxIterations(ctx context.Context, a *agent.Agent, sessionID, message string) {
	r.notify(ctx, a, hooks.EventNotification, sessionID, "warning", message)
	r.notify(ctx, a, hooks.EventOnMaxIterations, sessionID, "warning", message)
}

// notify is the shared dispatch path for the (level, message)-shaped
// hook events: notification, on_error, on_max_iterations. They all
// take the same Input fields and are observational (no Result is
// honored), so a single helper covers them all.
func (r *LocalRuntime) notify(ctx context.Context, a *agent.Agent, event hooks.EventType, sessionID, level, message string) {
	r.dispatchHook(ctx, a, event, &hooks.Input{
		SessionID:           sessionID,
		NotificationLevel:   level,
		NotificationMessage: message,
	}, nil)
}

// Agent-switch kinds passed via [hooks.Input.AgentSwitchKind] to
// describe what kind of transition triggered the on_agent_switch
// event. Constants instead of literals so the hook contract is
// discoverable from the runtime side and a typo trips a compile
// error.
const (
	agentSwitchKindTransferTask       = "transfer_task"
	agentSwitchKindTransferTaskReturn = "transfer_task_return"
	agentSwitchKindHandoff            = "handoff"
)

// executeOnAgentSwitchHooks fires on_agent_switch when the runtime
// changes the active agent. Observational; failures are logged. The
// hook runs alongside the existing [AgentSwitching] event, so users
// who already consume that event see no behaviour change.
func (r *LocalRuntime) executeOnAgentSwitchHooks(ctx context.Context, a *agent.Agent, sessionID, fromAgent, toAgent, kind string) {
	r.dispatchHook(ctx, a, hooks.EventOnAgentSwitch, &hooks.Input{
		SessionID:       sessionID,
		FromAgent:       fromAgent,
		ToAgent:         toAgent,
		AgentSwitchKind: kind,
	}, nil)
}

// executeBeforeLLMCallHooks fires before_llm_call just before each
// model call. A terminating verdict (decision="block" / continue=false
// / exit 2) stops the run loop — see [hooks.EventBeforeLLMCall] for
// the contract. Hooks that just want to contribute system messages
// should target turn_start instead.
func (r *LocalRuntime) executeBeforeLLMCallHooks(ctx context.Context, sess *session.Session, a *agent.Agent) (stop bool, message string) {
	result := r.dispatchHook(ctx, a, hooks.EventBeforeLLMCall, &hooks.Input{
		SessionID: sess.ID,
	}, nil)
	if result == nil || result.Allowed {
		return false, ""
	}
	return true, result.Message
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
// stream stopped). Resolves the agent itself so callsites in code paths
// without an agent handle (like the elicitation handler) stay short.
func (r *LocalRuntime) executeOnUserInputHooks(ctx context.Context, sessionID, logContext string) {
	a := r.CurrentAgent()
	if a == nil {
		return
	}
	slog.Debug("Executing on-user-input hooks", "context", logContext)
	r.dispatchHook(ctx, a, hooks.EventOnUserInput, &hooks.Input{
		SessionID: sessionID,
	}, nil)
}
