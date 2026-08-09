package forgespec

import (
	"github.com/lleontor705/cortex-ia/internal/components/mcpinject"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/services"
)

const (
	QualifiedVersion    = "1.4.0"
	QualifiedNPMPackage = "forgespec-mcp@" + QualifiedVersion
	OpenCodeCommand     = "forgespec-mcp"
)

// Contract declares ForgeSpec as the sole external authority for SDD contracts
// and task lifecycle state. The conservative tested ceiling requires newer
// versions to be disclosed as degraded until separately qualified.
func Contract() services.ServiceContract {
	version := ir.MustParseVersion("1.0.0")
	qualified := ir.MustParseVersion(QualifiedVersion)
	interval := ir.VersionRange{Minimum: version, MaximumTested: qualified}
	return services.ServiceContract{
		SchemaVersion: version, Owner: services.OwnerForgeSpec, Authority: services.AuthorityExternalService,
		Versions: interval, ExternalDependency: true,
		Responsibilities: []services.Responsibility{
			services.ResponsibilityContracts, services.ResponsibilityTaskDependencies, services.ResponsibilityTaskReadiness,
			services.ResponsibilityTaskClaim, services.ResponsibilityTaskStatus,
		},
		RequiredCapabilities: []services.CapabilityRequirement{{ID: "forgespec.capabilities", Versions: interval, Upstream: true}},
	}
}

// Templates returns the MCP server templates for forgespec-mcp.
func Templates() mcpinject.ServerTemplates {
	return mcpinject.ServerTemplates{
		Name: "forgespec", Service: Contract(),

		SeparateFileJSON: []byte(`{
  "command": "npx",
  "args": [
    "-y",
    "` + QualifiedNPMPackage + `"
  ]
}
`),

		DefaultOverlayJSON: []byte(`{
  "mcpServers": {
    "forgespec": {
      "command": "npx",
      "args": [
        "-y",
        "` + QualifiedNPMPackage + `"
      ]
    }
  }
}
`),

		OpenCodeOverlayJSON: []byte(`{
  "mcp": {
    "forgespec": {
      "type": "local",
      "command": [
        "` + OpenCodeCommand + `"
      ],
      "enabled": true
    }
  }
}
`),

		VSCodeOverlayJSON: []byte(`{
  "servers": {
    "forgespec": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "` + QualifiedNPMPackage + `"
      ]
    }
  }
}
`),

		TOMLCommand: "npx",
		TOMLArgs:    []string{"-y", QualifiedNPMPackage},
	}
}
