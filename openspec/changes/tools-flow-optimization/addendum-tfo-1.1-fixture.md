# tfo-1.1 existing fixture alignment

This additive scope amendment applies only to tfo-1.1 and REQ-TOOLS-001. The four original shared contract pins remain unchanged.

Add `internal/delegation/store_test.go` to the seven allowed paths listed for tfo-1.1. Change only the existing TestWorkItemsClaimsAndLeases assertion that expects individual lease release to succeed after transition to in_review: the authorized behavior now releases leases atomically during that transition, so verify that the lease is already absent. Do not append a new suite or weaken ownership, transition or release checks.

No acceptance semantics change: delivery, evidence and lease release remain transactional. All other task scopes and contract pins, including active tfo-3.2, remain unchanged. This amendment supersedes only the original seven-path enumeration for tfo-1.1 by adding the single fixture path.

Verification: `go test -count=1 ./internal/delegation ./internal/app` and the task's isolated submission harness. Real work-control commands continue to use the absolute installed schema-13 CLI; candidate tests use temporary state roots.
