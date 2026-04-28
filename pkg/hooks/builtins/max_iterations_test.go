package builtins_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/hooks/builtins"
)

// TestMaxIterationsTripsAfterLimit verifies the happy path: with a
// limit of 3, the first three calls are no-ops and the fourth returns
// a block decision. The reason carries the configured limit so the
// runtime's user-facing Error event explains why the run stopped.
func TestMaxIterationsTripsAfterLimit(t *testing.T) {
	t.Parallel()

	fn := lookup(t, builtins.MaxIterations)
	in := &hooks.Input{SessionID: "s1"}
	args := []string{"3"}

	for i := 1; i <= 3; i++ {
		out, err := fn(t.Context(), in, args)
		require.NoErrorf(t, err, "call %d must not error", i)
		require.Nilf(t, out, "call %d (within limit) must not trip", i)
	}

	out, err := fn(t.Context(), in, args)
	require.NoError(t, err)
	require.NotNil(t, out, "fourth call (over limit) must trip")
	assert.Equal(t, hooks.DecisionBlockValue, out.Decision)
	assert.Contains(t, out.Reason, "3", "reason must include the configured limit")
}

// TestMaxIterationsIsolatesSessions documents the per-session
// counter contract: a runtime serving multiple sessions must not let
// session A's calls count against session B's budget.
func TestMaxIterationsIsolatesSessions(t *testing.T) {
	t.Parallel()

	fn := lookup(t, builtins.MaxIterations)
	args := []string{"2"}

	// Session A: two no-op calls then one trip.
	for range 2 {
		out, err := fn(t.Context(), &hooks.Input{SessionID: "A"}, args)
		require.NoError(t, err)
		require.Nil(t, out)
	}
	out, err := fn(t.Context(), &hooks.Input{SessionID: "A"}, args)
	require.NoError(t, err)
	require.NotNil(t, out, "session A trips on its third call")

	// Session B: starts fresh, only sees one call so far.
	out, err = fn(t.Context(), &hooks.Input{SessionID: "B"}, args)
	require.NoError(t, err)
	require.Nil(t, out, "session B's counter must not include session A's calls")
}

// TestMaxIterationsNoOpWithoutValidLimit documents the lenient-args
// contract: a missing, non-integer, zero, or negative limit makes
// the builtin a no-op rather than tripping (the safer default for a
// misconfigured YAML).
func TestMaxIterationsNoOpWithoutValidLimit(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		nil,
		{},
		{"abc"},
		{"0"},
		{"-1"},
	}
	for _, args := range cases {
		fn := lookup(t, builtins.MaxIterations) // fresh state per case
		// Drive 50 calls — if the builtin were tripping erroneously,
		// at least one of these would return a non-nil Output.
		for range 50 {
			out, err := fn(t.Context(), &hooks.Input{SessionID: "s"}, args)
			require.NoError(t, err)
			require.Nilf(t, out, "args=%v: must never trip", args)
		}
	}
}

// TestMaxIterationsIgnoresIncompleteInput pins the defensive guard:
// missing SessionID produces no state mutation and no output. This
// protects against future dispatch changes where an edge case might
// fire before_llm_call without that field populated.
func TestMaxIterationsIgnoresIncompleteInput(t *testing.T) {
	t.Parallel()

	fn := lookup(t, builtins.MaxIterations)

	out, err := fn(t.Context(), nil, []string{"1"})
	require.NoError(t, err)
	assert.Nil(t, out)

	out, err = fn(t.Context(), &hooks.Input{}, []string{"1"})
	require.NoError(t, err)
	assert.Nil(t, out)
}

// TestMaxIterationsConcurrentCallsAreSafe is a smoke test for the
// builtin's mutex. Many goroutines incrementing the same session's
// counter must not race (run with -race).
func TestMaxIterationsConcurrentCallsAreSafe(t *testing.T) {
	t.Parallel()

	fn := lookup(t, builtins.MaxIterations)
	in := &hooks.Input{SessionID: "concurrent"}

	const callers = 50
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			_, _ = fn(t.Context(), in, []string{"100"})
		}()
	}
	wg.Wait()
}
