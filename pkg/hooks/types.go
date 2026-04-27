// Package hooks provides lifecycle hooks for agent tool execution.
// Hooks allow users to run shell commands or prompts at various points
// during the agent's execution lifecycle, providing deterministic control
// over agent behavior.
package hooks

import (
	"encoding/json"
	"time"
)

// EventType represents the type of hook event
type EventType string

const (
	// EventPreToolUse is triggered before a tool call executes.
	// Can allow/deny/modify tool calls; can block with feedback.
	EventPreToolUse EventType = "pre_tool_use"

	// EventPostToolUse is triggered after a tool completes successfully.
	// Can provide validation, feedback, or additional processing.
	EventPostToolUse EventType = "post_tool_use"

	// EventSessionStart is triggered when a session begins or resumes.
	// Can load context, setup environment, install dependencies.
	EventSessionStart EventType = "session_start"

	// EventTurnStart is triggered at the start of every agent turn (each
	// model call), AFTER the persisted messages are read but BEFORE the
	// model is invoked. Its AdditionalContext is appended as transient
	// system messages for that turn only — it is NOT persisted to the
	// session, so per-turn signals (date, environment, prompt files) are
	// recomputed every turn instead of bloating the message history on
	// every resume.
	EventTurnStart EventType = "turn_start"

	// EventBeforeLLMCall is triggered immediately before each model call,
	// AFTER turn_start has assembled the messages slice. Use this for
	// observability, cost guardrails, or auditing without contributing
	// system messages — turn_start is the right event for the latter.
	// The hook output is currently informational; a future extension may
	// honor a deny verdict to short-circuit the call.
	EventBeforeLLMCall EventType = "before_llm_call"

	// EventAfterLLMCall is triggered immediately after a successful model
	// call, BEFORE the response is recorded into the session and before
	// any tool calls are dispatched. Receives the assistant text content
	// in stop_response (matching the stop event). Failed model calls
	// fire EventOnError instead.
	EventAfterLLMCall EventType = "after_llm_call"

	// EventSessionEnd is triggered when a session terminates.
	// Can perform cleanup, logging, persist session state.
	EventSessionEnd EventType = "session_end"

	// EventOnUserInput is triggered when the agent needs input from the user.
	// Can log, notify, or perform actions before user interaction.
	EventOnUserInput EventType = "on_user_input"

	// EventStop is triggered when the model finishes its response and is about
	// to hand control back to the user. Can perform post-response validation,
	// logging, or cleanup.
	EventStop EventType = "stop"

	// EventNotification is triggered when the agent emits a notification to the user,
	// such as errors or warnings. Can send external notifications or log events.
	EventNotification EventType = "notification"

	// EventOnError is triggered specifically when the runtime hits an
	// error during a turn (model failures, repetitive tool-call loops).
	// Fires alongside EventNotification (level="error") so existing
	// notification hooks keep working; on_error gives a structured entry
	// point for users who want to react only to errors.
	EventOnError EventType = "on_error"

	// EventOnMaxIterations is triggered when the runtime reaches its
	// configured max_iterations limit. Fires alongside EventNotification
	// (level="warning") so existing notification hooks keep working;
	// on_max_iterations gives a structured entry point for users who
	// want to react only to that condition (e.g. log to a metrics
	// pipeline rather than a chat channel).
	EventOnMaxIterations EventType = "on_max_iterations"
)

// HookType represents the type of hook action
type HookType string

const (
	// HookTypeCommand executes a shell command
	HookTypeCommand HookType = "command"
)

// Hook represents a single hook configuration
type Hook struct {
	// Type specifies whether this is a command or prompt hook
	Type HookType `json:"type" yaml:"type"`

	// Command is the shell command to execute (for command hooks)
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Args are arbitrary string arguments passed to the hook handler.
	// They are interpreted by the handler kind: builtin hooks receive
	// them as the args parameter of [BuiltinFunc]; future handler kinds
	// (http, mcp, ...) can adopt the same field.
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`

	// Timeout is the execution timeout in seconds (default: 60)
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// GetTimeout returns the timeout duration, defaulting to 60 seconds
func (h *Hook) GetTimeout() time.Duration {
	if h.Timeout <= 0 {
		return 60 * time.Second
	}
	return time.Duration(h.Timeout) * time.Second
}

// MatcherConfig represents a hook matcher with its hooks
type MatcherConfig struct {
	// Matcher is a regex pattern to match tool names (e.g., "shell|edit_file")
	// Use "*" to match all tools
	Matcher string `json:"matcher,omitempty" yaml:"matcher,omitempty"`

	// Hooks are the hooks to execute when the matcher matches
	Hooks []Hook `json:"hooks" yaml:"hooks"`
}

// Config represents the hooks configuration for an agent
type Config struct {
	// PreToolUse hooks run before tool execution
	PreToolUse []MatcherConfig `json:"pre_tool_use,omitempty" yaml:"pre_tool_use,omitempty"`

	// PostToolUse hooks run after tool execution
	PostToolUse []MatcherConfig `json:"post_tool_use,omitempty" yaml:"post_tool_use,omitempty"`

	// SessionStart hooks run when a session begins
	SessionStart []Hook `json:"session_start,omitempty" yaml:"session_start,omitempty"`

	// TurnStart hooks run at the start of every agent turn (each model
	// call). AdditionalContext is injected transiently and never
	// persisted to the session.
	TurnStart []Hook `json:"turn_start,omitempty" yaml:"turn_start,omitempty"`

	// BeforeLLMCall hooks run just before each model call (after
	// turn_start). Output is informational; use turn_start to
	// contribute system messages.
	BeforeLLMCall []Hook `json:"before_llm_call,omitempty" yaml:"before_llm_call,omitempty"`

	// AfterLLMCall hooks run just after each successful model call,
	// before the response is recorded and tool calls dispatched.
	AfterLLMCall []Hook `json:"after_llm_call,omitempty" yaml:"after_llm_call,omitempty"`

	// SessionEnd hooks run when a session ends
	SessionEnd []Hook `json:"session_end,omitempty" yaml:"session_end,omitempty"`

	// OnUserInput hooks run when the agent needs user input
	OnUserInput []Hook `json:"on_user_input,omitempty" yaml:"on_user_input,omitempty"`

	// Stop hooks run when the model finishes responding
	Stop []Hook `json:"stop,omitempty" yaml:"stop,omitempty"`

	// Notification hooks run when the agent sends a notification (error, warning) to the user
	Notification []Hook `json:"notification,omitempty" yaml:"notification,omitempty"`

	// OnError hooks run when the runtime hits an error during a turn
	// (model failures, repetitive tool-call loops). Fires alongside
	// Notification with level="error".
	OnError []Hook `json:"on_error,omitempty" yaml:"on_error,omitempty"`

	// OnMaxIterations hooks run when the runtime reaches its configured
	// max_iterations limit. Fires alongside Notification with
	// level="warning".
	OnMaxIterations []Hook `json:"on_max_iterations,omitempty" yaml:"on_max_iterations,omitempty"`
}

// IsEmpty returns true if no hooks are configured
func (c *Config) IsEmpty() bool {
	return len(c.PreToolUse) == 0 &&
		len(c.PostToolUse) == 0 &&
		len(c.SessionStart) == 0 &&
		len(c.TurnStart) == 0 &&
		len(c.BeforeLLMCall) == 0 &&
		len(c.AfterLLMCall) == 0 &&
		len(c.SessionEnd) == 0 &&
		len(c.OnUserInput) == 0 &&
		len(c.Stop) == 0 &&
		len(c.Notification) == 0 &&
		len(c.OnError) == 0 &&
		len(c.OnMaxIterations) == 0
}

// Input represents the JSON input passed to hooks via stdin
type Input struct {
	// Common fields for all hooks
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	HookEventName EventType `json:"hook_event_name"`

	// Tool-related fields (for PreToolUse and PostToolUse)
	ToolName  string         `json:"tool_name,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`

	// PostToolUse specific
	ToolResponse any `json:"tool_response,omitempty"`

	// SessionStart specific
	Source string `json:"source,omitempty"` // "startup", "resume", "clear", "compact"

	// SessionEnd specific
	Reason string `json:"reason,omitempty"` // "clear", "logout", "prompt_input_exit", "other"

	// Stop specific
	StopResponse string `json:"stop_response,omitempty"` // The model's final response content

	// Notification specific
	NotificationLevel   string `json:"notification_level,omitempty"`   // "error" or "warning"
	NotificationMessage string `json:"notification_message,omitempty"` // The notification content
}

// ToJSON serializes the input to JSON
func (i *Input) ToJSON() ([]byte, error) {
	return json.Marshal(i)
}

// Decision represents a permission decision from a hook
type Decision string

const (
	// DecisionAllow allows the operation to proceed
	DecisionAllow Decision = "allow"

	// DecisionDeny blocks the operation
	DecisionDeny Decision = "deny"

	// DecisionAsk prompts the user for confirmation (PreToolUse only)
	DecisionAsk Decision = "ask"
)

// Output represents the JSON output from a hook
type Output struct {
	// Continue indicates whether to continue execution (default: true)
	Continue *bool `json:"continue,omitempty"`

	// StopReason is the message to show when continue=false
	StopReason string `json:"stop_reason,omitempty"`

	// SuppressOutput hides stdout from transcript
	SuppressOutput bool `json:"suppress_output,omitempty"`

	// SystemMessage is a warning to show the user
	SystemMessage string `json:"system_message,omitempty"`

	// Decision is for blocking operations
	Decision string `json:"decision,omitempty"`

	// Reason is the message explaining the decision
	Reason string `json:"reason,omitempty"`

	// HookSpecificOutput contains event-specific fields
	HookSpecificOutput *HookSpecificOutput `json:"hook_specific_output,omitempty"`
}

// ShouldContinue returns whether execution should continue
func (o *Output) ShouldContinue() bool {
	if o.Continue == nil {
		return true
	}
	return *o.Continue
}

// IsBlocked returns true if the decision is to block
func (o *Output) IsBlocked() bool {
	return o.Decision == "block"
}

// HookSpecificOutput contains event-specific output fields
type HookSpecificOutput struct {
	// HookEventName identifies which event this output is for
	HookEventName EventType `json:"hook_event_name,omitempty"`

	// PreToolUse fields
	PermissionDecision       Decision       `json:"permission_decision,omitempty"`
	PermissionDecisionReason string         `json:"permission_decision_reason,omitempty"`
	UpdatedInput             map[string]any `json:"updated_input,omitempty"`

	// PostToolUse/SessionStart fields
	AdditionalContext string `json:"additional_context,omitempty"`
}

// Result represents the result of executing hooks
type Result struct {
	// Allowed indicates if the operation should proceed
	Allowed bool

	// Message is feedback to include in the response
	Message string

	// ModifiedInput contains any modifications to tool input (PreToolUse only)
	ModifiedInput map[string]any

	// AdditionalContext is context to add (PostToolUse/SessionStart)
	AdditionalContext string

	// SystemMessage is a warning to show the user
	SystemMessage string

	// ExitCode is the exit code from the hook command (0 = success, 2 = blocking error)
	ExitCode int

	// Stderr contains any error output from the hook
	Stderr string
}
