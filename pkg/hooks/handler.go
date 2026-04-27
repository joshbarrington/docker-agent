package hooks

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/docker/docker-agent/pkg/shellpath"
)

// Handler executes a single hook invocation.
//
// A Handler is built by a [HandlerFactory] for a single [Hook] and is
// invoked at most once. The factory is responsible for digesting the Hook
// (validating it, copying out the fields the handler needs); Run only
// receives the JSON-encoded [Input] on stdin and the timeout-bounded
// context produced by the [Executor].
//
// A Handler MUST NOT apply [Hook.GetTimeout] itself: the executor wraps
// ctx with the timeout before calling Run. A Handler should respect ctx
// cancellation and return promptly.
type Handler interface {
	Run(ctx context.Context, input []byte) (HandlerResult, error)
}

// HandlerResult is the raw result of a single [Handler.Run] call.
//
// A handler can speak to the executor in either of two ways:
//
//   - The classic process protocol: leave Output nil, write JSON (or plain
//     text) to Stdout, signal blocking with ExitCode == 2, etc. The
//     executor parses Stdout into an [Output] when ExitCode == 0 and
//     Stdout begins with '{'. This is what command hooks do.
//
//   - The direct protocol: set Output to a pre-parsed [Output]. In-process
//     handlers use this to skip the JSON round-trip; ExitCode should stay
//     0 and Stdout/Stderr can be left empty.
type HandlerResult struct {
	// Stdout is the raw standard output produced by the handler.
	Stdout string
	// Stderr is the raw standard error produced by the handler.
	Stderr string
	// ExitCode is the handler's exit code. 0 means success; 2 is a
	// blocking error; any other non-zero value is treated as a
	// non-blocking error by the executor.
	ExitCode int
	// Output, when non-nil, short-circuits Stdout JSON parsing. In-process
	// handlers populate this directly.
	Output *Output
}

// HandlerEnv is the per-executor context passed to handler factories.
// It carries everything a handler may need that isn't part of the [Hook]
// definition itself. New fields can be added in the future without
// breaking factory signatures.
type HandlerEnv struct {
	// WorkingDir is the directory in which the handler should run.
	WorkingDir string
	// Env is the environment to expose to the handler, in os.Environ() form.
	Env []string
}

// HandlerFactory builds a [Handler] for a single hook invocation.
//
// Factories are expected to validate the hook (e.g. that a command handler
// has a non-empty [Hook.Command]) and return an error if the hook is not
// runnable. The returned Handler is used for exactly one [Handler.Run]
// call and may be discarded afterwards.
type HandlerFactory func(env HandlerEnv, hook Hook) (Handler, error)

// Registry maps a [HookType] to a [HandlerFactory], and a builtin name
// to a [BuiltinFunc].
//
// A Registry is safe for concurrent use. [NewRegistry] returns a registry
// pre-populated with the two universal handler kinds ("command" and
// "builtin"); embedders register additional handler types or named
// builtin Go functions on it before passing it to
// [NewExecutorWithRegistry].
//
// The process-wide [DefaultRegistry] is one such registry, used by
// [NewExecutor]. It is convenient for callers that don't have any
// runtime-owned builtins of their own; callers that do (e.g. the agent
// runtime) should construct a private registry to keep their builtins
// out of any shared global state.
type Registry struct {
	mu        sync.RWMutex
	factories map[HookType]HandlerFactory
	builtins  map[string]BuiltinFunc
}

// NewRegistry returns a registry pre-populated with the two universal
// handler kinds: [HookTypeCommand] (shell command hooks) and
// [HookTypeBuiltin] (in-process Go callbacks looked up via
// [Registry.RegisterBuiltin]).
func NewRegistry() *Registry {
	r := &Registry{
		factories: map[HookType]HandlerFactory{},
		builtins:  map[string]BuiltinFunc{},
	}
	r.Register(HookTypeCommand, newCommandFactory())
	r.Register(HookTypeBuiltin, r.builtinFactory)
	return r
}

// Register associates a factory with a hook type. A subsequent registration
// of the same type replaces the previous one.
func (r *Registry) Register(t HookType, f HandlerFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[t] = f
}

// Lookup returns the factory registered for t, or (nil, false) if none.
func (r *Registry) Lookup(t HookType) (HandlerFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[t]
	return f, ok
}

// DefaultRegistry is the process-wide registry used by [NewExecutor].
// It is a fully-loaded [NewRegistry] (HookTypeCommand and HookTypeBuiltin
// both wired in) with no named builtins registered. Callers that need
// runtime-owned builtins should construct a private registry rather than
// mutating the default one.
var DefaultRegistry = NewRegistry()

// newCommandFactory returns a [HandlerFactory] for [HookTypeCommand],
// resolving the OS shell once at factory-build time so per-hook
// invocations don't pay the shell-detection cost (which on Windows
// involves filesystem lookups for pwsh.exe / powershell.exe).
func newCommandFactory() HandlerFactory {
	shell, shellArgs := shellpath.DetectShell()
	return func(env HandlerEnv, hook Hook) (Handler, error) {
		if hook.Command == "" {
			return nil, errors.New("command hook requires a non-empty command")
		}
		return &commandHandler{
			workingDir: env.WorkingDir,
			env:        env.Env,
			shell:      shell,
			shellArgs:  shellArgs,
			command:    hook.Command,
		}, nil
	}
}

// commandHandler runs a hook by exec'ing its command under a shell.
// It is the long-standing behavior previously inlined in the executor,
// re-expressed as a [Handler] so that other handler kinds (in-process,
// http, mcp, ...) can be added without touching the executor.
type commandHandler struct {
	workingDir string
	env        []string
	shell      string
	shellArgs  []string
	command    string
}

// Run implements [Handler]. The caller is expected to have wrapped ctx
// with the hook timeout; commandHandler relies on context cancellation
// via [exec.CommandContext] to enforce it.
func (h *commandHandler) Run(ctx context.Context, input []byte) (HandlerResult, error) {
	cmd := exec.CommandContext(ctx, h.shell, append(h.shellArgs, h.command)...)
	cmd.Dir = h.workingDir
	cmd.Env = h.env
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := HandlerResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		// Surface ExitError as a structured exit code; anything else
		// (e.g. the binary couldn't be spawned) bubbles up as a real
		// error and the executor will fail-close PreToolUse calls.
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		res.ExitCode = -1
		return res, err
	}
	return res, nil
}
