package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// HookTypeBuiltin dispatches to a named, in-process Go function registered
// via [Registry.RegisterBuiltin]. The function name is stored in
// [Hook.Command], so we don't need a new YAML key — a builtin hook is
// written as `{type: builtin, command: "<name>"}`.
const HookTypeBuiltin HookType = "builtin"

// BuiltinFunc is the signature of an in-process hook handler. It receives
// the parsed [Input] (no JSON unmarshaling on the user's side), any
// per-hook arguments declared as [Hook.Args] in the YAML, and returns a
// parsed [Output], short-circuiting the stdout-as-JSON protocol that
// command hooks rely on.
//
// Returning a nil Output is fine — it produces a successful no-op result
// and is useful for fire-and-forget handlers (logging, telemetry, ...).
type BuiltinFunc func(ctx context.Context, in *Input, args []string) (*Output, error)

// RegisterBuiltin makes fn callable as `{type: builtin, command: name}`
// on this registry.
//
// Subsequent registrations of the same name replace the previous one,
// matching the [Registry.Register] contract. Empty name or nil fn are
// rejected.
//
// RegisterBuiltin is safe for concurrent use, but typical callers
// register from package-level setup or a runtime constructor.
func (r *Registry) RegisterBuiltin(name string, fn BuiltinFunc) error {
	if name == "" {
		return errors.New("builtin hook name must not be empty")
	}
	if fn == nil {
		return errors.New("builtin hook function must not be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins[name] = fn
	return nil
}

// LookupBuiltin returns the function registered as name, or (nil, false).
// Exported primarily for tests and tooling that want to enumerate or
// validate builtins without going through the [Executor].
func (r *Registry) LookupBuiltin(name string) (BuiltinFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.builtins[name]
	return fn, ok
}

// builtinFactory is the [HandlerFactory] for [HookTypeBuiltin]. It looks
// up the function by [Hook.Command] in the registry's builtin table and
// wraps it in a [Handler] that bridges the JSON-encoded executor input
// to the typed [BuiltinFunc] signature, capturing the hook's [Hook.Args]
// for the handler to receive at Run time.
//
// The factory is bound to the registry as a method value in [NewRegistry]
// so each registry resolves names against its own builtin table.
func (r *Registry) builtinFactory(_ HandlerEnv, hook Hook) (Handler, error) {
	name := hook.Command
	if name == "" {
		return nil, errors.New("builtin hook requires a name in command")
	}
	fn, ok := r.LookupBuiltin(name)
	if !ok {
		return nil, fmt.Errorf("no builtin hook registered as %q", name)
	}
	return &builtinHandler{fn: fn, args: hook.Args}, nil
}

// builtinHandler bridges the executor's JSON-on-stdin protocol to a typed
// [BuiltinFunc]. Decoding errors fail-close (-1) so PreToolUse hooks deny
// rather than silently allow.
type builtinHandler struct {
	fn   BuiltinFunc
	args []string
}

func (h *builtinHandler) Run(ctx context.Context, input []byte) (HandlerResult, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return HandlerResult{ExitCode: -1}, fmt.Errorf("decode hook input: %w", err)
	}
	out, err := h.fn(ctx, &in, h.args)
	if err != nil {
		return HandlerResult{ExitCode: -1}, err
	}
	return HandlerResult{Output: out}, nil
}
