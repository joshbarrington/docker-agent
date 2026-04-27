// Package hooks provides lifecycle hooks for agent tool execution.
// Hooks allow users to run shell commands or in-process Go functions at
// various points during the agent's execution lifecycle, providing
// deterministic control over agent behavior.
package hooks

import (
	"encoding/json"
	"time"
)

// EventType identifies a hook event.
type EventType string

const (
	// EventPreToolUse fires before a tool call. Can allow/deny/modify it.
	EventPreToolUse EventType = "pre_tool_use"
	// EventPostToolUse fires after a tool completes successfully.
	EventPostToolUse EventType = "post_tool_use"
	// EventSessionStart fires when a session begins or resumes.
	EventSessionStart EventType = "session_start"
	// EventTurnStart fires at the start of every agent turn (each model
	// call). AdditionalContext is injected transiently and never persisted.
	EventTurnStart EventType = "turn_start"
	// EventBeforeLLMCall fires immediately before each model call. Output
	// is informational; use turn_start to contribute system messages.
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
)

// HookType identifies the kind of handler used to run a hook.
type HookType string

const (
	// HookTypeCommand runs a shell command.
	HookTypeCommand HookType = "command"
	// HookTypeBuiltin dispatches to a named in-process Go function
	// registered via [Registry.RegisterBuiltin]. The name is stored in
	// [Hook.Command].
	HookTypeBuiltin HookType = "builtin"
)

// Hook is a single hook configuration entry.
type Hook struct {
	Type    HookType `json:"type" yaml:"type"`
	Command string   `json:"command,omitempty" yaml:"command,omitempty"`
	// Args are arbitrary string arguments passed to the hook handler.
	// Builtin hooks receive them as the args parameter of [BuiltinFunc].
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
	// Timeout is the execution timeout in seconds (default: 60).
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// GetTimeout returns the timeout duration, defaulting to 60 seconds.
func (h *Hook) GetTimeout() time.Duration {
	if h.Timeout <= 0 {
		return 60 * time.Second
	}
	return time.Duration(h.Timeout) * time.Second
}

// MatcherConfig is a hook matcher with its hooks. Matcher is a regex
// pattern matched against tool names; "" or "*" matches all tools.
type MatcherConfig struct {
	Matcher string `json:"matcher,omitempty" yaml:"matcher,omitempty"`
	Hooks   []Hook `json:"hooks" yaml:"hooks"`
}

// Config is the hooks configuration for an agent.
type Config struct {
	PreToolUse      []MatcherConfig `json:"pre_tool_use,omitempty" yaml:"pre_tool_use,omitempty"`
	PostToolUse     []MatcherConfig `json:"post_tool_use,omitempty" yaml:"post_tool_use,omitempty"`
	SessionStart    []Hook          `json:"session_start,omitempty" yaml:"session_start,omitempty"`
	TurnStart       []Hook          `json:"turn_start,omitempty" yaml:"turn_start,omitempty"`
	BeforeLLMCall   []Hook          `json:"before_llm_call,omitempty" yaml:"before_llm_call,omitempty"`
	AfterLLMCall    []Hook          `json:"after_llm_call,omitempty" yaml:"after_llm_call,omitempty"`
	SessionEnd      []Hook          `json:"session_end,omitempty" yaml:"session_end,omitempty"`
	OnUserInput     []Hook          `json:"on_user_input,omitempty" yaml:"on_user_input,omitempty"`
	Stop            []Hook          `json:"stop,omitempty" yaml:"stop,omitempty"`
	Notification    []Hook          `json:"notification,omitempty" yaml:"notification,omitempty"`
	OnError         []Hook          `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	OnMaxIterations []Hook          `json:"on_max_iterations,omitempty" yaml:"on_max_iterations,omitempty"`
}

// IsEmpty returns true if no hooks are configured.
func (c *Config) IsEmpty() bool {
	return len(c.PreToolUse) == 0 && len(c.PostToolUse) == 0 &&
		len(c.SessionStart) == 0 && len(c.TurnStart) == 0 &&
		len(c.BeforeLLMCall) == 0 && len(c.AfterLLMCall) == 0 &&
		len(c.SessionEnd) == 0 && len(c.OnUserInput) == 0 &&
		len(c.Stop) == 0 && len(c.Notification) == 0 &&
		len(c.OnError) == 0 && len(c.OnMaxIterations) == 0
}

// Input is the JSON-serializable payload passed to hooks via stdin.
type Input struct {
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	HookEventName EventType `json:"hook_event_name"`

	// Tool-related fields (PreToolUse and PostToolUse).
	ToolName  string         `json:"tool_name,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`

	// PostToolUse specific.
	ToolResponse any `json:"tool_response,omitempty"`

	// SessionStart specific: "startup", "resume", "clear", "compact".
	Source string `json:"source,omitempty"`
	// SessionEnd specific: "clear", "logout", "prompt_input_exit", "other".
	Reason string `json:"reason,omitempty"`
	// Stop / AfterLLMCall: the model's final response content.
	StopResponse string `json:"stop_response,omitempty"`
	// Notification specific.
	NotificationLevel   string `json:"notification_level,omitempty"`
	NotificationMessage string `json:"notification_message,omitempty"`
}

// ToJSON serializes the input.
func (i *Input) ToJSON() ([]byte, error) { return json.Marshal(i) }

// Decision is a permission decision returned by a hook.
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
	Decision string `json:"decision,omitempty"`
	// Reason explains the decision.
	Reason string `json:"reason,omitempty"`
	// HookSpecificOutput contains event-specific fields.
	HookSpecificOutput *HookSpecificOutput `json:"hook_specific_output,omitempty"`
}

// ShouldContinue reports whether execution should continue.
func (o *Output) ShouldContinue() bool { return o.Continue == nil || *o.Continue }

// IsBlocked reports whether the decision is "block".
func (o *Output) IsBlocked() bool { return o.Decision == "block" }

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
