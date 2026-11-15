# Remove Custom Providers — Task Contract

**Change**: remove-custom-providers · **Board**: remove-custom-providers · **Workflow**: sdd-lite · **Workload policy**: flexible
All removals are deletions; verification lines are raw commands. Waves run in order; tasks inside a wave with disjoint files may run in parallel.

## Tasks

### task-rem-001 — [install] Remove provider install transaction surface
- **Requirements:** REQ-REM-001
- **Files:** internal/install/provider.go (delete)
- **Objective:** Delete the entire provider transaction from the install service: ProviderCatalog/ProviderPreview/ProviderInstall, twin reconciliation, providerIdentityDigest, provider entry authoring, ErrProviderUnmanaged, and provider-specific mutate seams. The shared pipeline (txn.go, rollback.go, backup, service.go, doctor.go, uninstall.go) is untouched. Interim state: the three provider test files still reference removed symbols, so this task verifies by build only; task-rem-002 restores the test suite oracle.
- **Verification:** go build ./...
- **Review:** independent reviewer mandatory (install pipeline boundary). No dependencies (Wave 1).

### task-rem-002 — [install] Remove provider install test suites
- **Requirements:** REQ-REM-001
- **Files:** internal/install/provider_test.go (delete), internal/install/provider_container_test.go (delete), internal/install/provider_conflict_test.go (delete)
- **Objective:** Delete the three provider test suites together with the surface they verify (tests of a removed feature are removed, never weakened). Pure-test task: must not be decomposed on failure; fix or prune assertions directly if needed.
- **Verification:** go test -count=1 ./internal/install/...
- **Review:** auto-approve on green suite (pure-test deletion). Depends on task-rem-001 (Wave 1).

### task-rem-003 — [providermgr] Remove the providermgr package
- **Requirements:** REQ-REM-002
- **Files:** internal/providermgr/catalog.go (delete), internal/providermgr/catalog_test.go (delete), internal/providermgr/seed/nan.json (delete)
- **Objective:** Remove the whole package (catalog loading, seedAbsent, parsing) and the embedded nan seed; the sole consumer (internal/install/provider.go) is deleted by task-rem-001. No seed directory is materialized for fresh state roots afterwards. modelmgr, model commands, the archived OpenSpec change, and web/delegation are untouched.
- **Verification:** go build ./...
- **Review:** auto-approve (pure package deletion, compiler proves). Depends on task-rem-001 (Wave 1).

### task-rem-004 — [state] Retire state v2 Providers family
- **Requirements:** REQ-REM-003
- **Files:** internal/state/provider_v2.go (delete), internal/state/metadata_v2.go (edit), internal/state/provider_v2_test.go (delete)
- **Objective:** Per Decision D1: delete ProviderV2, validateProviders, provider_v2_test.go, and the Providers field with its normalization (metadata_v2.go ~249-262) and validation call sites (~319, ~497). MetadataSchemaV2 stays 2; no v3 constant. Existing v2 documents with a providers array keep loading because json.Unmarshal tolerates the stale key; the key disappears on the next natural commitStateV2 re-marshal.
- **Verification:** go test -count=1 ./internal/state/...
- **Review:** independent reviewer mandatory (state migration integrity). Depends on task-rem-001 (Wave 2).

### task-rem-005 — [state] Add provider retirement tolerance oracle
- **Requirements:** REQ-REM-003
- **Files:** internal/state/provider_retirement_test.go (new, ≤ 250 LOC)
- **Objective:** New modular regression oracle (pure-test, temporary home) proving the passive-retirement contract: (1) a v2 state.json containing a providers array classifies as PresenceV2 and validates; (2) a new commit drops the stale providers key while remaining families stay intact; (3) unknown future schema_version stays malformed; (4) MetadataSchemaV2 is 2 and no v3 constant exists.
- **Verification:** go test -count=1 ./internal/state/...
- **Review:** independent reviewer mandatory (state migration integrity). Depends on task-rem-004 (Wave 2).

### task-rem-006 — [tui] Remove providers TUI screen
- **Requirements:** REQ-REM-004
- **Files:** internal/tui/providers_screen.go (delete), internal/tui/providers_screen_test.go (delete), internal/tui/providers_home_test.go (delete)
- **Objective:** Delete the self-contained screen component with its phases, messages (providersCatalogMsg, providersResultMsg), and the cp-06 flow oracles (providers_home_test.go, 249 LOC) — tests of the removed surface are deleted, not weakened. masked_input.go deletion lands in task-rem-007 (its only consumer is this screen). Interim state verifies by build; task-rem-008 restores the TUI suite oracle.
- **Verification:** go build ./...
- **Review:** auto-approve (pure screen deletion, compiler proves). Depends on task-rem-001 (Wave 3).

### task-rem-007 — [tui] Rewire TUI home, actions and masked input
- **Requirements:** REQ-REM-004
- **Files:** internal/tui/model.go (edit), internal/tui/actions.go (edit), internal/tui/masked_input.go (delete)
- **Objective:** model.go: remove screenProviders, providersEntryIndex=3, the homeEntries row, providers state field + newProvidersState init, providersCatalogMsg/providersResultMsg routing, the hotkey p/P case, and the entry-3 handler; renumber subsequent index constants (uninstallEntryIndex 9→8, etc.). actions.go: remove openProviders/updateProviders and the providersEntryIndex cursor reference. Delete masked_input.go (sole consumer gone). Interim state verifies by build only; task-rem-008 restores the TUI suite oracle.
- **Verification:** go build ./...
- **Review:** independent reviewer mandatory (shared navigation wiring). Depends on task-rem-006 (Wave 3).

### task-rem-008 — [tui] Update home menu views and navigation oracles
- **Requirements:** REQ-REM-004
- **Files:** internal/tui/views.go (edit), internal/tui/navigation_test.go (edit)
- **Objective:** views.go: remove the "Instalar un proveedor personalizado..." homeDescriptions row and the screenProviders render case. navigation_test.go: update the count oracle 11→10 and remove the p/P hotkey transition case. The TUI suite must be fully green, proving homeEntries/homeDescriptions stay equally sized and no navigation regression remains.
- **Verification:** go test -count=1 ./internal/tui/...
- **Review:** auto-approve on green suite (deletions with updated oracles). Depends on task-rem-007 (Wave 3).

### task-rem-009 — [docs] Retirement stub and linked docs
- **Requirements:** REQ-REM-005
- **Files:** docs/getting-started/custom-providers.md (edit), docs/operations/stats-and-ui.md (edit), docs/architecture/codebase/dashboard.md (edit)
- **Objective:** Rewrite custom-providers.md as a short retirement stub containing the exact marker "This feature was removed" and stating provider installation is native in OpenCode (nan), keeping inbound links valid. stats-and-ui.md: drop the Providers bullet and point the former link to the stub. dashboard.md: remove the entry-10 diagram node and the screenProviders description.
- **Verification:** grep -q "This feature was removed" docs/getting-started/custom-providers.md
- **Review:** auto-approve (docs). Depends on task-rem-003, task-rem-008 (Wave 4).

### task-rem-010 — [docs] Architecture maps update
- **Requirements:** REQ-REM-005
- **Files:** docs/architecture/codebase/repository-map.md (edit), docs/architecture/codebase/reference-map.md (edit), docs/architecture/codebase/sync-and-cloud.md (edit)
- **Objective:** repository-map.md: remove the Providers screen from the screens list. reference-map.md: remove the provider manager row. sync-and-cloud.md: drop "custom providers" from the state.json description. No provider-installer surface remains documented as live.
- **Verification:** ! grep -q providermgr docs/architecture/codebase/repository-map.md docs/architecture/codebase/reference-map.md
- **Review:** auto-approve (docs). Depends on task-rem-003, task-rem-008 (Wave 4).

### task-rem-011 — [docs] CHANGELOG breaking-change entry
- **Requirements:** REQ-REM-005
- **Files:** CHANGELOG.md (edit)
- **Objective:** Add a breaking-change entry recording that the custom provider installer (TUI screen, CLI-adjacent service surface, state v2 Providers family) was removed in this version because the nan provider ships native OpenCode support; mention that existing state.json provider rows are passively retired on the next commit and previously installed provider blocks remain valid native OpenCode config.
- **Verification:** grep -q "custom provider installer" CHANGELOG.md
- **Review:** auto-approve (docs). Depends on task-rem-003, task-rem-008 (Wave 4).

Release gate after Wave 4 (reviewer/orchestrator level): gofmt -s -w ., go vet ./..., golangci-lint run ./..., go test -count=1 ./...
