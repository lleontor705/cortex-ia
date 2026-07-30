package forgespec

import (
	"strings"
	"testing"
)

func TestValidateDirectV1CapabilitiesPreservesQualifiedResponse(t *testing.T) {
	response := PublishedCapabilityResponse{
		Modes:        []string{"legacy", "direct-v1"},
		Capabilities: publishedP0Capabilities(),
	}
	if err := validateDirectV1Capabilities(response); err != nil {
		t.Fatalf("qualified direct-v1 response rejected: %v", err)
	}
}

func TestValidateDirectV1CapabilitiesFailsClosedWithoutDirectV1Mode(t *testing.T) {
	response := PublishedCapabilityResponse{
		Modes:        []string{"legacy"},
		Capabilities: publishedP0Capabilities(),
	}
	if err := validateDirectV1Capabilities(response); err == nil || !strings.Contains(err.Error(), "direct-v1 mode") {
		t.Fatalf("mode-less response error = %v, want direct-v1 mode failure", err)
	}
}

func TestValidateDirectV1CapabilitiesFailsClosedWithMissingRequiredIDs(t *testing.T) {
	response := PublishedCapabilityResponse{
		Modes: []string{"direct-v1"},
		Capabilities: []PublishedCapability{
			{ID: "forgespec.capabilities"},
			{ID: "task-cas"},
		},
	}
	if err := validateDirectV1Capabilities(response); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("incomplete response error = %v, want missing capability failure", err)
	}
}

func TestValidateDirectV1CapabilitiesUsesPublishedCapabilityIDMapping(t *testing.T) {
	response := PublishedCapabilityResponse{
		Modes:        []string{"direct-v1"},
		Capabilities: publishedP0Capabilities(),
	}
	response.Capabilities[0].ID = "capabilities"
	if err := validateDirectV1Capabilities(response); err != nil {
		t.Fatalf("mapped capability response rejected: %v", err)
	}
}

func publishedP0Capabilities() []PublishedCapability {
	capabilities := make([]PublishedCapability, 0, len(RequiredP0Capabilities()))
	for _, requirement := range RequiredP0Capabilities() {
		id := strings.TrimPrefix(string(requirement.ID), "forgespec/")
		if requirement.ID == "forgespec/capabilities" {
			id = "forgespec.capabilities"
		}
		capabilities = append(capabilities, PublishedCapability{ID: id})
	}
	return capabilities
}
