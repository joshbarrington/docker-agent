package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHandler is an in-process [Handler] used to prove that the
// executor dispatches to whatever factory the [Registry] returns, not just
// the built-in command handler. It captures the JSON input it received and
// returns a pre-parsed [Output] so the executor's "honor
// HandlerResult.Output as-is" path is also exercised.
type recordingHandler struct {
	calls atomic.Int32
	input atomic.Value // []byte
	out   *Output
}

func (h *recordingHandler) Run(_ context.Context, input []byte) (HandlerResult, error) {
	h.calls.Add(1)
	cp := append([]byte(nil), input...)
	h.input.Store(cp)
	return HandlerResult{Output: h.out}, nil
}

func (h *recordingHandler) capturedInput() []byte {
	v, _ := h.input.Load().([]byte)
	return v
}

// TestExecutorDispatchesToCustomHandler shows the smallest end-to-end use
// of the new pluggability: a custom HookType backed by an in-process Go
// Handler runs through the same executor pipeline as a "command" hook,
// and its pre-parsed Output (a deny verdict here) drives the aggregated
// Result.
func TestExecutorDispatchesToCustomHandler(t *testing.T) {
	t.Parallel()

	const customType HookType = "builtin-test"

	rec := &recordingHandler{
		out: &Output{
			Decision: "block",
			Reason:   "denied by builtin handler",
		},
	}

	registry := NewRegistry()
	registry.Register(customType, func(_ HandlerEnv, _ Hook) (Handler, error) {
		return rec, nil
	})

	config := &Config{
		PreToolUse: []MatcherConfig{
			{
				Matcher: "*",
				Hooks: []Hook{
					{Type: customType, Timeout: 5},
				},
			},
		},
	}

	exec := NewExecutorWithRegistry(config, t.TempDir(), nil, registry)
	input := &Input{
		SessionID: "test-session",
		ToolName:  "shell",
		ToolUseID: "test-id",
	}

	result, err := exec.Dispatch(t.Context(), EventPreToolUse, input)
	require.NoError(t, err)

	// The custom handler ran and saw a properly serialized Input on stdin.
	assert.Equal(t, int32(1), rec.calls.Load())
	var got Input
	require.NoError(t, json.Unmarshal(rec.capturedInput(), &got))
	assert.Equal(t, EventPreToolUse, got.HookEventName)
	assert.Equal(t, "shell", got.ToolName)

	// The pre-parsed Output drove the aggregated Result, so the call is
	// denied with the handler-supplied reason.
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Message, "denied by builtin handler")
}

// TestExecutorUnregisteredTypeIsRejected ensures the registry is the only
// way to plug in a handler: an unknown HookType is surfaced as a hook
// execution error, which (because PreToolUse is a security boundary)
// denies the tool call.
func TestExecutorUnregisteredTypeIsRejected(t *testing.T) {
	t.Parallel()

	config := &Config{
		PreToolUse: []MatcherConfig{
			{
				Matcher: "*",
				Hooks: []Hook{
					{Type: HookType("nope"), Timeout: 5},
				},
			},
		},
	}

	exec := NewExecutorWithRegistry(config, t.TempDir(), nil, NewRegistry())
	input := &Input{
		SessionID: "test-session",
		ToolName:  "shell",
		ToolUseID: "test-id",
	}

	result, err := exec.Dispatch(t.Context(), EventPreToolUse, input)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Message, "unsupported hook type")
}

// TestExecutorHandlerErrorDeniesPreToolUse documents the contract that any
// handler error (not just a non-zero exit) flows into the existing
// fail-closed behavior for PreToolUse.
func TestExecutorHandlerErrorDeniesPreToolUse(t *testing.T) {
	t.Parallel()

	const customType HookType = "always-fails"

	registry := NewRegistry()
	registry.Register(customType, func(_ HandlerEnv, _ Hook) (Handler, error) {
		return handlerFunc(func(context.Context, []byte) (HandlerResult, error) {
			return HandlerResult{ExitCode: -1}, errors.New("kaboom")
		}), nil
	})

	config := &Config{
		PreToolUse: []MatcherConfig{
			{
				Matcher: "*",
				Hooks: []Hook{
					{Type: customType, Timeout: 5},
				},
			},
		},
	}

	exec := NewExecutorWithRegistry(config, t.TempDir(), nil, registry)
	input := &Input{
		SessionID: "test-session",
		ToolName:  "shell",
		ToolUseID: "test-id",
	}

	result, err := exec.Dispatch(t.Context(), EventPreToolUse, input)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, -1, result.ExitCode)
}

// handlerFunc adapts a function value into a [Handler] for terse tests.
type handlerFunc func(context.Context, []byte) (HandlerResult, error)

func (f handlerFunc) Run(ctx context.Context, input []byte) (HandlerResult, error) {
	return f(ctx, input)
}
