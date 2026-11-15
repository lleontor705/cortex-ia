# Amendment 2026-09-26 — Automatic Twin-File Reconciliation: Ready-to-Apply Work Revise Plans

Locked operator decision: provider install reconciles twin-file duplication automatically (backup FIRST, then non-winning twin block removal via filemerge RemovePaths, then winner __replace__ overlay, then ownership commit; preview discloses; receipt discloses both mutations; failure restores both files; unmanaged-winner refusal preserved). Wave-1 items cp-01-catalog and cp-02-state are done+PASS and are NOT touched. The planner tool surface exposes no work-revise binding (work create rejects duplicate IDs), so the four payloads below are applied by the orchestrator/operator with the documented CLI equivalent. Revise validates pins against on-disk bytes; do not edit the JSON.

Apply each block verbatim:

    cortex-ia work revise <task-id> --revision <expected_revision> --plan @stdin

with the JSON block as stdin (e.g. copy block to a temp file and use --plan <file>). After all four, re-verify with cortex-ia work fingerprint <task-id> (or cortex_ia_work_fingerprint where exposed) and cortex_ia_board_status custom-providers. Expected pin set (this amendment's digests): spec.md 3152725603fd…, design.md 7729d5778c…, tasks.md da8b87c5f4…, plan.md f24a44b323… (untouched), proposal.md 4be198b063… (untouched).

## Plan: cp-03-install (expected_revision 2, expected_status ready)

~~~json
{
  "version": 1,
  "task_id": "cp-03-install",
  "board_id": "custom-providers",
  "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
  "expected_revision": 2,
  "expected_status": "ready",
  "expected_workflow": "sdd-lite",
  "expected_change_id": "add-custom-providers",
  "expected_spec_plane": "hybrid",
  "definition": {
    "title": "[install] Provider transaction: backup-first twin reconciliation, __replace__ overlay, ownership",
    "objective": "Add internal/install/provider.go exposing ProviderCatalog, ProviderPreview (fully resolved dry-run), and ProviderInstall on install.Service. Resolution picks the winning OpenCode config by the JSONC-wins-else-JSON precedence (modelmgr.ConfigPath, layout.ResolveConfigRelPath). Twin duplication is reconciled AUTOMATICALLY (locked operator amendment 2026-09-26, superseding refusal): when provider.id is defined in the non-winning twin, or differs across twins while the winner copy is installer-owned, ProviderInstall must capture one verified backup protecting BOTH config files FIRST, then atomically remove the stale provider.id block from the non-winning twin via the filemerge MutateJSONFile JSONMutation.RemovePaths edit, then apply the winner provider-subtree __replace__ overlay materializing ALL catalog models (variants array of {id, settings:{reasoningEffort}} for non-empty efforts, no variants key for empty ones, matching modelmgr's authored shape), then commit the Providers ownership record whose SemanticDigest is identity-only and provably excludes options.apiKey and headers. ProviderPreview must disclose the planned reconciliation (twin path + block removal + winner write + backup-first ordering) while writing nothing, and the receipt must disclose BOTH mutations (ReconciledTwinPath + ConfigPath) under one BackupID with no token field anywhere; if the twin edit or winner overlay fails after backup, both files are restored from the verified backup and state.json is left uncommitted. The unmanaged-winner refusal is preserved unchanged: provider.id present in the winner without a covering ownership record returns ErrProviderUnmanaged with zero mutations and zero backups. The token travels only as the ProviderInstallOptions.Token argument into options.apiKey. Reinstall with rotated token converges (wholesale replace, no orphan models); identical state reports already-present without a new backup. Contract: REQ-PROV-002, REQ-PROV-003, REQ-PROV-004.",
    "acceptance_criteria": "- Given winner lacks provider.nan and the twin defines it, When ProviderInstall runs, Then strict order backup, twin RemovePaths cleanup, winner __replace__ overlay, ownership commit; the receipt names the twin path and winner path under the single BackupID (REQ-PROV-004).\n- Given provider.nan differs across both twins with an installer-owned winner copy, Then backup-first reconciliation removes the twin block and converges the winner to the full catalog definition (8 models, variants per the pinned modelmgr shape), leaving exactly one managed provider.nan.\n- Given the same state, When ProviderPreview runs, Then the dry-run discloses planned twin cleanup, winner write, and backup-first ordering with every config/state/providers file mtime unchanged.\n- Given fault injection at the twin edit and separately at the winner overlay after backup, Then both files restore byte-identical from the verified backup and state.json holds no Providers commit.\n- Given provider.nan unmanaged in the winner, Then typed ErrProviderUnmanaged refusal, zero mutations, zero backups, and no reconciliation attempted against winner content.\n- Given valid catalog + sentinel token in a temp home, Then winner config carries npm @ai-sdk/openai-compatible, name, baseURL, options.apiKey, all 8 model entries with correct variants arrays, while JSONC comments and unrelated members of BOTH files stay byte-identical except the removed stale block; receipt, state.json, and SemanticDigest hold no token characters; token rotation leaves the digest unchanged.\n- Second identical install returns already-present without a new backup; rotated token converges with updated record.\n- All tests use temp home + CORTEX_IA_HOME override and synthetic sentinel tokens; TestREQ_PROV_002/003/004_* in the two dedicated files <=250 LOC each.\n- Mutation evidence over ownership-gate, reconciliation-ordering, restore-path, and overlay-construction branches; SURVIVED blocks in_review.",
    "verification": "go build ./... && go test -count=1 ./internal/install -run '^TestREQ_PROV_00[234]'",
    "allowed_files": [
      "internal/install/provider.go",
      "internal/install/provider_test.go",
      "internal/install/provider_conflict_test.go"
    ],
    "sdd_contract": {
      "version": 1,
      "workflow": "sdd-lite",
      "change_id": "add-custom-providers",
      "spec_plane": "hybrid",
      "pins": [
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/plan.md",
          "sha256": "f24a44b323cb1a063ab4be5e6f1ae3a6d0513020b3579df877d4ba1fbcac1125"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/proposal.md",
          "sha256": "4be198b0638b76769eea13c5762cc65122680b98d0fd67e9247e4b0d0a6b1dd6"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/specs/providers/spec.md",
          "sha256": "3152725603fd140a20f61d8e03b665f1be16f44c765ea07a7bc5cffab72847f8"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/design.md",
          "sha256": "7729d5778ca633fab5a00ed03004fa5a90eaef63025ef367b3a2edbed0c2416d"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/tasks.md",
          "sha256": "da8b87c5f49402cd696dc7029fe149250395cfe61d83bba76a443401a7cbd70e"
        }
      ],
      "requirement_ids": [
        "REQ-PROV-002",
        "REQ-PROV-003",
        "REQ-PROV-004"
      ]
    }
  }
}
~~~

## Plan: cp-04-screen (expected_revision 1, expected_status backlog)

~~~json
{
  "version": 1,
  "task_id": "cp-04-screen",
  "board_id": "custom-providers",
  "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
  "expected_revision": 1,
  "expected_status": "backlog",
  "expected_workflow": "sdd-lite",
  "expected_change_id": "add-custom-providers",
  "expected_spec_plane": "hybrid",
  "definition": {
    "title": "[tui] Masked token input and providers List-Input-Preview-Receipt machine",
    "objective": "Create internal/tui/providers_screen.go as a 1:1 structural mirror of the models_screen.go precedent: screen-local providersPhase machine List, Input, Preview, Receipt plus an Error degrade surface, driven entirely through package-var seams providersListCatalog/providersPreview/providersInstall over install.New(homeDir) so tests swap in fakes against temporary homes without touching ServiceAPI. Create internal/tui/masked_input.go: a manual rune-buffer token field (printable runes, backspace, paste-as-runes) that renders fixed-width bullet glyphs and exposes the raw value only for the single service call argument — the repo has no masked input today and bubbles must not be promoted from its indirect dependency. Preview must be provably inert (only the cp-03 dry-run seam is called) and must surface the dry-run's planned twin reconciliation in its render, Escape rewinds exactly one phase, and no rendered output ever contains raw token characters. Contract: REQ-PROV-003 (masked input facet), REQ-PROV-005.",
    "acceptance_criteria": "- Given catalog seam returns the Nan provider, When List renders, Then the provider and its 8 models with effort vocabularies appear.\n- Given Input phase and typed runes, When the view renders, Then only bullet glyphs of matching width appear and no raw character is present in any rendered string.\n- Given a token entered, When Preview renders, Then exactly the dry-run seam is called, no install seam fires, and Escape returns to Input keeping the buffer.\n- Given the dry-run reports a planned twin reconciliation, When Preview renders, Then the disclosure lines (twin path + block removal) appear without any filesystem write.\n- Given confirm on Preview, Then the machine transitions through Running phases Backup, Update config, Commit state and lands on Receipt showing action, config path, model count, and backup id with zero token characters.\n- Given catalog load error, Then Error surface offers only Escape to Home and Input never renders empty.\n- Tests TestREQ_PROV_003_* and TestREQ_PROV_005_* in providers_screen_test.go <=250 LOC, seams faked, no real homes.\n- Mutation evidence over phase-transition guards and masking branches; SURVIVED blocks in_review.",
    "verification": "go build ./... && go test -count=1 ./internal/tui -run '^TestREQ_PROV_00[35]'",
    "allowed_files": [
      "internal/tui/masked_input.go",
      "internal/tui/providers_screen.go",
      "internal/tui/providers_screen_test.go"
    ],
    "sdd_contract": {
      "version": 1,
      "workflow": "sdd-lite",
      "change_id": "add-custom-providers",
      "spec_plane": "hybrid",
      "pins": [
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/plan.md",
          "sha256": "f24a44b323cb1a063ab4be5e6f1ae3a6d0513020b3579df877d4ba1fbcac1125"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/proposal.md",
          "sha256": "4be198b0638b76769eea13c5762cc65122680b98d0fd67e9247e4b0d0a6b1dd6"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/specs/providers/spec.md",
          "sha256": "3152725603fd140a20f61d8e03b665f1be16f44c765ea07a7bc5cffab72847f8"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/design.md",
          "sha256": "7729d5778ca633fab5a00ed03004fa5a90eaef63025ef367b3a2edbed0c2416d"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/tasks.md",
          "sha256": "da8b87c5f49402cd696dc7029fe149250395cfe61d83bba76a443401a7cbd70e"
        }
      ],
      "requirement_ids": [
        "REQ-PROV-003",
        "REQ-PROV-005"
      ]
    }
  }
}
~~~

## Plan: cp-05-home (expected_revision 1, expected_status backlog)

~~~json
{
  "version": 1,
  "task_id": "cp-05-home",
  "board_id": "custom-providers",
  "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
  "expected_revision": 1,
  "expected_status": "backlog",
  "expected_workflow": "sdd-lite",
  "expected_change_id": "add-custom-providers",
  "expected_spec_plane": "hybrid",
  "definition": {
    "title": "[tui] Home entry 9 with hotkey p and frozen numeric keys",
    "objective": "Wire the providers screen into Home across the three registration places: append 'Install custom provider' to homeEntries (model.go, index 9), add the tenth homeDescriptions line (views.go), and extend selectHomeEntry with case 9; append screenProviders at the end of the screen iota so no existing index shifts. Add case 'p','P' in updateHome to open the providers screen; numeric keys 1-9 keep their exact current mappings (no remap, key 0 stays reserved unbound) and the m/M models hotkey is untouched. Add providerPhases (Backup, Update config, Commit state) and the running-command wrappers in actions.go that drive the cp-04 seams. This task must keep every existing TUI navigation and boot suite green because those suites clamp against len(homeEntries); the new TestREQ oracles land in cp-06, and per policy this task does not append to any test file above 300 LOC. Contract: REQ-PROV-005 (wiring facet), REQ-PROV-006.",
    "acceptance_criteria": "- Given Home, When pressing p or P, Then the active screen becomes the providers List phase.\n- Given Home, When pressing 1 through 9, Then entry indices 0 through 8 are selected exactly as before this change, and m still opens the models screen.\n- homeEntries and homeDescriptions have equal length 10; cursor up/down wraps are clamped and enter on index 9 activates the providers screen.\n- selectHomeEntry case 9 sets screenProviders and starts no other operation.\n- go vet ./internal/tui/... clean and the full existing ./internal/tui suite green.\n- Combined source diff <=120 LOC; mutation evidence over the new key and selectHomeEntry branches; SURVIVED blocks in_review.",
    "verification": "go build ./... && go vet ./internal/tui/... && go test -count=1 ./internal/tui",
    "allowed_files": [
      "internal/tui/model.go",
      "internal/tui/views.go",
      "internal/tui/actions.go"
    ],
    "sdd_contract": {
      "version": 1,
      "workflow": "sdd-lite",
      "change_id": "add-custom-providers",
      "spec_plane": "hybrid",
      "pins": [
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/plan.md",
          "sha256": "f24a44b323cb1a063ab4be5e6f1ae3a6d0513020b3579df877d4ba1fbcac1125"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/proposal.md",
          "sha256": "4be198b0638b76769eea13c5762cc65122680b98d0fd67e9247e4b0d0a6b1dd6"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/specs/providers/spec.md",
          "sha256": "3152725603fd140a20f61d8e03b665f1be16f44c765ea07a7bc5cffab72847f8"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/design.md",
          "sha256": "7729d5778ca633fab5a00ed03004fa5a90eaef63025ef367b3a2edbed0c2416d"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/tasks.md",
          "sha256": "da8b87c5f49402cd696dc7029fe149250395cfe61d83bba76a443401a7cbd70e"
        }
      ],
      "requirement_ids": [
        "REQ-PROV-005",
        "REQ-PROV-006"
      ]
    }
  }
}
~~~

## Plan: cp-06-flow (expected_revision 1, expected_status backlog)

~~~json
{
  "version": 1,
  "task_id": "cp-06-flow",
  "board_id": "custom-providers",
  "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
  "expected_revision": 1,
  "expected_status": "backlog",
  "expected_workflow": "sdd-lite",
  "expected_change_id": "add-custom-providers",
  "expected_spec_plane": "hybrid",
  "definition": {
    "title": "[tui] End-to-end key scheme and provider flow regression oracle",
    "objective": "Add the durable authority-and-navigation regression oracle for the feature in one new dedicated test file: TestREQ_PROV_006_* proves hotkey p opens providers, numeric keys 1-9 are unmoved, m still opens models, and entry 9 is fully cursor/enter navigable with description rendering; TestREQ_PROV_005_* drives the complete List to Input to Preview to Receipt flow through the package-var seams with a temp home and CORTEX_IA_HOME override, asserting Preview performs zero writes and the Receipt never renders token characters. Pure-test task (anti-decomposition rule 6: fix or simplify assertions rather than decompose); it is also the final full-repo gate node for the DAG. Contract: REQ-PROV-001 (surfaced), REQ-PROV-005, REQ-PROV-006.",
    "acceptance_criteria": "- TestREQ_PROV_006_* covers all four key-scheme scenarios from specs/providers/spec.md including the m-survives scenario.\n- TestREQ_PROV_005_* end-to-end flow over faked seams asserts phase order, Preview inertness, and zero token characters in every rendered frame.\n- New file internal/tui/providers_home_test.go <=250 LOC; nothing appended to existing files.\n- Full repo suite green: go test -count=1 ./... with no real-home access (temp dirs + CORTEX_IA_HOME only).\n- Perpetually-green check: any new assertion proven green without the cp-04/cp-05 code must be strengthened before approval.",
    "verification": "go test -count=1 ./internal/tui -run '^TestREQ_PROV_00[156]' && go test -count=1 ./...",
    "allowed_files": [
      "internal/tui/providers_home_test.go"
    ],
    "sdd_contract": {
      "version": 1,
      "workflow": "sdd-lite",
      "change_id": "add-custom-providers",
      "spec_plane": "hybrid",
      "pins": [
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/plan.md",
          "sha256": "f24a44b323cb1a063ab4be5e6f1ae3a6d0513020b3579df877d4ba1fbcac1125"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/proposal.md",
          "sha256": "4be198b0638b76769eea13c5762cc65122680b98d0fd67e9247e4b0d0a6b1dd6"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/specs/providers/spec.md",
          "sha256": "3152725603fd140a20f61d8e03b665f1be16f44c765ea07a7bc5cffab72847f8"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/design.md",
          "sha256": "7729d5778ca633fab5a00ed03004fa5a90eaef63025ef367b3a2edbed0c2416d"
        },
        {
          "transport": "workspace_file",
          "project": "/Volumes/Archivos/github_repositories/lleontor705/cortex-ia",
          "locator": "openspec/changes/add-custom-providers/tasks.md",
          "sha256": "da8b87c5f49402cd696dc7029fe149250395cfe61d83bba76a443401a7cbd70e"
        }
      ],
      "requirement_ids": [
        "REQ-PROV-001",
        "REQ-PROV-005",
        "REQ-PROV-006"
      ]
    }
  }
}
~~~
