package runtime

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
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/permissions"
	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/telemetry"
	"github.com/docker/docker-agent/pkg/tools"
)

// processToolCalls executes a batch of tool calls for an agent.
//
// Returns (stopRun, message) when a post_tool_use hook signalled a
// terminating verdict during this batch; the run loop then fans out
// the standard Error / notification / on_error stanzas before exiting.
// (false, "") in every other path — including user cancellation,
// which halts the *batch* but keeps the loop alive so the synthesised
// tool error responses can be sent back to the model on the next turn.
func (r *LocalRuntime) processToolCalls(ctx context.Context, sess *session.Session, calls []tools.ToolCall, agentTools []tools.Tool, events chan Event) (stopRun bool, stopMessage string) {
	a := r.resolveSessionAgent(sess)
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
			r.addToolErrorResponse(ctx, sess, tc, agentToolMap[tc.Function.Name], events, a, reason)
		}
	}

	for i, toolCall := range calls {
		callCtx, callSpan := r.startSpan(ctx, "runtime.tool.call", trace.WithAttributes(
			attribute.String("tool.name", toolCall.Function.Name),
			attribute.String("tool.type", string(toolCall.Type)),
			attribute.String("agent", a.Name()),
			attribute.String("session.id", sess.ID),
			attribute.String("tool.call_id", toolCall.ID),
		))

		slog.Debug("Processing tool call", "agent", a.Name(), "tool", toolCall.Function.Name, "session_id", sess.ID)

		// Tools the model invokes must be in the agent's tool set. After
		// a handoff the model may hallucinate tools it saw in history
		// from a previous agent; surfacing an error response lets it
		// self-correct.
		tool, available := agentToolMap[toolCall.Function.Name]
		if !available {
			slog.Warn("Tool call for unavailable tool", "agent", a.Name(), "tool", toolCall.Function.Name, "session_id", sess.ID)
			r.addToolErrorResponse(ctx, sess, toolCall, tools.Tool{Name: toolCall.Function.Name}, events, a,
				fmt.Sprintf("Tool '%s' is not available. You can only use the tools provided to you.", toolCall.Function.Name))
			callSpan.SetStatus(codes.Error, "tool not available")
			callSpan.End()
			continue
		}

		// Build the tool invoker. Runtime-managed tools (transfer_task,
		// handoff, ...) skip pre/post hooks; user tools go through the
		// hook-aware path and may produce a stopRun outcome.
		invoke := r.toolInvoker(callCtx, sess, toolCall, tool, events, a)

		outcome := r.executeWithApproval(callCtx, sess, toolCall, tool, events, a, invoke)

		if outcome.canceled {
			callSpan.SetStatus(codes.Ok, "tool call canceled by user")
		} else {
			callSpan.SetStatus(codes.Ok, "tool call processed")
		}
		callSpan.End()

		switch {
		case outcome.canceled:
			synthesizeRemaining(calls[i+1:],
				"The tool call was canceled because a previous tool call in the same batch was canceled by the user.")
			return false, ""
		case outcome.stopRun:
			synthesizeRemaining(calls[i+1:],
				"The tool call was skipped because a post_tool_use hook signalled run termination.")
			return true, outcome.stopMessage
		}
	}
	return false, ""
}

// toolApprovalOutcome carries the verdicts of [LocalRuntime.executeWithApproval].
// canceled and stopRun are mutually exclusive in practice but the loop
// treats them differently: cancellation halts the current batch silently;
// stopRun also terminates the agent's run loop with a user-visible reason.
type toolApprovalOutcome struct {
	canceled    bool
	stopRun     bool
	stopMessage string
}

// toolInvoker returns a closure that runs the tool when approved.
// Runtime-managed tools (those registered in r.toolMap) skip pre/post
// hooks; everything else goes through [LocalRuntime.runTool] and may
// yield a stopRun outcome from a post_tool_use hook.
func (r *LocalRuntime) toolInvoker(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	events chan Event,
	a *agent.Agent,
) func() toolApprovalOutcome {
	if handler, ok := r.toolMap[toolCall.Function.Name]; ok {
		return func() toolApprovalOutcome {
			r.runAgentTool(ctx, handler, sess, toolCall, tool, events, a)
			return toolApprovalOutcome{}
		}
	}
	return func() toolApprovalOutcome {
		return r.runTool(ctx, tool, toolCall, events, sess, a)
	}
}

// executeWithApproval handles the approval flow and runs the tool when
// approved. The approval flow considers (in order):
//
//  1. sess.ToolsApproved (--yolo flag) — auto-approve everything.
//  2. Session-level permissions (if configured) — Allow/Ask/Deny rules.
//  3. Team-level permissions config.
//  4. Read-only hint — auto-approve.
//  5. Default: ask the user for confirmation.
//
// The returned [toolApprovalOutcome] captures user cancellation and
// any post_tool_use stopRun verdict propagated from invoke.
func (r *LocalRuntime) executeWithApproval(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	events chan Event,
	a *agent.Agent,
	invoke func() toolApprovalOutcome,
) toolApprovalOutcome {
	toolName := toolCall.Function.Name

	if sess.ToolsApproved {
		slog.Debug("Tool auto-approved by --yolo flag", "tool", toolName, "session_id", sess.ID)
		return invoke()
	}

	// Parse tool arguments once for permission matching.
	var toolArgs map[string]any
	if toolCall.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs); err != nil {
			slog.Debug("Failed to parse tool arguments for permission check", "tool", toolName, "error", err)
			// Fall through with nil args — only tool name patterns can match.
		}
	}

	for _, pc := range r.permissionCheckers(sess) {
		switch pc.checker.CheckWithArgs(toolName, toolArgs) {
		case permissions.Deny:
			slog.Debug("Tool denied by permissions", "tool", toolName, "source", pc.source, "session_id", sess.ID)
			r.addToolErrorResponse(ctx, sess, toolCall, tool, events, a,
				fmt.Sprintf("Tool '%s' is denied by %s.", toolName, pc.source))
			return toolApprovalOutcome{}
		case permissions.Allow:
			slog.Debug("Tool auto-approved by permissions", "tool", toolName, "source", pc.source, "session_id", sess.ID)
			return invoke()
		case permissions.ForceAsk:
			slog.Debug("Tool requires confirmation (ask pattern)", "tool", toolName, "source", pc.source, "session_id", sess.ID)
			return r.askUserForConfirmation(ctx, sess, toolCall, tool, events, a, invoke)
		case permissions.Ask:
			// No explicit match at this level; fall through.
		}
	}

	if tool.Annotations.ReadOnlyHint {
		return invoke()
	}
	return r.askUserForConfirmation(ctx, sess, toolCall, tool, events, a, invoke)
}

// permissionChecker pairs a checker with a human-readable source label.
type permissionChecker struct {
	checker *permissions.Checker
	source  string
}

// permissionCheckers returns the ordered list of permission checkers to evaluate.
func (r *LocalRuntime) permissionCheckers(sess *session.Session) []permissionChecker {
	var checkers []permissionChecker
	if sess.Permissions != nil {
		checkers = append(checkers, permissionChecker{
			checker: permissions.NewChecker(&latest.PermissionsConfig{
				Allow: sess.Permissions.Allow,
				Ask:   sess.Permissions.Ask,
				Deny:  sess.Permissions.Deny,
			}),
			source: "session permissions",
		})
	}
	if tc := r.team.Permissions(); tc != nil {
		checkers = append(checkers, permissionChecker{
			checker: tc,
			source:  "permissions configuration",
		})
	}
	return checkers
}

// askUserForConfirmation sends a confirmation event and waits for the
// user's response. Only called when --yolo is not active and no
// permission rule auto-approved the tool.
func (r *LocalRuntime) askUserForConfirmation(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	events chan Event,
	a *agent.Agent,
	invoke func() toolApprovalOutcome,
) toolApprovalOutcome {
	toolName := toolCall.Function.Name
	slog.Debug("Tools not approved, waiting for resume", "tool", toolName, "session_id", sess.ID)
	events <- ToolCallConfirmation(toolCall, tool, a.Name())

	r.executeOnUserInputHooks(ctx, sess.ID, "tool confirmation")

	select {
	case req := <-r.resumeChan:
		switch req.Type {
		case ResumeTypeApprove:
			slog.Debug("Resume signal received, approving tool", "tool", toolName, "session_id", sess.ID)
			return invoke()
		case ResumeTypeApproveSession:
			slog.Debug("Resume signal received, approving session", "tool", toolName, "session_id", sess.ID)
			sess.ToolsApproved = true
			return invoke()
		case ResumeTypeApproveTool:
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
			return invoke()
		case ResumeTypeReject:
			slog.Debug("Resume signal received, rejecting tool", "tool", toolName, "session_id", sess.ID, "reason", req.Reason)
			rejectMsg := "The user rejected the tool call."
			if strings.TrimSpace(req.Reason) != "" {
				rejectMsg += " Reason: " + strings.TrimSpace(req.Reason)
			}
			r.addToolErrorResponse(ctx, sess, toolCall, tool, events, a, rejectMsg)
		}
		return toolApprovalOutcome{}
	case <-ctx.Done():
		slog.Debug("Context cancelled while waiting for resume", "tool", toolName, "session_id", sess.ID)
		r.addToolErrorResponse(ctx, sess, toolCall, tool, events, a, "The tool call was canceled by the user.")
		return toolApprovalOutcome{canceled: true}
	}
}

// executeToolWithHandler is a common helper that handles tool execution, error handling,
// event emission, and session updates. It reduces duplication between runTool and runAgentTool.
func (r *LocalRuntime) executeToolWithHandler(
	ctx context.Context,
	toolCall tools.ToolCall,
	tool tools.Tool,
	events chan Event,
	sess *session.Session,
	a *agent.Agent,
	spanName string,
	execute func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error),
) *tools.ToolCallResult {
	ctx, span := r.startSpan(ctx, spanName, trace.WithAttributes(
		attribute.String("tool.name", toolCall.Function.Name),
		attribute.String("agent", a.Name()),
		attribute.String("session.id", sess.ID),
		attribute.String("tool.call_id", toolCall.ID),
	))
	defer span.End()

	events <- ToolCall(toolCall, tool, a.Name())

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

	events <- ToolCallResponse(toolCall.ID, tool, res, res.Output, a.Name())

	// Ensure tool response content is not empty for API compatibility
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

	// If the tool result contains images, attach them as MultiContent
	if len(res.Images) > 0 {
		multiContent := []chat.MessagePart{
			{
				Type: chat.MessagePartTypeText,
				Text: content,
			},
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

	addAgentMessage(sess, a, &toolResponseMsg, events)
	return res
}

// runTool executes a user tool from a toolset (MCP, filesystem, ...).
// Returns a [toolApprovalOutcome] whose stopRun/stopMessage fields
// reflect any post_tool_use deny verdict; canceled stays false (user
// cancellation only happens during the approval flow, before this).
func (r *LocalRuntime) runTool(ctx context.Context, tool tools.Tool, toolCall tools.ToolCall, events chan Event, sess *session.Session, a *agent.Agent) toolApprovalOutcome {
	blocked, toolCall := r.executePreToolHook(ctx, sess, toolCall, tool, events, a)
	if blocked {
		return toolApprovalOutcome{}
	}

	res := r.executeToolWithHandler(ctx, toolCall, tool, events, sess, a, "runtime.tool.handler",
		func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error) {
			res, err := tool.Handler(ctx, toolCall)
			return res, 0, err
		})

	stop, msg := r.executePostToolHook(ctx, sess, toolCall, res, a, events)
	return toolApprovalOutcome{stopRun: stop, stopMessage: msg}
}

// newHooksInput builds a hooks.Input from the common tool-call fields.
// [hooks.Executor.Dispatch] auto-fills Cwd from the executor's working
// directory, so callers don't set it here.
func newHooksInput(sess *session.Session, toolCall tools.ToolCall) *hooks.Input {
	return &hooks.Input{
		SessionID: sess.ID,
		ToolName:  toolCall.Function.Name,
		ToolUseID: toolCall.ID,
		ToolInput: parseToolInput(toolCall.Function.Arguments),
	}
}

func newPostToolHookInput(sess *session.Session, toolCall tools.ToolCall, res *tools.ToolCallResult) *hooks.Input {
	input := newHooksInput(sess, toolCall)
	if res != nil {
		input.ToolResponse = res.Output
		input.ToolError = res.IsError
	}
	return input
}

// executePreToolHook runs the pre-tool-use hook and returns whether the tool
// call was blocked and the (possibly modified) tool call.
func (r *LocalRuntime) executePreToolHook(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	tool tools.Tool,
	events chan Event,
	a *agent.Agent,
) (blocked bool, modifiedTC tools.ToolCall) {
	// dispatchHook returns nil when no hook is configured, the agent is
	// missing, or dispatch failed — in every case the right move is to
	// run the tool unchanged.
	result := r.dispatchHook(ctx, a, hooks.EventPreToolUse, newHooksInput(sess, toolCall), events)
	if result == nil {
		return false, toolCall
	}

	if !result.Allowed {
		slog.Debug("Pre-tool hook blocked tool call", "tool", toolCall.Function.Name, "message", result.Message)
		events <- HookBlocked(toolCall, tool, result.Message, a.Name())
		r.addToolErrorResponse(ctx, sess, toolCall, tool, events, a, "Tool call blocked by hook: "+result.Message)
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

// executePostToolHook runs the post-tool-use hook. SystemMessage is
// emitted as a Warning by [dispatchHook]. A terminating verdict
// (decision="block" / continue=false / exit 2) is propagated to the
// run loop via the (stop, message) return.
func (r *LocalRuntime) executePostToolHook(
	ctx context.Context,
	sess *session.Session,
	toolCall tools.ToolCall,
	res *tools.ToolCallResult,
	a *agent.Agent,
	events chan Event,
) (stop bool, message string) {
	result := r.dispatchHook(ctx, a, hooks.EventPostToolUse, newPostToolHookInput(sess, toolCall, res), events)
	if result == nil || result.Allowed {
		return false, ""
	}
	return true, result.Message
}

// parseToolInput parses tool arguments JSON into a map
func parseToolInput(arguments string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(arguments), &result); err != nil {
		return nil
	}
	return result
}

func (r *LocalRuntime) runAgentTool(ctx context.Context, handler ToolHandlerFunc, sess *session.Session, toolCall tools.ToolCall, tool tools.Tool, events chan Event, a *agent.Agent) {
	r.executeToolWithHandler(ctx, toolCall, tool, events, sess, a, "runtime.tool.handler.runtime",
		func(ctx context.Context) (*tools.ToolCallResult, time.Duration, error) {
			start := time.Now()
			res, err := handler(ctx, sess, toolCall, events)
			return res, time.Since(start), err
		})
}

func addAgentMessage(sess *session.Session, a *agent.Agent, msg *chat.Message, events chan Event) {
	agentMsg := session.NewAgentMessage(a.Name(), msg)
	sess.AddMessage(agentMsg)
	events <- MessageAdded(sess.ID, agentMsg, a.Name())
}

// addToolErrorResponse adds a tool error response to the session and emits the event.
// This consolidates the common pattern used by validation, rejection, and cancellation responses.
func (r *LocalRuntime) addToolErrorResponse(_ context.Context, sess *session.Session, toolCall tools.ToolCall, tool tools.Tool, events chan Event, a *agent.Agent, errorMsg string) {
	events <- ToolCallResponse(toolCall.ID, tool, tools.ResultError(errorMsg), errorMsg, a.Name())

	toolResponseMsg := chat.Message{
		Role:       chat.MessageRoleTool,
		Content:    errorMsg,
		ToolCallID: toolCall.ID,
		IsError:    true,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	addAgentMessage(sess, a, &toolResponseMsg, events)
}
