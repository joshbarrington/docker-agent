package builtins

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/docker/docker-agent/pkg/hooks"
)

// MaxIterations is the registered name of the max_iterations builtin.
const MaxIterations = "max_iterations"

// maxIterationsBuiltin counts before_llm_call invocations per session
// and signals a terminating verdict once the configured limit is
// exceeded.
//
// This is a hard stop with no resume protocol — distinct from the
// agent.MaxIterations flag, which has its own special UX
// (MaxIterationsReachedEvent + a resume dialog) and stays in
// pkg/runtime. Use this builtin to express "stop after N model calls,
// period" in YAML.
//
// Args layout: `[limit]`. Missing or invalid args make the hook a
// no-op so a misconfigured YAML doesn't accidentally cap a run at
// zero. State is per-session, keyed by [hooks.Input.SessionID], and
// cleared from session_end via [State.ClearSession].
type maxIterationsBuiltin struct {
	mu     sync.Mutex
	counts map[string]int // SessionID -> calls observed
}

func newMaxIterations() *maxIterationsBuiltin {
	return &maxIterationsBuiltin{counts: map[string]int{}}
}

func (b *maxIterationsBuiltin) hook(_ context.Context, in *hooks.Input, args []string) (*hooks.Output, error) {
	if in == nil || in.SessionID == "" || len(args) == 0 {
		return nil, nil
	}
	limit, err := strconv.Atoi(args[0])
	if err != nil || limit <= 0 {
		slog.Debug("max_iterations: ignoring invalid limit", "arg", args[0], "error", err)
		return nil, nil
	}

	b.mu.Lock()
	b.counts[in.SessionID]++
	count := b.counts[in.SessionID]
	b.mu.Unlock()

	if count <= limit {
		return nil, nil
	}

	slog.Warn("max_iterations tripped",
		"count", count, "limit", limit, "session_id", in.SessionID)

	return &hooks.Output{
		Decision: hooks.DecisionBlockValue,
		Reason: fmt.Sprintf(
			"Agent terminated: max_iterations builtin reached its limit of %d model call(s).",
			limit),
	}, nil
}

func (b *maxIterationsBuiltin) clearSession(sessionID string) {
	b.mu.Lock()
	delete(b.counts, sessionID)
	b.mu.Unlock()
}
