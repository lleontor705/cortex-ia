package verify

// FindingCode is a stable, machine-readable doctor finding identifier.
type FindingCode string

const (
	FindingRuntimeVersion FindingCode = "doctor.runtime.version"
	FindingEvidenceStale  FindingCode = "doctor.evidence.freshness"
	FindingSchemaInterval FindingCode = "doctor.schema.interval"
	FindingAssetHash      FindingCode = "doctor.asset.hash"
	FindingOwnership      FindingCode = "doctor.asset.ownership"
	FindingPermissions    FindingCode = "doctor.permissions.delta"
	FindingSecret         FindingCode = "doctor.secret.exposure"
	FindingServiceVersion FindingCode = "doctor.service.version"
	FindingBinding        FindingCode = "doctor.binding.resolution"
	FindingManifest       FindingCode = "doctor.manifest.consistency"
)

// CheckKind identifies a required doctor diagnostic domain.
type CheckKind string

const (
	CheckRuntimeVersion    CheckKind = "runtime-version"
	CheckEvidenceFreshness CheckKind = "evidence-freshness"
	CheckSchemaInterval    CheckKind = "schema-interval"
	CheckAssetHash         CheckKind = "asset-hash"
	CheckOwnership         CheckKind = "ownership"
	CheckPermissions       CheckKind = "permissions"
	CheckSecrets           CheckKind = "secrets"
	CheckServiceVersion    CheckKind = "service-version"
	CheckBinding           CheckKind = "binding"
	CheckManifest          CheckKind = "manifest"
)

// CheckState is the observed state of one diagnostic domain.
type CheckState string

const (
	StateHealthy      CheckState = "healthy"
	StateMismatch     CheckState = "mismatch"
	StateStale        CheckState = "stale"
	StateBeyondTested CheckState = "beyond-tested"
	StateCorrupt      CheckState = "corrupt"
	StateCustomized   CheckState = "customized"
	StateUnknown      CheckState = "unknown"
	StatePresent      CheckState = "present"
	StateUnsupported  CheckState = "unsupported"
	StateUnresolved   CheckState = "unresolved"
)

// Observation is evidence collected by a target-specific doctor adapter.
// Diagnose converts unhealthy observations into stable actionable findings.
type Observation struct {
	Kind        CheckKind
	State       CheckState
	Target      string
	Path        string
	Observed    string
	Expected    string
	Evidence    string
	Remediation string
}

// Finding is an actionable doctor result with a stable external contract.
type Finding struct {
	Code        FindingCode `json:"code"`
	Severity    Severity    `json:"severity"`
	Target      string      `json:"target"`
	Path        string      `json:"path"`
	Observed    string      `json:"observed"`
	Expected    string      `json:"expected"`
	Evidence    string      `json:"evidence"`
	Remediation string      `json:"remediation"`
	Blocking    bool        `json:"blocking"`
}

// DoctorReport is the qualification result for the selected profile.
type DoctorReport struct {
	Profile   string    `json:"profile"`
	Qualified bool      `json:"qualified"`
	Findings  []Finding `json:"findings"`
}

// Blockers returns the number of findings that block install or use.
func (r DoctorReport) Blockers() int {
	count := 0
	for _, finding := range r.Findings {
		if finding.Blocking {
			count++
		}
	}
	return count
}

type findingPolicy struct {
	code     FindingCode
	severity Severity
}

var doctorPolicies = map[CheckKind]findingPolicy{
	CheckRuntimeVersion:    {code: FindingRuntimeVersion, severity: SeverityWarning},
	CheckEvidenceFreshness: {code: FindingEvidenceStale, severity: SeverityWarning},
	CheckSchemaInterval:    {code: FindingSchemaInterval, severity: SeverityError},
	CheckAssetHash:         {code: FindingAssetHash, severity: SeverityError},
	CheckOwnership:         {code: FindingOwnership, severity: SeverityError},
	CheckPermissions:       {code: FindingPermissions, severity: SeverityError},
	CheckSecrets:           {code: FindingSecret, severity: SeverityError},
	CheckServiceVersion:    {code: FindingServiceVersion, severity: SeverityError},
	CheckBinding:           {code: FindingBinding, severity: SeverityError},
	CheckManifest:          {code: FindingManifest, severity: SeverityError},
}

// AllDoctorCheckKinds returns every diagnostic domain required for qualification.
func AllDoctorCheckKinds() []CheckKind {
	return []CheckKind{
		CheckRuntimeVersion,
		CheckEvidenceFreshness,
		CheckSchemaInterval,
		CheckAssetHash,
		CheckOwnership,
		CheckPermissions,
		CheckSecrets,
		CheckServiceVersion,
		CheckBinding,
		CheckManifest,
	}
}

// Diagnose evaluates collected observations conservatively. In particular,
// stale evidence and versions beyond the tested range always block qualification.
func Diagnose(profile string, observations []Observation) DoctorReport {
	report := DoctorReport{Profile: profile, Qualified: true}
	for _, observation := range observations {
		if observation.State == StateHealthy {
			continue
		}

		policy, known := doctorPolicies[observation.Kind]
		if !known {
			policy = findingPolicy{code: FindingCode("doctor.check.unknown"), severity: SeverityError}
		}

		report.Findings = append(report.Findings, Finding{
			Code:        policy.code,
			Severity:    policy.severity,
			Target:      observation.Target,
			Path:        observation.Path,
			Observed:    observation.Observed,
			Expected:    observation.Expected,
			Evidence:    observation.Evidence,
			Remediation: observation.Remediation,
			Blocking:    true,
		})
		report.Qualified = false
	}
	return report
}
