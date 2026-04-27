package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker-agent/pkg/agent"
	"github.com/docker/docker-agent/pkg/backoff"
	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/model/provider"
	"github.com/docker/docker-agent/pkg/modelerrors"
	"github.com/docker/docker-agent/pkg/modelsdev"
	"github.com/docker/docker-agent/pkg/session"
	"github.com/docker/docker-agent/pkg/tools"
)

// modelWithFallback holds a provider and its identification for logging
type modelWithFallback struct {
	provider   provider.Provider
	isFallback bool
	index      int // index in fallback list (-1 for primary)
}

// buildModelChain returns the ordered list of models to try: primary first, then fallbacks.
func buildModelChain(primary provider.Provider, fallbacks []provider.Provider) []modelWithFallback {
	chain := make([]modelWithFallback, 0, 1+len(fallbacks))
	chain = append(chain, modelWithFallback{
		provider:   primary,
		isFallback: false,
		index:      -1,
	})
	for i, fb := range fallbacks {
		chain = append(chain, modelWithFallback{
			provider:   fb,
			isFallback: true,
			index:      i,
		})
	}
	return chain
}

// logFallbackAttempt logs information about a fallback attempt
func logFallbackAttempt(agentName string, model modelWithFallback, attempt, maxRetries int, err error) {
	if model.isFallback {
		slog.Warn("Fallback model attempt",
			"agent", agentName,
			"model", model.provider.ID(),
			"fallback_index", model.index,
			"attempt", attempt+1,
			"max_retries", maxRetries+1,
			"previous_error", err)
	} else {
		slog.Warn("Primary model failed, trying fallbacks",
			"agent", agentName,
			"model", model.provider.ID(),
			"error", err)
	}
}

// logRetryBackoff logs when we're backing off before a retry
func logRetryBackoff(agentName, modelID string, attempt int, backoffDelay time.Duration) {
	slog.Debug("Backing off before retry",
		"agent", agentName,
		"model", modelID,
		"attempt", attempt+1,
		"backoff", backoffDelay)
}

// getEffectiveCooldown returns the cooldown duration to use for an agent.
// Uses the agent's configured cooldown, or the default if not set.
func getEffectiveCooldown(a *agent.Agent) time.Duration {
	cooldown := a.FallbackCooldown()
	if cooldown == 0 {
		return modelerrors.DefaultCooldown
	}
	return cooldown
}

// getEffectiveRetries returns the number of retries to use for the agent.
// If no retries are explicitly configured (retries == 0), returns
// the default to provide sensible retry behavior out of the box.
// This ensures that transient errors (e.g., Anthropic 529 overloaded) are
// retried even when no fallback models are configured.
//
// Note: Users who explicitly want 0 retries can set retries: -1 in their config
// (though this is an edge case - most users want some retries for resilience).
func getEffectiveRetries(a *agent.Agent) int {
	retries := a.FallbackRetries()
	// -1 means "explicitly no retries" (workaround for Go's zero value)
	if retries < 0 {
		return 0
	}
	// 0 means "use default" - always provide retries for transient error resilience
	if retries == 0 {
		return modelerrors.DefaultRetries
	}
	return retries
}

// tryModelWithFallback attempts to create a stream and get a response using the primary model,
// falling back to configured fallback models if the primary fails.
//
// Retry behavior:
// - Retryable errors (5xx, timeouts): retry the same model with exponential backoff
// - Non-retryable errors (429, 4xx): skip to the next model in the chain immediately
//
// Cooldown behavior:
//   - When the primary fails with a non-retryable error and a fallback succeeds, the runtime
//     "sticks" with that fallback for a configurable cooldown period.
//   - During cooldown, subsequent calls skip the primary and start from the pinned fallback.
//   - When cooldown expires, the primary is tried again; if it succeeds, cooldown is cleared.
//
// Returns the stream result, the model that was used, and any error.
func (r *LocalRuntime) tryModelWithFallback(
	ctx context.Context,
	a *agent.Agent,
	primaryModel provider.Provider,
	messages []chat.Message,
	agentTools []tools.Tool,
	sess *session.Session,
	m *modelsdev.Model,
	events chan Event,
) (streamResult, provider.Provider, error) {
	fallbackModels := a.FallbackModels()

	fallbackRetries := getEffectiveRetries(a)

	// Build the chain of models to try: primary (index 0) + fallbacks (index 1+)
	modelChain := buildModelChain(primaryModel, fallbackModels)

	// Check if we're in a cooldown period and should skip the primary
	startIndex := 0
	inCooldown := false
	cooldownState := r.cooldowns.Get(a.Name())
	if cooldownState != nil && len(fallbackModels) > cooldownState.fallbackIndex {
		// We're in cooldown - start from the pinned fallback (skip primary)
		startIndex = cooldownState.fallbackIndex + 1 // +1 because index 0 is primary
		inCooldown = true
		slog.Debug("Skipping primary due to cooldown",
			"agent", a.Name(),
			"start_from_fallback_index", cooldownState.fallbackIndex,
			"cooldown_until", cooldownState.until.Format(time.RFC3339))
	}

	var lastErr error
	primaryFailedWithNonRetryable := false
	hasFallbacks := len(fallbackModels) > 0

	for chainIdx := startIndex; chainIdx < len(modelChain); chainIdx++ {
		modelEntry := modelChain[chainIdx]

		// Each model in the chain gets (1 + retries) attempts for retryable errors.
		// Non-retryable errors (429 with fallbacks, 4xx) skip immediately to the next model.
		// 429 without fallbacks is retried directly on the same model.
		maxAttempts := 1 + fallbackRetries

		for attempt := range maxAttempts {
			// Check context before each attempt
			if ctx.Err() != nil {
				return streamResult{}, nil, ctx.Err()
			}

			// Apply backoff before retry (not on first attempt of each model)
			if attempt > 0 {
				backoffDelay := backoff.Calculate(attempt - 1)
				logRetryBackoff(a.Name(), modelEntry.provider.ID(), attempt, backoffDelay)
				if !backoff.SleepWithContext(ctx, backoffDelay) {
					return streamResult{}, nil, ctx.Err()
				}
			}

			// Emit fallback event when transitioning to a new model (but not when starting in cooldown)
			if chainIdx > startIndex && attempt == 0 {
				logFallbackAttempt(a.Name(), modelEntry, attempt, fallbackRetries, lastErr)
				// Get the previous model's ID for the event
				prevModelID := modelChain[chainIdx-1].provider.ID()
				reason := ""
				if lastErr != nil {
					reason = lastErr.Error()
				}
				events <- ModelFallback(
					a.Name(),
					prevModelID,
					modelEntry.provider.ID(),
					reason,
					attempt+1,
					maxAttempts,
				)
			}

			slog.Debug("Creating chat completion stream",
				"agent", a.Name(),
				"model", modelEntry.provider.ID(),
				"is_fallback", modelEntry.isFallback,
				"in_cooldown", inCooldown,
				"attempt", attempt+1)

			stream, err := modelEntry.provider.CreateChatCompletionStream(ctx, messages, agentTools)
			if err != nil {
				lastErr = err

				// Context cancellation is never retryable
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return streamResult{}, nil, err
				}

				decision := r.handleModelError(ctx, err, a, modelEntry, attempt, hasFallbacks, &primaryFailedWithNonRetryable)
				if decision == retryDecisionReturn {
					return streamResult{}, nil, ctx.Err()
				} else if decision == retryDecisionBreak {
					break
				}
				continue
			}

			// Stream created successfully, now handle it
			slog.Debug("Processing stream", "agent", a.Name(), "model", modelEntry.provider.ID())

			// If the provider is a rule-based router, notify the sidebar
			// of the selected sub-model's YAML-configured name.
			if rp, ok := modelEntry.provider.(interface{ LastSelectedModelID() string }); ok {
				if selected := rp.LastSelectedModelID(); selected != "" {
					events <- AgentInfo(a.Name(), selected, a.Description(), a.WelcomeMessage())
				}
			}

			res, err := handleStream(ctx, stream, a, agentTools, sess, m, r.telemetry, events)
			if err != nil {
				lastErr = err

				// Context cancellation stops everything
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return streamResult{}, nil, err
				}

				decision := r.handleModelError(ctx, err, a, modelEntry, attempt, hasFallbacks, &primaryFailedWithNonRetryable)
				if decision == retryDecisionReturn {
					return streamResult{}, nil, ctx.Err()
				} else if decision == retryDecisionBreak {
					break
				}
				continue
			}

			// Success!
			// Handle cooldown state based on which model succeeded
			switch {
			case modelEntry.isFallback && primaryFailedWithNonRetryable:
				// Primary failed with non-retryable error, fallback succeeded.
				// Set cooldown to stick with this fallback.
				r.cooldowns.Set(a.Name(), modelEntry.index, getEffectiveCooldown(a))
			case !modelEntry.isFallback:
				// Primary succeeded - clear any existing cooldown.
				// This handles both normal success and recovery after cooldown expires.
				r.cooldowns.Clear(a.Name())
			}

			return res, modelEntry.provider, nil
		}
	}

	// All models and retries exhausted.
	// If the last error (or any error in the chain) was a context overflow,
	// wrap it in a ContextOverflowError so the caller can auto-compact.
	if lastErr != nil {
		prefix := "model failed"
		if hasFallbacks {
			prefix = "all models failed"
		}
		wrapped := fmt.Errorf("%s: %w", prefix, lastErr)
		if modelerrors.IsContextOverflowError(lastErr) {
			return streamResult{}, nil, modelerrors.NewContextOverflowError(wrapped)
		}
		return streamResult{}, nil, wrapped
	}
	return streamResult{}, nil, errors.New("model failed with unknown error")
}

// retryDecision is the outcome of handleModelError.
type retryDecision int

const (
	// retryDecisionContinue means retry the same model (backoff already applied).
	retryDecisionContinue retryDecision = iota
	// retryDecisionBreak means skip to the next model in the fallback chain.
	retryDecisionBreak
	// retryDecisionReturn means context was cancelled; return immediately.
	retryDecisionReturn
)

// handleModelError classifies err and decides what to do next:
//   - retryDecisionReturn   — context cancelled while sleeping; caller returns ctx.Err()
//   - retryDecisionBreak    — non-retryable error or 429 with fallbacks; skip to next model
//   - retryDecisionContinue — retryable error or 429 without fallbacks; retry same model
//
// Side-effect: sets *primaryFailedWithNonRetryable when the primary model fails with a
// non-retryable (or rate-limited-with-fallbacks) error.
func (r *LocalRuntime) handleModelError(
	ctx context.Context,
	err error,
	a *agent.Agent,
	modelEntry modelWithFallback,
	attempt int,
	hasFallbacks bool,
	primaryFailedWithNonRetryable *bool,
) retryDecision {
	retryable, rateLimited, retryAfter := modelerrors.ClassifyModelError(err)

	if rateLimited {
		// Gate: only retry on 429 if opt-in is enabled AND no fallbacks exist.
		// Default behavior (retryOnRateLimit=false) treats 429 as non-retryable,
		// identical to today's behavior before this feature was added.
		if !r.retryOnRateLimit || hasFallbacks {
			slog.Warn("Rate limited, treating as non-retryable",
				"agent", a.Name(),
				"model", modelEntry.provider.ID(),
				"retry_on_rate_limit_enabled", r.retryOnRateLimit,
				"has_fallbacks", hasFallbacks,
				"error", err)
			if !modelEntry.isFallback {
				*primaryFailedWithNonRetryable = true
			}
			return retryDecisionBreak
		}

		// Opt-in enabled, no fallbacks → retry same model after honouring Retry-After (or backoff).
		waitDuration := retryAfter
		if waitDuration <= 0 {
			waitDuration = backoff.Calculate(attempt)
		} else if waitDuration > backoff.MaxRetryAfterWait {
			slog.Warn("Retry-After exceeds maximum, capping",
				"agent", a.Name(),
				"model", modelEntry.provider.ID(),
				"retry_after", retryAfter,
				"max", backoff.MaxRetryAfterWait)
			waitDuration = backoff.MaxRetryAfterWait
		}
		slog.Warn("Rate limited, retrying (opt-in enabled)",
			"agent", a.Name(),
			"model", modelEntry.provider.ID(),
			"attempt", attempt+1,
			"wait", waitDuration,
			"retry_after_from_header", retryAfter > 0,
			"error", err)
		if !backoff.SleepWithContext(ctx, waitDuration) {
			return retryDecisionReturn
		}
		return retryDecisionContinue
	}

	if !retryable {
		slog.Error("Non-retryable error from model",
			"agent", a.Name(),
			"model", modelEntry.provider.ID(),
			"error", err)
		if !modelEntry.isFallback {
			*primaryFailedWithNonRetryable = true
		}
		return retryDecisionBreak
	}

	slog.Warn("Retryable error from model",
		"agent", a.Name(),
		"model", modelEntry.provider.ID(),
		"attempt", attempt+1,
		"error", err)
	return retryDecisionContinue
}
