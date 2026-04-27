package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/concurrent"
	"github.com/docker/docker-agent/pkg/model/provider/base"
	"github.com/docker/docker-agent/pkg/tools"
)

type stubToolSet struct {
	startErr error
	tools    []tools.Tool
	listErr  error
}

// Verify interface compliance
var (
	_ tools.ToolSet   = (*stubToolSet)(nil)
	_ tools.Startable = (*stubToolSet)(nil)
)

func newStubToolSet(startErr error, toolsList []tools.Tool, listErr error) tools.ToolSet {
	return &stubToolSet{
		startErr: startErr,
		tools:    toolsList,
		listErr:  listErr,
	}
}

func (s *stubToolSet) Start(context.Context) error { return s.startErr }
func (s *stubToolSet) Stop(context.Context) error  { return nil }
func (s *stubToolSet) Tools(context.Context) ([]tools.Tool, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.tools, nil
}

// flappyToolSet is a ToolSet+Startable that returns a scripted sequence of
// errors from Start(). nil in the sequence means success.
type flappyToolSet struct {
	errs    []error
	callIdx int
	stubs   []tools.Tool
}

var (
	_ tools.ToolSet   = (*flappyToolSet)(nil)
	_ tools.Startable = (*flappyToolSet)(nil)
)

func (f *flappyToolSet) Start(_ context.Context) error {
	if f.callIdx >= len(f.errs) {
		return nil
	}
	err := f.errs[f.callIdx]
	f.callIdx++
	return err
}

func (f *flappyToolSet) Stop(_ context.Context) error { return nil }

func (f *flappyToolSet) Tools(_ context.Context) ([]tools.Tool, error) {
	return f.stubs, nil
}

func TestAgentTools(t *testing.T) {
	tests := []struct {
		name          string
		toolsets      []tools.ToolSet
		wantToolCount int
		wantWarnings  int
	}{
		{
			name:          "partial success",
			toolsets:      []tools.ToolSet{newStubToolSet(nil, []tools.Tool{{Name: "good", Parameters: map[string]any{}}}, nil), newStubToolSet(errors.New("boom"), nil, nil)},
			wantToolCount: 1,
			wantWarnings:  1,
		},
		{
			name:          "all fail on start",
			toolsets:      []tools.ToolSet{newStubToolSet(errors.New("fail1"), nil, nil), newStubToolSet(errors.New("fail2"), nil, nil)},
			wantToolCount: 0,
			wantWarnings:  2,
		},
		{
			name:          "list failure becomes warning",
			toolsets:      []tools.ToolSet{newStubToolSet(nil, nil, errors.New("list boom"))},
			wantToolCount: 0,
			wantWarnings:  1,
		},
		{
			name:          "no toolsets",
			toolsets:      nil,
			wantToolCount: 0,
			wantWarnings:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New("root", "test", WithToolSets(tt.toolsets...))
			got, err := a.Tools(t.Context())

			require.NoError(t, err)
			require.Len(t, got, tt.wantToolCount)

			warnings := a.DrainWarnings()
			if tt.wantWarnings == 0 {
				require.Nil(t, warnings)
			} else {
				require.Len(t, warnings, tt.wantWarnings)
			}
		})
	}
}

// mockProvider implements provider.Provider for testing
type mockProvider struct {
	id string
}

func (m *mockProvider) ID() string { return m.id }
func (m *mockProvider) CreateChatCompletionStream(_ context.Context, _ []chat.Message, _ []tools.Tool) (chat.MessageStream, error) {
	return nil, nil
}
func (m *mockProvider) BaseConfig() base.Config { return base.Config{} }

func TestModelOverride(t *testing.T) {
	t.Parallel()

	defaultModel := &mockProvider{id: "openai/gpt-4o"}
	overrideModel := &mockProvider{id: "anthropic/claude-sonnet-4-0"}

	a := New("root", "test", WithModel(defaultModel))

	// Initially should return the default model
	assert.Equal(t, "openai/gpt-4o", a.Model().ID())
	assert.False(t, a.HasModelOverride())

	// Set an override
	a.SetModelOverride(overrideModel)
	assert.True(t, a.HasModelOverride())
	assert.Equal(t, "anthropic/claude-sonnet-4-0", a.Model().ID())

	// ConfiguredModels still reflects the originally configured models
	configuredModels := a.ConfiguredModels()
	require.Len(t, configuredModels, 1)
	assert.Equal(t, "openai/gpt-4o", configuredModels[0].ID())

	// Clear the override
	a.SetModelOverride(nil)
	assert.False(t, a.HasModelOverride())
	assert.Equal(t, "openai/gpt-4o", a.Model().ID())
}

func TestSetModelOverride_ReturnsSnapshotOfStoredValue(t *testing.T) {
	// SetModelOverride must return a snapshot of the value it just stored,
	// not what a subsequent SnapshotModelOverride() would load. This is the
	// guarantee that closes the race window for scoped overrides: if a
	// concurrent caller stores a different override after our store but
	// before we capture our snapshot, our snapshot must still refer to
	// what we stored, so the deferred CAS-restore will fail (concurrent
	// change wins) instead of incorrectly succeeding.
	t.Parallel()

	defaultModel := &mockProvider{id: "default"}
	oursModel := &mockProvider{id: "ours"}
	othersModel := &mockProvider{id: "others"}

	a := New("root", "test", WithModel(defaultModel))

	// Capture the snapshot returned by SetModelOverride.
	prev := a.SnapshotModelOverride()
	oursSnap := a.SetModelOverride(oursModel)

	// Simulate a concurrent caller storing a different override _after_ we
	// stored ours but _before_ a hypothetical post-store SnapshotModelOverride.
	a.SetModelOverride(othersModel)
	require.Equal(t, "others", a.Model().ID())

	// The deferred restore must be a no-op because oursSnap holds the
	// pointer we stored, not the current pointer.
	a.RestoreModelOverride(prev, oursSnap)
	assert.Equal(t, "others", a.Model().ID(),
		"concurrent override must be preserved; the snapshot returned by SetModelOverride captures the stored pointer")
}

func TestSetModelOverride_ClearReturnsZeroSnapshot(t *testing.T) {
	t.Parallel()

	a := New("root", "test", WithModel(&mockProvider{id: "default"}))

	// Calling SetModelOverride with no providers (or nil) clears the override.
	// The returned snapshot should round-trip cleanly through RestoreModelOverride.
	cleared := a.SetModelOverride()
	assert.False(t, a.HasModelOverride())

	// Now set an override and restore using `cleared` as `prev`.
	oursSnap := a.SetModelOverride(&mockProvider{id: "ours"})
	require.True(t, a.HasModelOverride())

	a.RestoreModelOverride(cleared, oursSnap)
	assert.False(t, a.HasModelOverride(), "restoring a cleared snapshot must clear the override")
}

func TestSnapshotAndRestoreModelOverride(t *testing.T) {
	t.Parallel()

	defaultModel := &mockProvider{id: "openai/gpt-4o"}
	skillModel := &mockProvider{id: "openai/gpt-4o-mini"}
	userModel := &mockProvider{id: "anthropic/claude-sonnet-4-0"}

	t.Run("restores when no concurrent change", func(t *testing.T) {
		t.Parallel()
		a := New("root", "test", WithModel(defaultModel))

		prev := a.SnapshotModelOverride()
		a.SetModelOverride(skillModel)
		ours := a.SnapshotModelOverride()
		assert.Equal(t, "openai/gpt-4o-mini", a.Model().ID())

		a.RestoreModelOverride(prev, ours)
		assert.False(t, a.HasModelOverride())
		assert.Equal(t, "openai/gpt-4o", a.Model().ID())
	})

	t.Run("restores back to a pre-existing override", func(t *testing.T) {
		t.Parallel()
		a := New("root", "test", WithModel(defaultModel))
		a.SetModelOverride(userModel)

		prev := a.SnapshotModelOverride()
		a.SetModelOverride(skillModel)
		ours := a.SnapshotModelOverride()
		assert.Equal(t, "openai/gpt-4o-mini", a.Model().ID())

		a.RestoreModelOverride(prev, ours)
		assert.Equal(t, "anthropic/claude-sonnet-4-0", a.Model().ID())
	})

	t.Run("keeps a concurrent change instead of restoring", func(t *testing.T) {
		// This is the TUI-while-skill-runs scenario: another caller
		// changes the override between SnapshotModelOverride and
		// RestoreModelOverride. The deferred restore must NOT clobber
		// that change.
		t.Parallel()
		a := New("root", "test", WithModel(defaultModel))

		prev := a.SnapshotModelOverride()
		a.SetModelOverride(skillModel)
		ours := a.SnapshotModelOverride()

		// Simulate concurrent TUI model switch.
		a.SetModelOverride(userModel)

		a.RestoreModelOverride(prev, ours)
		require.True(t, a.HasModelOverride(), "user's model choice must be preserved")
		assert.Equal(t, "anthropic/claude-sonnet-4-0", a.Model().ID())
	})

	t.Run("keeps a concurrent clear instead of restoring", func(t *testing.T) {
		// Same as above but the concurrent caller clears the override
		// (e.g. user revert via TUI). The restore must respect that.
		t.Parallel()
		a := New("root", "test", WithModel(defaultModel))
		a.SetModelOverride(userModel)

		prev := a.SnapshotModelOverride()
		a.SetModelOverride(skillModel)
		ours := a.SnapshotModelOverride()

		// Simulate concurrent TUI revert.
		a.SetModelOverride()

		a.RestoreModelOverride(prev, ours)
		assert.False(t, a.HasModelOverride(), "user's revert must be preserved")
	})
}

func TestModel_LogsSelection(t *testing.T) {
	t.Parallel()

	var buf concurrent.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })

	model1 := &mockProvider{id: "anthropic/claude-sonnet-4-0"}
	model2 := &mockProvider{id: "openai/gpt-4o"}

	a := New("scanner", "test", WithModel(model1), WithModel(model2))

	// Verify basic selection logging
	selected := a.Model()
	logOutput := buf.String()

	assert.Contains(t, logOutput, "Model selected")
	assert.Contains(t, logOutput, "agent=scanner")
	assert.Contains(t, logOutput, selected.ID())
	assert.Contains(t, logOutput, "pool_size=2")

	// Verify override scenario logs correct pool_size
	buf.Reset()
	override := &mockProvider{id: "google/gemini-2.0-flash"}
	a.SetModelOverride(override)

	selected = a.Model()
	logOutput = buf.String()

	assert.Equal(t, "google/gemini-2.0-flash", selected.ID())
	assert.Contains(t, logOutput, "google/gemini-2.0-flash")
	assert.Contains(t, logOutput, "pool_size=1")
}

func TestModelOverride_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	defaultModel := &mockProvider{id: "default"}
	overrideModel := &mockProvider{id: "override"}

	a := New("root", "test", WithModel(defaultModel))

	// Run concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for range 100 {
			a.SetModelOverride(overrideModel)
			a.SetModelOverride(nil)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = a.Model()
			_ = a.HasModelOverride()
		}
		done <- true
	}()

	<-done
	<-done
	// If we got here without a race condition panic, the test passes
}

// TestAgentReProbeEmitsWarningThenNotice verifies the full retry lifecycle:
// turn 1 fails → warning emitted; turn 2 succeeds → notice emitted; tools available.
func TestAgentReProbeEmitsWarningThenNotice(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("server unavailable")
	stub := &flappyToolSet{
		errs:  []error{errBoom, nil},
		stubs: []tools.Tool{{Name: "mcp_ping", Parameters: map[string]any{}}},
	}
	a := New("root", "test", WithToolSets(stub))

	// Turn 1: start fails → 1 warning, 0 tools.
	got, err := a.Tools(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got, "turn 1: no tools while toolset is unavailable")
	warnings := a.DrainWarnings()
	require.Len(t, warnings, 1, "turn 1: exactly one warning expected")
	assert.Contains(t, warnings[0], "start failed")

	// Turn 2: start succeeds → 1 recovery warning, tools available.
	got, err = a.Tools(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, 1, "turn 2: tool should be available after recovery")
	recovery := a.DrainWarnings()
	require.Len(t, recovery, 1, "turn 2: exactly one recovery warning expected")
	assert.Contains(t, recovery[0], "now available", "turn 2: recovery warning must mention availability")
}

// TestAgentNoDuplicateStartWarnings verifies that repeated failures generate
// only one warning (on the first failure), not one per retry.
func TestAgentNoDuplicateStartWarnings(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("server unavailable")
	stub := &flappyToolSet{
		errs:  []error{errBoom, errBoom, errBoom},
		stubs: []tools.Tool{{Name: "mcp_ping", Parameters: map[string]any{}}},
	}
	a := New("root", "test", WithToolSets(stub))

	// Turn 1: first failure → warning.
	_, err := a.Tools(t.Context())
	require.NoError(t, err)
	warnings := a.DrainWarnings()
	require.Len(t, warnings, 1, "turn 1: exactly one warning on first failure")

	// Turn 2: repeated failure → no new warning.
	_, err = a.Tools(t.Context())
	require.NoError(t, err)
	assert.Empty(t, a.DrainWarnings(), "turn 2: no duplicate warning on repeated failure")

	// Turn 3: still failing → still no new warning.
	_, err = a.Tools(t.Context())
	require.NoError(t, err)
	assert.Empty(t, a.DrainWarnings(), "turn 3: no duplicate warning on repeated failure")
}

// TestAgentWarningsConcurrentAccess exercises the warnings queue from
// multiple goroutines to catch regressions in locking. Run with -race to
// actually detect a regression.
func TestAgentWarningsConcurrentAccess(t *testing.T) {
	t.Parallel()

	a := New("root", "test")

	const writers = 8
	const drainers = 4
	const perWriter = 200

	var wg sync.WaitGroup
	wg.Add(writers + drainers)

	for range writers {
		go func() {
			defer wg.Done()
			for range perWriter {
				a.addToolWarning("boom")
			}
		}()
	}

	stop := make(chan struct{})
	for range drainers {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					// One final drain so we can assert a total count.
					_ = a.DrainWarnings()
					return
				default:
					_ = a.DrainWarnings()
				}
			}
		}()
	}

	// Give writers a little time to finish, then signal drainers to stop.
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()

	// A successful run means no data race and no panic; we don't assert a
	// specific number of warnings drained because drainers run concurrently
	// with writers.
}
