package runtime

import (
	"context"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/permissions"
	"github.com/docker/docker-agent/pkg/runtime/toolexec"
	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/tools"
)

// processToolCalls builds a [toolexec.Dispatcher] for the current event
// channel and delegates the per-call orchestration to it. The dispatcher
// owns the approval flow, hook dispatch, tracing, telemetry, and event
// emission; this method only assembles the runtime-side adapters.
//
// A new dispatcher is constructed per call (cheap: a struct literal plus
// a small handler-binding map) so its handlers and emitter capture the
// current RunStream's events channel.
//
// Returns (stopRun, message) when a post_tool_use hook signalled a
// terminating verdict during this batch; the run loop then fans out the
// standard Error / notification / on_error stanzas before exiting.
// (false, "") in every other path — including user cancellation, which
// halts the *batch* but keeps the loop alive so the synthesised tool
// error responses can be sent back to the model on the next turn.
func (r *LocalRuntime) processToolCalls(ctx context.Context, sess *session.Session, calls []tools.ToolCall, agentTools []tools.Tool, events chan Event) (stopRun bool, stopMessage string) {
	d := &toolexec.Dispatcher{
		Tracer:      r.tracer,
		Hooks:       newHookDispatcher(r, events),
		Resume:      r.resumeChan,
		AgentFor:    r.resolveSessionAgent,
		Permissions: r.permissionCheckers,
		Handlers:    r.bindRuntimeHandlers(events),
	}
	return d.Process(ctx, sess, calls, agentTools, &chanEmitter{events: events})
}

// bindRuntimeHandlers wraps every entry in [r.toolMap] (which receives
// chan Event) into a [toolexec.ToolHandler] (which doesn't), capturing
// the current events channel via closure. Doing this once per dispatch
// keeps the toolexec interface clean of runtime-specific event types.
func (r *LocalRuntime) bindRuntimeHandlers(events chan Event) map[string]toolexec.ToolHandler {
	handlers := make(map[string]toolexec.ToolHandler, len(r.toolMap))
	for name, h := range r.toolMap {
		handlers[name] = func(ctx context.Context, sess *session.Session, tc tools.ToolCall) (*tools.ToolCallResult, error) {
			return h(ctx, sess, tc, events)
		}
	}
	return handlers
}

// permissionCheckers returns the ordered list of permission checkers to evaluate
// (session-level first, then team-level).
func (r *LocalRuntime) permissionCheckers(sess *session.Session) []toolexec.NamedChecker {
	var checkers []toolexec.NamedChecker
	if sess.Permissions != nil {
		checkers = append(checkers, toolexec.NamedChecker{
			Checker: permissions.NewChecker(&latest.PermissionsConfig{
				Allow: sess.Permissions.Allow,
				Ask:   sess.Permissions.Ask,
				Deny:  sess.Permissions.Deny,
			}),
			Source: "session permissions",
		})
	}
	if tc := r.team.Permissions(); tc != nil {
		checkers = append(checkers, toolexec.NamedChecker{
			Checker: tc,
			Source:  "permissions configuration",
		})
	}
	return checkers
}

// chanEmitter adapts a chan Event into a [toolexec.Emitter]. It is the
// only place where the dispatcher's typed event surface meets the
// runtime's event channel; new dispatcher events should grow this type
// in lockstep with the [toolexec.Emitter] interface.
type chanEmitter struct {
	events chan Event
}

func (e *chanEmitter) EmitToolCall(toolCall tools.ToolCall, tool tools.Tool, agentName string) {
	e.events <- ToolCall(toolCall, tool, agentName)
}

func (e *chanEmitter) EmitToolCallResponse(toolCallID string, tool tools.Tool, result *tools.ToolCallResult, output, agentName string) {
	e.events <- ToolCallResponse(toolCallID, tool, result, output, agentName)
}

func (e *chanEmitter) EmitToolCallConfirmation(toolCall tools.ToolCall, tool tools.Tool, agentName string) {
	e.events <- ToolCallConfirmation(toolCall, tool, agentName)
}

func (e *chanEmitter) EmitHookBlocked(toolCall tools.ToolCall, tool tools.Tool, message, agentName string) {
	e.events <- HookBlocked(toolCall, tool, message, agentName)
}

func (e *chanEmitter) EmitMessageAdded(sessionID string, msg *session.Message, agentName string) {
	e.events <- MessageAdded(sessionID, msg, agentName)
}

// hookDispatcher adapts the runtime's per-agent [hooks.Executor] machinery
// into the small [toolexec.HookDispatcher] interface. The events channel
// is captured here so [LocalRuntime.dispatchHook] can surface
// SystemMessage as a Warning event during dispatch.
type hookDispatcher struct {
	r      *LocalRuntime
	events chan Event
}

func newHookDispatcher(r *LocalRuntime, events chan Event) toolexec.HookDispatcher {
	return &hookDispatcher{r: r, events: events}
}

func (h *hookDispatcher) Dispatch(ctx context.Context, a *agent.Agent, event hooks.EventType, in *hooks.Input) *hooks.Result {
	return h.r.dispatchHook(ctx, a, event, in, h.events)
}

func (h *hookDispatcher) NotifyUserInput(ctx context.Context, sessionID, label string) {
	h.r.executeOnUserInputHooks(ctx, sessionID, label)
}

func (h *hookDispatcher) NotifyApprovalDecision(ctx context.Context, sess *session.Session, a *agent.Agent, tc tools.ToolCall, decision, source string) {
	h.r.executeOnToolApprovalDecisionHooks(ctx, sess, a, tc, decision, source)
}

// allowSourceFor maps a permission-checker source label to the
// corresponding approval-decision source classifier. Thin wrapper kept
// in the runtime package so tests can pin the stable mapping without
// reaching into [toolexec]'s internals.
func allowSourceFor(checkerSource string) string {
	if checkerSource == "session permissions" {
		return ApprovalSourceSessionPermissionsAllow
	}
	return ApprovalSourceTeamPermissionsAllow
}

// denySourceFor mirrors allowSourceFor for the deny path.
func denySourceFor(checkerSource string) string {
	if checkerSource == "session permissions" {
		return ApprovalSourceSessionPermissionsDeny
	}
	return ApprovalSourceTeamPermissionsDeny
}

// addAgentMessage records a chat message to the session and emits the
// resulting MessageAdded event. Used by the loop for assistant messages
// and max-iteration stop messages. The dispatcher emits its own variant
// directly via the [toolexec.Emitter] interface.
func addAgentMessage(sess *session.Session, a *agent.Agent, msg *chat.Message, events chan Event) {
	agentMsg := session.NewAgentMessage(a.Name(), msg)
	sess.AddMessage(agentMsg)
	events <- MessageAdded(sess.ID, agentMsg, a.Name())
}
