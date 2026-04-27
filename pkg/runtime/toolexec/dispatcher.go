package toolexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/telemetry"
	"github.com/docker/docker-agent/pkg/tools"
)

// Verdicts and sources surfaced via [HookDispatcher.NotifyApprovalDecision].
// The strings are part of the on_tool_approval_decision hook contract and
// must stay stable.
const (
	ApprovalDecisionAllow    = "allow"
	ApprovalDecisionDeny     = "deny"
	ApprovalDecisionCanceled = "canceled"

	ApprovalSourceYolo                    = "yolo"
	ApprovalSourceSessionPermissionsAllow = "session_permissions_allow"
	ApprovalSourceSessionPermissionsDeny  = "session_permissions_deny"
	ApprovalSourceTeamPermissionsAllow    = "team_permissions_allow"
	ApprovalSourceTeamPermissionsDeny     = "team_permissions_deny"
	ApprovalSourceReadOnlyHint            = "readonly_hint"
	ApprovalSourceUserApproved            = "user_approved"
	ApprovalSourceUserApprovedSession     = "user_approved_session"
	ApprovalSourceUserApprovedTool        = "user_approved_tool"
	ApprovalSourceUserRejected            = "user_rejected"
	ApprovalSourceContextCanceled         = "context_canceled"
)

// CallOutcome captures the verdicts of a single tool invocation as
// observed by the dispatcher.
//
// Canceled and StopRun are mutually exclusive in practice but signal
// different things to the caller: cancellation halts the current batch
// silently (the run loop continues so the synthesised tool error
// responses can be sent back to the model on the next turn); StopRun
// also terminates the agent's run loop with a user-visible reason
// produced by a post_tool_use hook deny verdict.
type CallOutcome struct {
	Canceled    bool
	StopRun     bool
	StopMessage string
}

// Emitter receives the events the [Dispatcher] emits while processing a
// batch of tool calls. Runtimes typically implement this by sending typed
// events to their event channel.
//
// The dispatcher only emits the five events below. Runtime-managed
// handlers (registered via [Dispatcher.Handlers]) emit any additional
// runtime-specific events directly via the channel they captured at
// registration time.
type Emitter interface {
	EmitToolCall(toolCall tools.ToolCall, tool tools.Tool, agentName string)
	EmitToolCallResponse(toolCallID string, tool tools.Tool, result *tools.ToolCallResult, output, agentName string)
	EmitToolCallConfirmation(toolCall tools.ToolCall, tool tools.Tool, agentName string)
	EmitHookBlocked(toolCall tools.ToolCall, tool tools.Tool, message, agentName string)
	EmitMessageAdded(sessionID string, msg *session.Message, agentName string)
}

// HookDispatcher abstracts pre/post tool-use hook dispatch and the
// "user is being prompted" notification.
type HookDispatcher interface {
	// Dispatch fires a tool-related hook (typically [hooks.EventPreToolUse]
	// or [hooks.EventPostToolUse]).  Return nil when no hook is configured
	// or dispatch failed: the dispatcher treats nil uniformly as
	// "carry on with the original call". SystemMessage emission is the
	// implementation's responsibility.
	Dispatch(ctx context.Context, a *agent.Agent, event hooks.EventType, in *hooks.Input) *hooks.Result

	// NotifyUserInput is invoked just before the dispatcher blocks waiting
	// for the user (tool confirmation). Implementations typically fire
	// [hooks.EventOnUserInput].
	NotifyUserInput(ctx context.Context, sessionID, label string)

	// NotifyApprovalDecision is invoked once per tool call after the
	// approval pipeline (auto-allow, deny, user confirmation, ...) has
	// resolved a verdict. Implementations typically fire
	// [hooks.EventOnToolApprovalDecision] with decision and source set
	// to the supplied strings (see ApprovalDecision* / ApprovalSource*
	// constants).
	NotifyApprovalDecision(ctx context.Context, sess *session.Session, a *agent.Agent, tc tools.ToolCall, decision, source string)
}

// ToolHandler is the signature for runtime-managed tool handlers
// (e.g. transfer_task, handoff, change_model). Handlers receive the
// parsed call and return the result.
//
// The dispatcher wraps every handler in the same tracing/telemetry/event-
// emission pipeline used for ordinary toolset tools, so handlers MUST NOT
// emit ToolCall/ToolCallResponse themselves. Handlers that need to emit
// additional runtime-specific events (e.g. an agent-info event after a
// model change) should be wired by the caller to capture the relevant
// channel via closure when registering the handler.
type ToolHandler func(ctx context.Context, sess *session.Session, tc tools.ToolCall) (*tools.ToolCallResult, error)

// ResumeRequest carries the user's response to a tool-confirmation prompt.
// The runtime aliases this type publicly via runtime.ResumeRequest so the
// dispatcher and the runtime share one definition.
type ResumeRequest struct {
	Type     ResumeType
	Reason   string // Optional; primarily used with [ResumeTypeReject]
	ToolName string // Optional; used with [ResumeTypeApproveTool]
}

// ResumeType identifies the kind of confirmation a user responded with.
type ResumeType string

const (
	ResumeTypeApprove        ResumeType = "approve"
	ResumeTypeApproveSession ResumeType = "approve-session"
	ResumeTypeApproveTool    ResumeType = "approve-tool"
	ResumeTypeReject         ResumeType = "reject"
)

// Dispatcher executes batches of tool calls. Construct one per runtime
// and call [Dispatcher.Process] for each LLM response. The dispatcher is
// goroutine-safe only insofar as its dependencies are.
type Dispatcher struct {
	// Tracer records per-call spans. May be nil (no-op tracing).
	Tracer trace.Tracer

	// Hooks dispatches pre/post tool-use hooks. May be nil for runtimes
	// without hook support; in that case every call runs unchanged.
	Hooks HookDispatcher

	// Resume receives user-confirmation responses. Must be set; the
	// dispatcher blocks on it whenever a tool requires confirmation.
	Resume <-chan ResumeRequest

	// AgentFor returns the active agent for a session. Required.
	AgentFor func(*session.Session) *agent.Agent

	// Permissions returns the ordered list of permission checkers for a
	// session (typically session-level first, then team-level).
	Permissions func(*session.Session) []NamedChecker

	// Handlers maps tool names to runtime-managed handlers (transfer_task,
	// handoff, change_model, ...). Tools not in this map are routed to
	// their toolset Handler.
	Handlers map[string]ToolHandler
}

// Process runs every tool call in calls in order, emitting events through
// em.
//
// Returns (stopRun, message) when a post_tool_use hook signalled a
// terminating verdict during this batch; the run loop then fans out the
// standard Error / notification / on_error stanzas before exiting.
// (false, "") in every other path — including user cancellation, which
// halts the *batch* but keeps the loop alive so the synthesised tool
// error responses can be sent back to the model on the next turn.
func (d *Dispatcher) Process(ctx context.Context, sess *session.Session, calls []tools.ToolCall, agentTools []tools.Tool, em Emitter) (stopRun bool, stopMessage string) {
	a := d.AgentFor(sess)
	slog.Debug("Processing tool calls", "agent", a.Name(), "call_count", len(calls))

	agentToolMap := make(map[string]tools.Tool, len(agentTools))
	for _, t := range agentTools {
		agentToolMap[t.Name] = t
	}

	// synthesizeRemaining adds error responses for tool calls we won't
	// run because the batch was halted (user cancellation or post-tool
	// stopRun). Orphan function calls without matching outputs are
	// rejected by the Responses API, so we surface them as errors
	// rather than dropping them.
	synthesizeRemaining := func(remaining []tools.ToolCall, reason string) {
		for _, tc := range remaining {
			d.addToolErrorResponse(sess, tc, agentToolMap[tc.Function.Name], em, a, reason)
		}
	}

	for i, toolCall := range calls {
		callCtx, callSpan := d.startSpan(ctx, "runtime.tool.call", trace.WithAttributes(
			attribute.String("tool.name", toolCall.Function.Name),
			attribute.String("tool.type", string(toolCall.Type)),
			attribute.String("agent", a.Name()),
			attribute.String("session.id", sess.ID),
			attribute.String("tool.call_id", toolCall.ID),
		))

		slog.Debug("Processing tool call", "agent", a.Name(), "tool", toolCall.Function.Name, "session_id", sess.ID)

		// Resolve the tool: it must be in the agent's tool set to be callable.
		// After a handoff the model may hallucinate tools it saw in the
		// conversation history from a previous agent; rejecting unknown
		// tools with an error response lets it self-correct.
		tool, available := agentToolMap[toolCall.Function.Name]
		if !available {
			slog.Warn("Tool call for unavailable tool", "agent", a.Name(), "tool", toolCall.Function.Name, "session_id", sess.ID)
			errTool := tools.Tool{Name: toolCall.Function.Name}
			d.addToolErrorResponse(sess, toolCall, errTool, em, a, fmt.Sprintf("Tool '%s' is not available. You can only use the tools provided to you.", toolCall.Function.Name))
			callSpan.SetStatus(codes.Error, "tool not available")
			callSpan.End()
			continue
		}

		// Pick the handler: runtime-managed tools (transfer_task, handoff)
		// have dedicated handlers; everything else goes through the toolset.
		// Runtime-managed handlers skip pre/post hooks; toolset tools go
		// through the hook-aware path and may produce a stopRun outcome.
		var runTool func() CallOutcome
		if handler, exists := d.Handlers[toolCall.Function.Name]; exists {
			runTool = func() CallOutcome {
				d.runHandlerTool(callCtx, handler, sess, toolCall, tool, em, a)
				return CallOutcome{}
			}
		} else {
			runTool = func() CallOutcome {
				return d.runToolsetTool(callCtx, tool, toolCall, em, sess, a)
			}
		}

		outcome := d.executeWithApproval(callCtx, sess, toolCall, tool, em, a, runTool)

		if outcome.Canceled {
			callSpan.SetStatus(codes.Ok, "tool call canceled by user")
		} else {
			callSpan.SetStatus(codes.Ok, "tool call processed")
		}
		callSpan.End()

		switch {
		case outcome.Canceled:
			synthesizeRemaining(calls[i+1:],
				"The tool call was canceled because a previous tool call in the same batch was canceled by the user.")
			return false, ""
		case outcome.StopRun:
			synthesizeRemaining(calls[i+1:],
				"The tool call was skipped because a post_tool_use hook signalled run termination.")
			return true, outcome.StopMessage
		}
	}
	return false, ""
}

// executeWithApproval handles the tool approval flow and executes the tool.
//
// The approval flow is fully resolved by [Decide]; this function only
// translates the resulting decision into runtime side effects (run,
// deny-with-error-response, ask-and-wait) and forwards the decision to
// [HookDispatcher.NotifyApprovalDecision] for hook tracking.
func (d *Dispatcher) executeWithApproval(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	em Emitter,
	a *agent.Agent,
	runTool func() CallOutcome,
) CallOutcome {
	toolName := toolCall.Function.Name

	decision := Decide(
		sess.ToolsApproved,
		d.permissionsFor(sess),
		toolName,
		ParseToolInput(toolCall.Function.Arguments),
		tool.Annotations.ReadOnlyHint,
	)

	switch decision.Outcome {
	case OutcomeAllow:
		logAllow(decision, toolName, sess.ID)
		d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionAllow, allowSourceForDecision(decision))
		return runTool()
	case OutcomeDeny:
		slog.Debug("Tool denied by permissions", "tool", toolName, "source", decision.Source, "session_id", sess.ID)
		d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionDeny, denySourceForChecker(decision.Source))
		d.addToolErrorResponse(sess, toolCall, tool, em, a, fmt.Sprintf("Tool '%s' is denied by %s.", toolName, decision.Source))
		return CallOutcome{}
	case OutcomeAsk:
		if decision.Reason == ReasonChecker {
			slog.Debug("Tool requires confirmation (ask pattern)", "tool", toolName, "source", decision.Source, "session_id", sess.ID)
		}
		return d.askUserForConfirmation(ctx, sess, toolCall, tool, em, a, runTool)
	}
	return CallOutcome{}
}

// permissionsFor returns the ordered checkers for sess, or nil when none
// are configured. Centralizing the nil-guard keeps callers terse.
func (d *Dispatcher) permissionsFor(sess *session.Session) []NamedChecker {
	if d.Permissions == nil {
		return nil
	}
	return d.Permissions(sess)
}

// notifyApproval forwards the resolved approval decision to the
// HookDispatcher, when one is configured. Centralised so the nil-guard
// stays in one place.
func (d *Dispatcher) notifyApproval(ctx context.Context, sess *session.Session, a *agent.Agent, tc tools.ToolCall, decision, source string) {
	if d.Hooks == nil {
		return
	}
	d.Hooks.NotifyApprovalDecision(ctx, sess, a, tc, decision, source)
}

// allowSourceForDecision maps a [PermissionDecision] with [OutcomeAllow]
// onto the corresponding ApprovalSource* constant.
func allowSourceForDecision(d PermissionDecision) string {
	switch d.Reason {
	case ReasonYolo:
		return ApprovalSourceYolo
	case ReasonReadOnlyHint:
		return ApprovalSourceReadOnlyHint
	case ReasonChecker:
		return allowSourceForChecker(d.Source)
	}
	return allowSourceForChecker(d.Source)
}

// allowSourceForChecker maps a checker source label ("session permissions"
// or "permissions configuration") onto the corresponding ApprovalSource*
// allow constant.
func allowSourceForChecker(checkerSource string) string {
	if checkerSource == "session permissions" {
		return ApprovalSourceSessionPermissionsAllow
	}
	return ApprovalSourceTeamPermissionsAllow
}

// denySourceForChecker mirrors allowSourceForChecker for the deny path.
func denySourceForChecker(checkerSource string) string {
	if checkerSource == "session permissions" {
		return ApprovalSourceSessionPermissionsDeny
	}
	return ApprovalSourceTeamPermissionsDeny
}

// logAllow emits the auto-approval debug log appropriate to the reason
// (--yolo, an explicit checker rule, or the read-only hint).
func logAllow(decision PermissionDecision, toolName, sessionID string) {
	switch decision.Reason {
	case ReasonYolo:
		slog.Debug("Tool auto-approved by --yolo flag", "tool", toolName, "session_id", sessionID)
	case ReasonChecker:
		slog.Debug("Tool auto-approved by permissions", "tool", toolName, "source", decision.Source, "session_id", sessionID)
		// ReasonReadOnlyHint is intentionally silent (matches prior behaviour).
	}
}

// askUserForConfirmation sends a confirmation event and waits for user
// response. Only called when no permission rule auto-approved the tool.
func (d *Dispatcher) askUserForConfirmation(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	em Emitter,
	a *agent.Agent,
	runTool func() CallOutcome,
) CallOutcome {
	toolName := toolCall.Function.Name
	slog.Debug("Tools not approved, waiting for resume", "tool", toolName, "session_id", sess.ID)
	em.EmitToolCallConfirmation(toolCall, tool, a.Name())

	if d.Hooks != nil {
		d.Hooks.NotifyUserInput(ctx, sess.ID, "tool confirmation")
	}

	select {
	case req := <-d.Resume:
		switch req.Type {
		case ResumeTypeApprove:
			slog.Debug("Resume signal received, approving tool", "tool", toolName, "session_id", sess.ID)
			d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionAllow, ApprovalSourceUserApproved)
			return runTool()
		case ResumeTypeApproveSession:
			slog.Debug("Resume signal received, approving session", "tool", toolName, "session_id", sess.ID)
			sess.ToolsApproved = true
			d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionAllow, ApprovalSourceUserApprovedSession)
			return runTool()
		case ResumeTypeApproveTool:
			// Add the tool to session's allow list for future auto-approval.
			approvedTool := req.ToolName
			if approvedTool == "" {
				approvedTool = toolName
			}
			if sess.Permissions == nil {
				sess.Permissions = &session.PermissionsConfig{}
			}
			if !slices.Contains(sess.Permissions.Allow, approvedTool) {
				sess.Permissions.Allow = append(sess.Permissions.Allow, approvedTool)
			}
			slog.Debug("Resume signal received, approving tool permanently", "tool", approvedTool, "session_id", sess.ID)
			d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionAllow, ApprovalSourceUserApprovedTool)
			return runTool()
		case ResumeTypeReject:
			slog.Debug("Resume signal received, rejecting tool", "tool", toolName, "session_id", sess.ID, "reason", req.Reason)
			d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionDeny, ApprovalSourceUserRejected)
			rejectMsg := "The user rejected the tool call."
			if strings.TrimSpace(req.Reason) != "" {
				rejectMsg += " Reason: " + strings.TrimSpace(req.Reason)
			}
			d.addToolErrorResponse(sess, toolCall, tool, em, a, rejectMsg)
		}
		return CallOutcome{}
	case <-ctx.Done():
		slog.Debug("Context cancelled while waiting for resume", "tool", toolName, "session_id", sess.ID)
		d.notifyApproval(ctx, sess, a, toolCall, ApprovalDecisionCanceled, ApprovalSourceContextCanceled)
		d.addToolErrorResponse(sess, toolCall, tool, em, a, "The tool call was canceled by the user.")
		return CallOutcome{Canceled: true}
	}
}

// executeToolWithHandler is the common pipeline shared by toolset tools and
// runtime-managed handlers: tracing, event emission, telemetry, error
// translation, and session message persistence.
func (d *Dispatcher) executeToolWithHandler(
	ctx context.Context,
	toolCall tools.ToolCall,
	tool tools.Tool,
	em Emitter,
	sess *session.Session,
	a *agent.Agent,
	spanName string,
	execute func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error),
) *tools.ToolCallResult {
	ctx, span := d.startSpan(ctx, spanName, trace.WithAttributes(
		attribute.String("tool.name", toolCall.Function.Name),
		attribute.String("agent", a.Name()),
		attribute.String("session.id", sess.ID),
		attribute.String("tool.call_id", toolCall.ID),
	))
	defer span.End()

	em.EmitToolCall(toolCall, tool, a.Name())

	res, duration, err := execute(ctx)

	telemetry.RecordToolCall(ctx, toolCall.Function.Name, sess.ID, a.Name(), duration, err)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			slog.Debug("Tool handler canceled by context", "tool", toolCall.Function.Name, "agent", a.Name(), "session_id", sess.ID)
			res = tools.ResultError("The tool call was canceled by the user.")
			span.SetStatus(codes.Ok, "tool handler canceled by user")
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, "tool handler error")
			slog.Error("Error calling tool", "tool", toolCall.Function.Name, "error", err)
			res = tools.ResultError(fmt.Sprintf("Error calling tool: %v", err))
		}
	} else {
		span.SetStatus(codes.Ok, "tool handler completed")
		slog.Debug("Tool call completed", "tool", toolCall.Function.Name, "output_length", len(res.Output))
	}

	em.EmitToolCallResponse(toolCall.ID, tool, res, res.Output, a.Name())

	// Ensure tool response content is not empty for API compatibility.
	content := res.Output
	if strings.TrimSpace(content) == "" {
		content = "(no output)"
	}

	toolResponseMsg := chat.Message{
		Role:       chat.MessageRoleTool,
		Content:    content,
		ToolCallID: toolCall.ID,
		IsError:    res.IsError,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	// If the tool result contains images, attach them as MultiContent.
	if len(res.Images) > 0 {
		multiContent := []chat.MessagePart{
			{Type: chat.MessagePartTypeText, Text: content},
		}
		for _, img := range res.Images {
			multiContent = append(multiContent, chat.MessagePart{
				Type: chat.MessagePartTypeImageURL,
				ImageURL: &chat.MessageImageURL{
					URL:    "data:" + img.MimeType + ";base64," + img.Data,
					Detail: chat.ImageURLDetailAuto,
				},
			})
		}
		toolResponseMsg.MultiContent = multiContent
	}

	addAgentMessage(sess, a, &toolResponseMsg, em)
	return res
}

// runToolsetTool executes a tool from an agent's toolset (MCP, filesystem, ...).
// Returns a [CallOutcome] whose StopRun/StopMessage fields reflect any
// post_tool_use deny verdict; Canceled stays false (user cancellation
// only happens during the approval flow, before this).
func (d *Dispatcher) runToolsetTool(ctx context.Context, tool tools.Tool, toolCall tools.ToolCall, em Emitter, sess *session.Session, a *agent.Agent) CallOutcome {
	// Pre-tool hook: may block the call or rewrite its arguments.
	blocked, toolCall := d.preHook(ctx, sess, toolCall, tool, em, a)
	if blocked {
		return CallOutcome{}
	}

	res := d.executeToolWithHandler(ctx, toolCall, tool, em, sess, a, "runtime.tool.handler",
		func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error) {
			res, err := tool.Handler(ctx, toolCall)
			return res, 0, err
		})

	// Post-tool hook: SystemMessage is surfaced by the HookDispatcher
	// implementation; a terminating verdict (decision="block" /
	// continue=false / exit 2) is propagated to the run loop via
	// CallOutcome.
	stop, msg := d.postHook(ctx, sess, toolCall, res, a)
	return CallOutcome{StopRun: stop, StopMessage: msg}
}

// runHandlerTool executes a runtime-managed tool handler.
func (d *Dispatcher) runHandlerTool(ctx context.Context, handler ToolHandler, sess *session.Session, toolCall tools.ToolCall, tool tools.Tool, em Emitter, a *agent.Agent) {
	d.executeToolWithHandler(ctx, toolCall, tool, em, sess, a, "runtime.tool.handler.runtime",
		func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error) {
			start := time.Now()
			res, err := handler(ctx, sess, toolCall)
			return res, time.Since(start), err
		})
}

// preHook fires the pre-tool-use hook and reports whether the tool call
// was blocked (along with the possibly modified call when allowed).
func (d *Dispatcher) preHook(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	em Emitter,
	a *agent.Agent,
) (blocked bool, modifiedTC tools.ToolCall) {
	if d.Hooks == nil {
		return false, toolCall
	}

	// Dispatch returns nil when no hook is configured, the agent is
	// missing, or dispatch failed — in every case the right move is to
	// run the tool unchanged.
	result := d.Hooks.Dispatch(ctx, a, hooks.EventPreToolUse, NewHooksInput(sess, toolCall))
	if result == nil {
		return false, toolCall
	}

	if !result.Allowed {
		slog.Debug("Pre-tool hook blocked tool call", "tool", toolCall.Function.Name, "message", result.Message)
		em.EmitHookBlocked(toolCall, tool, result.Message, a.Name())
		d.addToolErrorResponse(sess, toolCall, tool, em, a, "Tool call blocked by hook: "+result.Message)
		return true, toolCall
	}

	if result.ModifiedInput != nil {
		if updated, merr := json.Marshal(result.ModifiedInput); merr != nil {
			slog.Warn("Failed to marshal modified tool input from hook", "tool", toolCall.Function.Name, "error", merr)
		} else {
			slog.Debug("Pre-tool hook modified tool input", "tool", toolCall.Function.Name)
			toolCall.Function.Arguments = string(updated)
		}
	}
	return false, toolCall
}

// postHook fires the post-tool-use hook. SystemMessage emission is the
// [HookDispatcher]'s responsibility. A terminating verdict
// (decision="block" / continue=false / exit 2) is propagated via the
// (stop, message) return.
func (d *Dispatcher) postHook(ctx context.Context, sess *session.Session, toolCall tools.ToolCall, res *tools.ToolCallResult, a *agent.Agent) (stop bool, message string) {
	if d.Hooks == nil {
		return false, ""
	}
	result := d.Hooks.Dispatch(ctx, a, hooks.EventPostToolUse, NewPostToolHooksInput(sess, toolCall, res))
	if result == nil || result.Allowed {
		return false, ""
	}
	return true, result.Message
}

// addToolErrorResponse appends an error tool-response to the session and
// emits the corresponding events. Consolidates the pattern shared by
// validation, rejection, and cancellation responses.
func (d *Dispatcher) addToolErrorResponse(sess *session.Session, toolCall tools.ToolCall, tool tools.Tool, em Emitter, a *agent.Agent, errorMsg string) {
	em.EmitToolCallResponse(toolCall.ID, tool, tools.ResultError(errorMsg), errorMsg, a.Name())

	toolResponseMsg := chat.Message{
		Role:       chat.MessageRoleTool,
		Content:    errorMsg,
		ToolCallID: toolCall.ID,
		IsError:    true,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	addAgentMessage(sess, a, &toolResponseMsg, em)
}

// addAgentMessage records a chat message to the session and emits the
// resulting [*session.Message] via the [Emitter].
func addAgentMessage(sess *session.Session, a *agent.Agent, msg *chat.Message, em Emitter) {
	agentMsg := session.NewAgentMessage(a.Name(), msg)
	sess.AddMessage(agentMsg)
	em.EmitMessageAdded(sess.ID, agentMsg, a.Name())
}

// startSpan wraps Tracer.Start; a nil tracer is a no-op so callers don't
// need a guard.
func (d *Dispatcher) startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if d.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return d.Tracer.Start(ctx, name, opts...)
}
