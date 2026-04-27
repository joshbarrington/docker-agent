package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/team"
)

// TestGetHooksExecutorAutoInjectsBuiltins verifies that the agent's
// AddDate / AddEnvironmentInfo flags are translated into the right
// builtin hook events at executor build time:
//
//   - AddDate is wired into turn_start so the date refreshes per turn.
//   - AddEnvironmentInfo is wired into session_start since wd / OS /
//     arch don't change during a session.
//
// The test exercises behavior end-to-end via Dispatch + AdditionalContext
// rather than inspecting the executor's internals.
func TestGetHooksExecutorAutoInjectsBuiltins(t *testing.T) {
	t.Parallel()

	prov := &mockProvider{id: "test/mock-model", stream: &mockStream{}}

	const dateNeedle = "Today's date: "
	const envNeedle = "running in"

	cases := []struct {
		name           string
		opts           []agent.Opt
		wantNoExecutor bool
		wantTurnStart  string // substring expected in turn_start AdditionalContext, "" means no hooks
		wantSessStart  string // substring expected in session_start AdditionalContext, "" means no hooks
	}{
		{
			name:           "no flags: no implicit hooks, no executor",
			opts:           []agent.Opt{agent.WithModel(prov)},
			wantNoExecutor: true,
		},
		{
			name:          "AddDate only fires on turn_start",
			opts:          []agent.Opt{agent.WithModel(prov), agent.WithAddDate(true)},
			wantTurnStart: dateNeedle + time.Now().Format("2006-01-02"),
		},
		{
			name:          "AddEnvironmentInfo only fires on session_start",
			opts:          []agent.Opt{agent.WithModel(prov), agent.WithAddEnvironmentInfo(true)},
			wantSessStart: envNeedle,
		},
		{
			name: "both flags route to their respective events",
			opts: []agent.Opt{
				agent.WithModel(prov),
				agent.WithAddDate(true),
				agent.WithAddEnvironmentInfo(true),
			},
			wantTurnStart: dateNeedle + time.Now().Format("2006-01-02"),
			wantSessStart: envNeedle,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := agent.New("root", "instructions", tc.opts...)
			tm := team.New(team.WithAgents(a))
			r, err := NewLocalRuntime(tm, WithModelStore(mockModelStore{}))
			require.NoError(t, err)

			exec := r.getHooksExecutor(a)
			if tc.wantNoExecutor {
				assert.Nil(t, exec, "no flags must not produce an executor")
				return
			}
			require.NotNil(t, exec)

			// turn_start
			if tc.wantTurnStart != "" {
				require.True(t, exec.Has(hooks.EventTurnStart),
					"turn_start must be active when AddDate is set")
				res, err := exec.Dispatch(t.Context(), hooks.EventTurnStart, &hooks.Input{
					SessionID: "test-session",
					Cwd:       t.TempDir(),
				})
				require.NoError(t, err)
				assert.True(t, res.Allowed)
				assert.Contains(t, res.AdditionalContext, tc.wantTurnStart)
				// Cross-check: AddEnvironmentInfo must NOT contribute here.
				assert.NotContains(t, res.AdditionalContext, envNeedle,
					"env info must not leak into turn_start output")
			} else {
				assert.False(t, exec.Has(hooks.EventTurnStart),
					"turn_start must be inactive when AddDate is not set")
			}

			// session_start
			if tc.wantSessStart != "" {
				require.True(t, exec.Has(hooks.EventSessionStart),
					"session_start must be active when AddEnvironmentInfo is set")
				res, err := exec.Dispatch(t.Context(), hooks.EventSessionStart, &hooks.Input{
					SessionID: "test-session",
					Cwd:       t.TempDir(),
					Source:    "startup",
				})
				require.NoError(t, err)
				assert.True(t, res.Allowed)
				assert.Contains(t, res.AdditionalContext, tc.wantSessStart)
				// Cross-check: AddDate must NOT contribute here.
				assert.NotContains(t, res.AdditionalContext, dateNeedle,
					"date must not leak into session_start output")
			} else {
				assert.False(t, exec.Has(hooks.EventSessionStart),
					"session_start must be inactive when AddEnvironmentInfo is not set")
			}
		})
	}
}

// TestAddPromptFilesBuiltinReadsViaArgs verifies that AddPromptFiles is
// auto-injected as a turn_start builtin hook with the file list passed
// through Hook.Args, and that the builtin reads each file's contents
// from the input's Cwd. The test pins both the wiring (turn_start, not
// session_start) and the read mechanics (one file -> one chunk in
// AdditionalContext) without round-tripping through RunStream.
func TestAddPromptFilesBuiltinReadsViaArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const (
		promptFile = "PROMPT.md"
		promptBody = "Project guidelines: prefer Go."
	)
	require.NoError(t, os.WriteFile(filepath.Join(dir, promptFile), []byte(promptBody), 0o600))

	prov := &mockProvider{id: "test/mock-model", stream: &mockStream{}}
	a := agent.New("root", "instructions",
		agent.WithModel(prov),
		agent.WithAddPromptFiles([]string{promptFile}),
	)
	tm := team.New(team.WithAgents(a))
	r, err := NewLocalRuntime(tm, WithModelStore(mockModelStore{}))
	require.NoError(t, err)

	exec := r.getHooksExecutor(a)
	require.NotNil(t, exec, "AddPromptFiles must produce an executor")
	require.True(t, exec.Has(hooks.EventTurnStart),
		"AddPromptFiles must be wired to turn_start, not session_start")

	res, err := exec.Dispatch(t.Context(), hooks.EventTurnStart, &hooks.Input{
		SessionID: "test-session",
		Cwd:       dir,
	})
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Contains(t, res.AdditionalContext, promptBody,
		"prompt file body must be reported as additional context")
}
