package userprompt

import (
	"github.com/docker/docker-agent/pkg/tui/components/spinner"
	"github.com/docker/docker-agent/pkg/tui/components/toolcommon"
	"github.com/docker/docker-agent/pkg/tui/core/layout"
	"github.com/docker/docker-agent/pkg/tui/service"
	"github.com/docker/docker-agent/pkg/tui/types"
)

// New creates a component for the user_prompt tool call.
// It intentionally does not render the tool call's arguments (the question,
// title or schema). It only indicates that a question is being asked to the
// user, via the tool's status icon and display name.
func New(msg *types.Message, sessionState service.SessionStateReader) layout.Model {
	return toolcommon.NewBase(msg, sessionState, render)
}

func render(msg *types.Message, s spinner.Spinner, sessionState service.SessionStateReader, width, _ int) string {
	return toolcommon.RenderTool(msg, s, "", "", width, sessionState.HideToolResults())
}
