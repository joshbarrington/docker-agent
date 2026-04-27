package builtins

import (
	"context"

	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
)

// AddEnvironmentInfo is the registered name of the add_environment_info builtin.
const AddEnvironmentInfo = "add_environment_info"

// addEnvironmentInfo emits cwd / git / OS / arch info as session_start
// additional context. No-op when Cwd is empty.
func addEnvironmentInfo(_ context.Context, in *hooks.Input, _ []string) (*hooks.Output, error) {
	if in == nil || in.Cwd == "" {
		return nil, nil
	}
	return hooks.NewAdditionalContextOutput(hooks.EventSessionStart, session.GetEnvironmentInfo(in.Cwd)), nil
}
