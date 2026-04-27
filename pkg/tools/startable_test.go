package tools_test

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/docker/docker-agent/pkg/tools"
)

// stubDescriber implements ToolSet and Describer.
type stubDescriber struct{ desc string }

func (s *stubDescriber) Tools(context.Context) ([]tools.Tool, error) { return nil, nil }
func (s *stubDescriber) Describe() string                            { return s.desc }

// stubToolSet implements ToolSet only (no Describer).
type stubToolSet struct{}

func (s *stubToolSet) Tools(context.Context) ([]tools.Tool, error) { return nil, nil }

// flappyToolSet implements ToolSet + Startable with a scripted sequence of errors.
// Each call to Start() consumes the next error from errs; nil means success.
type flappyToolSet struct {
	errs     []error
	callIdx  int
	startups int // number of successful Start() calls
}

func (f *flappyToolSet) Tools(context.Context) ([]tools.Tool, error) {
	return []tools.Tool{{Name: "flappy_tool"}}, nil
}

func (f *flappyToolSet) Start(_ context.Context) error {
	if f.callIdx < len(f.errs) {
		err := f.errs[f.callIdx]
		f.callIdx++
		if err != nil {
			return err
		}
	}
	f.startups++
	return nil
}

func (f *flappyToolSet) Stop(_ context.Context) error {
	return nil
}

func TestDescribeToolSet_UsesDescriber(t *testing.T) {
	t.Parallel()

	ts := &stubDescriber{desc: "mcp(ref=docker:github-official)"}
	assert.Check(t, is.Equal(tools.DescribeToolSet(ts), "mcp(ref=docker:github-official)"))
}

func TestDescribeToolSet_UnwrapsStartableAndUsesDescriber(t *testing.T) {
	t.Parallel()

	inner := &stubDescriber{desc: "mcp(stdio cmd=python args=-m,srv)"}
	wrapped := tools.NewStartable(inner)
	assert.Check(t, is.Equal(tools.DescribeToolSet(wrapped), "mcp(stdio cmd=python args=-m,srv)"))
}

func TestDescribeToolSet_FallsBackToTypeName(t *testing.T) {
	t.Parallel()

	ts := &stubToolSet{}
	assert.Check(t, is.Equal(tools.DescribeToolSet(ts), "*tools_test.stubToolSet"))
}

func TestDescribeToolSet_FallsBackToTypeNameWhenDescribeEmpty(t *testing.T) {
	t.Parallel()

	ts := &stubDescriber{desc: ""}
	assert.Check(t, is.Equal(tools.DescribeToolSet(ts), "*tools_test.stubDescriber"))
}

func TestDescribeToolSet_UnwrapsStartableAndFallsBackToTypeName(t *testing.T) {
	t.Parallel()

	inner := &stubToolSet{}
	wrapped := tools.NewStartable(inner)
	assert.Check(t, is.Equal(tools.DescribeToolSet(wrapped), "*tools_test.stubToolSet"))
}

// TestStartableToolSet_ShouldReportFailure_OncePerStreak verifies that
// ShouldReportFailure returns true exactly once per failure streak,
// suppressing duplicate warnings on repeated retries.
func TestStartableToolSet_ShouldReportFailure_OncePerStreak(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	f := &flappyToolSet{errs: []error{errBoom, errBoom, nil}}
	s := tools.NewStartable(f)

	// Turn 1: first failure — should report.
	err := s.Start(t.Context())
	assert.Check(t, err != nil, "expected error on turn 1")
	assert.Check(t, is.Equal(s.ShouldReportFailure(), true), "turn 1: first failure should be reported")
	assert.Check(t, is.Equal(s.ShouldReportFailure(), false), "turn 1: second call must return false")

	// Turn 2: second failure in same streak — must NOT report again.
	err = s.Start(t.Context())
	assert.Check(t, err != nil, "expected error on turn 2")
	assert.Check(t, is.Equal(s.ShouldReportFailure(), false), "turn 2: duplicate failure must not report")

	// Turn 3: success — ConsumeRecovery fires exactly once.
	err = s.Start(t.Context())
	assert.Check(t, err == nil, "expected success on turn 3")
	assert.Check(t, is.Equal(s.ConsumeRecovery(), true), "turn 3: recovery must be signalled")
	assert.Check(t, is.Equal(s.ConsumeRecovery(), false), "turn 3: recovery must fire only once")
}

// TestStartableToolSet_NoRecoveryWithoutPriorFailure verifies that
// ConsumeRecovery returns false when Start succeeds on the very first try.
func TestStartableToolSet_NoRecoveryWithoutPriorFailure(t *testing.T) {
	t.Parallel()

	f := &flappyToolSet{errs: []error{nil}}
	s := tools.NewStartable(f)

	err := s.Start(t.Context())
	assert.Check(t, err == nil)
	assert.Check(t, is.Equal(s.ShouldReportFailure(), false), "no failure: ShouldReportFailure must be false")
	assert.Check(t, is.Equal(s.ConsumeRecovery(), false), "no prior failure: ConsumeRecovery must be false")
}

// TestStartableToolSet_RecoveryThenFailureWarnsAgain verifies that after a full
// fail→report→recover cycle, a subsequent new failure generates a fresh warning.
func TestStartableToolSet_RecoveryThenFailureWarnsAgain(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	f := &flappyToolSet{errs: []error{errBoom, nil, errBoom}}
	s := tools.NewStartable(f)

	// Cycle 1: fail then recover.
	err := s.Start(t.Context())
	assert.Check(t, err != nil)
	assert.Check(t, is.Equal(s.ShouldReportFailure(), true))

	err = s.Start(t.Context())
	assert.Check(t, err == nil)
	assert.Check(t, is.Equal(s.ConsumeRecovery(), true))

	// Now stop so we can start again (resets started flag).
	assert.Check(t, s.Stop(t.Context()) == nil)

	// Cycle 2: new failure — must warn again.
	err = s.Start(t.Context())
	assert.Check(t, err != nil)
	assert.Check(t, is.Equal(s.ShouldReportFailure(), true), "fresh failure after recovery must warn")
}

// TestStartableToolSet_StopResetsFailureState verifies that after a failure streak,
// an explicit Stop() clears all tracking so the next failure warns again.
func TestStartableToolSet_StopResetsFailureState(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	f := &flappyToolSet{errs: []error{errBoom, errBoom}}
	s := tools.NewStartable(f)

	// First failure: consume the warning.
	err := s.Start(t.Context())
	assert.Check(t, err != nil)
	assert.Check(t, is.Equal(s.ShouldReportFailure(), true))

	// Stop resets state.
	assert.Check(t, s.Stop(t.Context()) == nil)

	// Second failure after Stop: must warn again.
	err = s.Start(t.Context())
	assert.Check(t, err != nil)
	assert.Check(t, is.Equal(s.ShouldReportFailure(), true), "failure after Stop must produce fresh warning")
}
