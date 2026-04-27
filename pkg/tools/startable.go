package tools

import (
	"context"
	"fmt"
	"sync"
)

// Describer can be implemented by a ToolSet to provide a short, user-visible
// description that uniquely identifies the toolset instance (e.g. for use in
// error messages and warnings). The string must never contain secrets.
type Describer interface {
	Describe() string
}

// DescribeToolSet returns a short description for ts suitable for user-visible
// messages. It unwraps a StartableToolSet, then delegates to Describer if
// implemented. Falls back to the Go type name when not.
func DescribeToolSet(ts ToolSet) string {
	if s, ok := ts.(*StartableToolSet); ok {
		ts = s.ToolSet
	}
	if d, ok := ts.(Describer); ok {
		if desc := d.Describe(); desc != "" {
			return desc
		}
	}
	return fmt.Sprintf("%T", ts)
}

// StartableToolSet wraps a ToolSet with lazy, single-flight start semantics.
// This is the canonical way to manage toolset lifecycle.
//
// Failure and recovery tracking:
//   - freshFailure is set to true on the first Start() failure in a streak
//     (i.e. when hasEverFailed transitions false→true). It is consumed by
//     ShouldReportFailure() which returns true exactly once per streak.
//   - hasEverFailed stays true for the duration of the failure streak.
//   - pendingRecovery is set to true on the first successful Start() after a
//     failure streak. It is consumed by ConsumeRecovery().
//   - ConsumeRecovery() also resets hasEverFailed, so the next failure streak
//     generates a fresh warning.
type StartableToolSet struct {
	ToolSet

	mu              sync.Mutex
	started         bool
	hasEverFailed   bool // true for the duration of a failure streak
	freshFailure    bool // true only for the first failure in a streak; consumed by ShouldReportFailure
	pendingRecovery bool // true when a recovery notice is pending; consumed by ConsumeRecovery
}

// NewStartable wraps a ToolSet for lazy initialization.
func NewStartable(ts ToolSet) *StartableToolSet {
	return &StartableToolSet{ToolSet: ts}
}

// IsStarted returns whether the toolset has been successfully started.
// For toolsets that don't implement Startable, this always returns true.
func (s *StartableToolSet) IsStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// Start starts the toolset with single-flight semantics.
// Concurrent callers block until the start attempt completes.
// If start fails, a future call will retry.
// If the underlying toolset doesn't implement Startable, this is a no-op.
func (s *StartableToolSet) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if startable, ok := As[Startable](s.ToolSet); ok {
		if err := startable.Start(ctx); err != nil {
			// Only set freshFailure on the very first failure in a streak so
			// that repeated failed retries don't each emit a new warning.
			if !s.hasEverFailed {
				s.hasEverFailed = true
				s.freshFailure = true
			}
			return err
		}
	}

	// Successful start: if this followed a failure streak, signal recovery.
	if s.hasEverFailed {
		s.pendingRecovery = true
	}
	s.started = true
	return nil
}

// Stop stops the toolset if it implements Startable and resets
// the started flag so that a subsequent Start will re-initialize.
func (s *StartableToolSet) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started = false
	s.hasEverFailed = false
	s.freshFailure = false
	s.pendingRecovery = false
	if startable, ok := As[Startable](s.ToolSet); ok {
		return startable.Stop(ctx)
	}
	return nil
}

// ShouldReportFailure returns true the first time Start() fails in a new
// failure streak — i.e. when hasEverFailed transitions from false to true.
// It returns false for all subsequent failures in the same streak, preventing
// repeated "start failed" warnings from flooding the user. It is safe to call
// even when Start() did not return an error (it will return false).
func (s *StartableToolSet) ShouldReportFailure() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.freshFailure {
		return false
	}
	s.freshFailure = false
	return true
}

// ConsumeRecovery returns true exactly once after a Start() that succeeded
// following a previously-reported failure streak. Calling it also resets
// hasEverFailed and freshFailure so that a future failure generates a fresh warning.
func (s *StartableToolSet) ConsumeRecovery() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pendingRecovery {
		return false
	}
	s.pendingRecovery = false
	s.hasEverFailed = false
	s.freshFailure = false
	return true
}

// Unwrap returns the underlying ToolSet.
func (s *StartableToolSet) Unwrap() ToolSet {
	return s.ToolSet
}

// Unwrapper is implemented by toolset wrappers that decorate another ToolSet.
// This allows As to walk the wrapper chain and find inner capabilities.
type Unwrapper interface {
	Unwrap() ToolSet
}

// As performs a type assertion on a ToolSet, walking the wrapper chain if needed.
// It checks the outermost toolset first, then recursively unwraps through any
// Unwrapper implementations (including StartableToolSet and decorator wrappers)
// until it finds a match or reaches the end of the chain.
//
// Example:
//
//	if pp, ok := tools.As[tools.PromptProvider](toolset); ok {
//	    prompts, _ := pp.ListPrompts(ctx)
//	}
func As[T any](ts ToolSet) (T, bool) {
	for ts != nil {
		if result, ok := ts.(T); ok {
			return result, true
		}
		if u, ok := ts.(Unwrapper); ok {
			ts = u.Unwrap()
		} else {
			break
		}
	}
	var zero T
	return zero, false
}
