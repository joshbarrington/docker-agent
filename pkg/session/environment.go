package session

import (
	"fmt"
	"runtime"
)

// GetEnvironmentInfo returns formatted environment information including
// working directory, git repository status, and platform information
func GetEnvironmentInfo(workingDir string) string {
	return fmt.Sprintf(`Here is useful information about the environment you are running in:
	<env>
	Working directory: %s
	Is directory a git repo: %s
	Operating System: %s
	CPU Architecture: %s
	</env>`, workingDir, boolToYesNo(isGitRepo(workingDir)), getOperatingSystem(), getArchitecture())
}

// boolToYesNo converts a boolean to "Yes" or "No" string
func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func getOperatingSystem() string {
	switch runtime.GOOS {
	case "darwin":
		return "MacOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func getArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}
