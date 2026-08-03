package forgespec

import "fmt"

// ValidateHistoryBoundary is the mode guard for the bridge. Legacy evidence
// may be read, but a direct-v1 board must never be routed through a legacy
// adapter or share the legacy board identity.
func ValidateHistoryBoundary(legacyMode, directMode, legacyBoardID, directBoardID string, legacyAdapter bool) error {
	if legacyMode != "legacy" {
		return fmt.Errorf("legacy mode required, got %q", legacyMode)
	}
	if directMode != "direct-v1" {
		return fmt.Errorf("direct-v1 mode required, got %q", directMode)
	}
	if legacyBoardID == "" || directBoardID == "" || legacyBoardID == directBoardID {
		return fmt.Errorf("legacy and direct-v1 board IDs must be distinct")
	}
	if legacyAdapter {
		return fmt.Errorf("legacy adapter is forbidden for direct-v1 board")
	}
	return nil
}
