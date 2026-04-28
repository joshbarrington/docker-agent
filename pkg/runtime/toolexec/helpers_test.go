package toolexec_test

import (
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/permissions"
)

// newDenyChecker returns a [*permissions.Checker] that denies the given
// tool name and ignores everything else.
func newDenyChecker(toolName string) *permissions.Checker {
	return permissions.NewChecker(&latest.PermissionsConfig{
		Deny: []string{toolName},
	})
}
