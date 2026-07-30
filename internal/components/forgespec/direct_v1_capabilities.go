package forgespec

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// ValidateDirectV1Capabilities validates the published mode and the exact
// capability IDs required for direct-v1. A compatibility boolean alone is not
// sufficient because it may describe a different negotiated mode.
func ValidateDirectV1Capabilities(response PublishedCapabilityResponse) error {
	return validateDirectV1Capabilities(response)
}

func validateDirectV1Capabilities(response PublishedCapabilityResponse) error {
	hasDirectV1 := false
	for _, mode := range response.Modes {
		if mode == "direct-v1" {
			hasDirectV1 = true
			break
		}
	}
	if !hasDirectV1 {
		return fmt.Errorf("direct-v1 mode is not supported")
	}
	observed := make(map[ir.SemanticID]bool, len(response.Capabilities))
	for _, capability := range response.Capabilities {
		observed[MapPublishedCapabilityID(capability.ID)] = true
	}
	missing := make([]string, 0)
	for _, required := range RequiredP0Capabilities() {
		if !observed[required.ID] {
			missing = append(missing, string(required.ID))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("direct-v1 capability set is incomplete: missing %s", strings.Join(missing, ", "))
	}
	return nil
}
