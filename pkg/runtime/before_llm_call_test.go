package runtime

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/team"
)

// TestBeforeLLMCallHookFiresOncePerLoopIteration is a regression test
// for a duplicate dispatch in [LocalRuntime.RunStream] that fired
// [LocalRuntime.executeBeforeLLMCallHooks] twice per iteration. The
// bug would silently break stateful before_llm_call hooks (the
// max_iterations builtin would have tripped at half its configured
// limit). A single-turn session must observe exactly one fire.
func TestBeforeLLMCallHookFiresOncePerLoopIteration(t *testing.T) {
	t.Parallel()

	const counterName = "test-before-llm-counter"
	var calls atomic.Int32

	stream := newStreamBuilder().
		AddContent("Hello").
		AddStopWithUsage(3, 2).
		Build()
	prov := &mockProvider{id: "test/mock-model", stream: stream}

	root := agent.New("root", "test agent",
		agent.WithModel(prov),
		agent.WithHooks(&latest.HooksConfig{
			BeforeLLMCall: []latest.HookDefinition{
				{Type: "builtin", Command: counterName},
			},
		}),
	)
	tm := team.New(team.WithAgents(root))

	rt, err := NewLocalRuntime(tm,
		WithSessionCompaction(false),
		WithModelStore(mockModelStore{}),
	)
	require.NoError(t, err)

	// Builtin lookup happens at dispatch time, not at executor build,
	// so registering after NewLocalRuntime is sufficient.
	require.NoError(t, rt.hooksRegistry.RegisterBuiltin(
		counterName,
		func(_ context.Context, _ *hooks.Input, _ []string) (*hooks.Output, error) {
			calls.Add(1)
			return nil, nil
		},
	))

	sess := session.New(session.WithUserMessage("hi"))
	sess.Title = "Unit Test"

	for range rt.RunStream(t.Context(), sess) {
	}

	assert.Equal(t, int32(1), calls.Load(),
		"before_llm_call must fire exactly once per loop iteration; "+
			"a duplicate dispatch would silently break stateful hooks like max_iterations")
}
