package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const externalRootBlockedReason = "kiro/external-root/blocked"

// ExternalRootObservation is the executable, pre-mutation result for an
// adapter whose config root cannot be safely contained by the workflow home.
// It deliberately records a blocked result instead of hiding the cell behind
// a test skip.
type ExternalRootObservation struct {
	Adapter             string
	RequestedProfile    string
	EffectiveProfile    string
	Disposition         Disposition
	ReasonID            string
	ObservedError       string
	Command             string
	ExitCode            int
	ProtectedRootDigest string
	Mutation            string
	Report              EvidenceReport
}

func observeExternalRootBlocked(adapter, profile, command string, exitCode int, observedErr string, protectedRoot []byte) (ExternalRootObservation, error) {
	if strings.TrimSpace(adapter) == "" || strings.TrimSpace(profile) == "" {
		return ExternalRootObservation{}, fmt.Errorf("external-root evidence requires adapter and profile")
	}
	if strings.TrimSpace(command) == "" || exitCode == 0 {
		return ExternalRootObservation{}, fmt.Errorf("external-root evidence requires failing command and non-zero exit")
	}
	if strings.TrimSpace(observedErr) == "" {
		return ExternalRootObservation{}, fmt.Errorf("external-root evidence requires observed error")
	}
	if protectedRoot == nil {
		return ExternalRootObservation{}, fmt.Errorf("external-root evidence requires protected-root snapshot")
	}

	rootDigest := sha256.Sum256(protectedRoot)
	protectedDigest := "sha256:" + hex.EncodeToString(rootDigest[:])
	matrix := Matrix{
		Adapters: []string{adapter},
		Profiles: []string{profile},
		Cells: []Cell{{
			Adapter: adapter, RequestedProfile: profile, EffectiveProfile: profile,
			Disposition: DispositionRejected, ReasonID: externalRootBlockedReason,
			Command: command, ExitCode: exitCode,
			Hash: protectedDigest,
			Evidence: map[string]string{
				"mutation": "none", "pre_mutation": "proven", "protected_root_digest": protectedDigest,
				"observed_error": observedErr,
			},
		}},
	}
	report, err := AggregateEvidence(matrix, EvidenceOptions{
		ContractVersion: "1.0.0", ContractFingerprint: protectedDigest,
		PrimaryModel: "route/unresolved", FallbackModel: "route/unresolved",
		ModelDegradation: externalRootBlockedReason, QualityPlan: "kiro/external-root",
		TrustEvidence: []string{"protected-root-snapshot:" + protectedDigest},
		Permissions:   []string{"filesystem/read"},
	})
	if err != nil {
		return ExternalRootObservation{}, err
	}
	return ExternalRootObservation{
		Adapter: adapter, RequestedProfile: profile, EffectiveProfile: profile,
		Disposition: DispositionRejected, ReasonID: externalRootBlockedReason,
		ObservedError: observedErr, Command: command, ExitCode: exitCode,
		ProtectedRootDigest: protectedDigest, Mutation: "none", Report: report,
	}, nil
}
