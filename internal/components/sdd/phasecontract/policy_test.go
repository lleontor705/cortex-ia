package phasecontract

import (
	"reflect"
	"testing"
)

func TestPolicySnapshotIsStableAndKeyed(t *testing.T) {
	snapshot := PolicySnapshot()
	if snapshot.Version == "" || snapshot.SourceFingerprint == "" {
		t.Fatalf("policy snapshot metadata = %#v, want version and fingerprint", snapshot)
	}
	if len(snapshot.Entries) != 9 {
		t.Fatalf("policy entries = %d, want 9", len(snapshot.Entries))
	}
	for _, entry := range snapshot.Entries {
		if entry.Phase == "" || entry.RetryProfile == "" || entry.RouteKey == "" || entry.ModelRouteKey == "" || entry.ReasonID == "" {
			t.Errorf("incomplete policy entry: %#v", entry)
		}
	}
	if !reflect.DeepEqual(snapshot, PolicySnapshot()) {
		t.Fatal("policy snapshot is not deterministic")
	}
}

func TestPolicyKeysResolveExecutableRetryProfiles(t *testing.T) {
	for _, key := range RetryProfileKeys() {
		policy, err := RetryPolicyForKey(key)
		if err != nil {
			t.Fatalf("RetryPolicyForKey(%q): %v", key, err)
		}
		if policy.TransientMax == 0 || policy.SemanticMax == 0 || policy.NoProgressCycles == 0 {
			t.Errorf("RetryPolicyForKey(%q) = %#v, want executable ceilings", key, policy)
		}
	}
}
