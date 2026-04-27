// Package hooks provides lifecycle hooks for agent tool execution.
// Hooks allow users to run shell commands or in-process Go functions at
// various points during the agent's execution lifecycle, providing
// deterministic control over agent behavior.
package hooks

import (
	"encoding/json"
)

// EventType identifies a hook event.
type EventType string

const (
	// EventPreToolUse fires before a tool call. Can allow/deny/modify it.
	EventPreToolUse EventType = "pre_tool_use"
	// EventPostToolUse fires after a tool completes successfully.
	// Returning decision="block" (or continue=false / exit code 2)
	// stops the run loop after the current tool batch — useful for
	// circuit-breaker patterns like a tool-call loop detector.
	EventPostToolUse EventType = "post_tool_use"
	// EventSessionStart fires when a session begins or resumes.
	EventSessionStart EventType = "session_start"
	// EventTurnStart fires at the start of every agent turn (each model
	// call). AdditionalContext is injected transiently and never persisted.
	EventTurnStart EventType = "turn_start"
	// EventBeforeLLMCall fires immediately before each model call.
	// Returning decision="block" (or continue=false / exit code 2)
	// stops the run loop before the model is invoked — useful for hard
	// budget guards. Use turn_start to contribute system messages;
	// this event's AdditionalContext is not consumed.
	EventBeforeLLMCall EventType = "before_llm_call"
	// EventAfterLLMCall fires immediately after a successful model call,
	// before the response is recorded. Failed calls fire EventOnError.
	EventAfterLLMCall EventType = "after_llm_call"
	// EventSessionEnd fires when a session terminates.
	EventSessionEnd EventType = "session_end"
	// EventOnUserInput fires when the agent needs input from the user.
	EventOnUserInput EventType = "on_user_input"
	// EventStop fires when the model finishes its response.
	EventStop EventType = "stop"
	// EventNotification fires when the agent emits a notification.
	EventNotification EventType = "notification"
	// EventOnError fires when the runtime hits an error during a turn.
	EventOnError EventType = "on_error"
	// EventOnMaxIterations fires when the runtime reaches its max_iterations limit.
	EventOnMaxIterations EventType = "on_max_iterations"
	// EventOnAgentSwitch fires whenever the runtime moves the active
	// agent to a new one — either delegating a task (transfer_task),
	// handing off the conversation (handoff), or returning to the
	// caller after a transferred task completes. Observational; useful
	// for audit, transcript, and metrics pipelines that track which
	// agent ran which tools without subscribing to the runtime event
	// channel.
	EventOnAgentSwitch EventType = "on_agent_switch"
	// EventOnSessionResume fires when the user explicitly approves the
	// runtime to continue past its configured max_iterations limit.
	// Observational; useful for alerting on extended-runtime sessions
	// or for pipelines that bill / quota-track per resume.
	EventOnSessionResume EventType = "on_session_resume"
	// EventOnToolApprovalDecision fires after the runtime's tool
	// approval chain (yolo / permissions / readonly / ask) has resolved
	// a verdict for a tool call, before the call is executed (for
	// allow) or its error response is recorded (for deny / canceled).
	// Observational; gives audit pipelines a single, structured "who
	// approved what" record without re-implementing the chain.
	EventOnToolApprovalDecision EventType = "on_tool_approval_decision"
)

// Input is the JSON-serializable payload passed to hooks via stdin.
type Input struct {
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	HookEventName EventType `json:"hook_event_name"`

	// AgentName identifies the agent dispatching the event. Useful for
	// builtin hooks that need to look up per-agent state via a runtime
	// closure (e.g. response cache).
	AgentName string `json:"agent_name,omitempty"`

	// LastUserMessage is the text content of the latest user message in
	// the session at dispatch time. Populated for events that respond to
	// a user turn (stop, after_llm_call). Empty for events that aren't
	// turn-scoped (session_start, session_end, notification, ...).
	LastUserMessage string `json:"last_user_message,omitempty"`

	// Tool-related fields (PreToolUse and PostToolUse).
	ToolName  string         `json:"tool_name,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`

	// PostToolUse specific.
	ToolResponse any  `json:"tool_response,omitempty"`
	ToolError    bool `json:"tool_error,omitempty"`

	// SessionStart specific: "startup", "resume", "clear", "compact".
	Source string `json:"source,omitempty"`
	// SessionEnd specific: "clear", "logout", "prompt_input_exit", "other".
	Reason string `json:"reason,omitempty"`
	// Stop / AfterLLMCall: the model's final response content.
	StopResponse string `json:"stop_response,omitempty"`
	// Notification specific.
	NotificationLevel   string `json:"notification_level,omitempty"`
	NotificationMessage string `json:"notification_message,omitempty"`

	// OnAgentSwitch specific: the agent the runtime is moving away
	// from (FromAgent) and the one it's switching to (ToAgent), plus
	// the cause of the transition ("transfer_task", "handoff",
	// "transfer_task_return"). Empty FromAgent is valid for the
	// initial switch into the team's default agent.
	FromAgent       string `json:"from_agent,omitempty"`
	ToAgent         string `json:"to_agent,omitempty"`
	AgentSwitchKind string `json:"agent_switch_kind,omitempty"`

	// OnSessionResume specific: the iteration cap that was reached
	// (PreviousMaxIterations) and the new cap after the user approved
	// continuation (NewMaxIterations). Carrying both lets audit
	// pipelines compute how much extra runtime was granted without
	// reconstructing it from the iteration counter.
	PreviousMaxIterations int `json:"previous_max_iterations,omitempty"`
	NewMaxIterations      int `json:"new_max_iterations,omitempty"`

	// OnToolApprovalDecision specific: the verdict resolved by the
	// approval chain ("allow", "deny", "canceled") and a stable
	// classifier for what produced it ("yolo",
	// "session_permissions_allow", "session_permissions_deny",
	// "team_permissions_allow", "team_permissions_deny",
	// "readonly_hint", "user_approved", "user_approved_session",
	// "user_approved_tool", "user_rejected", "context_canceled").
	ApprovalDecision string `json:"approval_decision,omitempty"`
	ApprovalSource   string `json:"approval_source,omitempty"`
}

// ToJSON serializes the input.
func (i *Input) ToJSON() ([]byte, error) { return json.Marshal(i) }

// ErrorPolicy controls what happens when a non-fail-closed hook fails.
type ErrorPolicy string

const (
	ErrorPolicyWarn   ErrorPolicy = "warn"
	ErrorPolicyIgnore ErrorPolicy = "ignore"
	ErrorPolicyBlock  ErrorPolicy = "block"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionAsk   Decision = "ask"
)

// NewAdditionalContextOutput is a small helper for in-process [BuiltinFunc]
// implementations that just want to contribute additional context for a
// given event. Returning the result of this helper is equivalent to
// returning a fully-populated [Output] with [HookSpecificOutput] set.
func NewAdditionalContextOutput(event EventType, content string) *Output {
	if content == "" {
		return nil
	}
	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     event,
			AdditionalContext: content,
		},
	}
}

// Output is the JSON-decoded output of a hook.
type Output struct {
	// Continue indicates whether to continue execution (default: true).
	Continue *bool `json:"continue,omitempty"`
	// StopReason is shown when continue=false.
	StopReason string `json:"stop_reason,omitempty"`
	// SuppressOutput hides stdout from transcript.
	SuppressOutput bool `json:"suppress_output,omitempty"`
	// SystemMessage is a warning to show the user.
	SystemMessage string `json:"system_message,omitempty"`
	// Decision is for blocking operations ("block", ...).
	// In-process builtin hooks should use [DecisionBlockValue].
	Decision string `json:"decision,omitempty"`
	// Reason explains the decision.
	Reason string `json:"reason,omitempty"`
	// HookSpecificOutput contains event-specific fields.
	HookSpecificOutput *HookSpecificOutput `json:"hook_specific_output,omitempty"`
}

// ShouldContinue reports whether execution should continue.
func (o *Output) ShouldContinue() bool { return o.Continue == nil || *o.Continue }

// DecisionBlockValue is the canonical value of [Output.Decision] used
// by hooks to signal a deny/terminate verdict on the current event.
const DecisionBlockValue = "block"

// IsBlocked reports whether the decision is "block".
func (o *Output) IsBlocked() bool { return o.Decision == DecisionBlockValue }

// HookSpecificOutput holds event-specific output fields.
type HookSpecificOutput struct {
	HookEventName EventType `json:"hook_event_name,omitempty"`

	// PreToolUse fields.
	PermissionDecision       Decision       `json:"permission_decision,omitempty"`
	PermissionDecisionReason string         `json:"permission_decision_reason,omitempty"`
	UpdatedInput             map[string]any `json:"updated_input,omitempty"`

	// PostToolUse / SessionStart / TurnStart / Stop fields.
	AdditionalContext string `json:"additional_context,omitempty"`
}

// Result is the aggregated outcome of dispatching one event.
type Result struct {
	// Allowed indicates if the operation should proceed.
	Allowed bool
	// Message is feedback to include in the response.
	Message string
	// ModifiedInput contains modifications to tool input (PreToolUse).
	ModifiedInput map[string]any
	// AdditionalContext is context added by the hooks.
	AdditionalContext string
	// SystemMessage is a warning to show the user.
	SystemMessage string
	// ExitCode is the worst exit code seen (0 = success, 2 = blocking error, -1 = exec failure).
	ExitCode int
	// Stderr captures stderr from a failing hook.
	Stderr string
}
