package latest

// ValidateHooksConfigForTest exposes the unexported [HooksConfig.validate]
// method for use by tests in the latest_test package. It is only compiled
// during `go test`.
func ValidateHooksConfigForTest(h *HooksConfig) error {
	return h.validate()
}
