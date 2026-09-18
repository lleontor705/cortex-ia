# Real-World Subagent Dispatch Envelope Example

This example illustrates an intent-preserving subagent dispatch envelope adhering to the IPDP v2.0 protocol.

---

## 1. Coordinator Dispatch Payload

This payload is generated dynamically by an Orchestrator or Planner subagent and injected into the target minion's execution context.

```xml
<minion-dispatch contract_version="2.0">
  <task_id>task-sqlite-wal-checkpoint-042</task_id>
  <role>implement</role>
  
  <intent>
    Add an automated passive WAL checkpoint trigger when WAL file size exceeds 10MB 
    in internal/delegation/sqlite.go.
  </intent>

  <allowed_files>
    <file>internal/delegation/sqlite.go</file>
    <file>internal/delegation/sqlite_test.go</file>
  </allowed_files>

  <non_goals>
    <item>Do NOT modify table schemas, migration ledgers, or add new SQL indexes.</item>
    <item>Do NOT change the default busy_timeout or connection pool limits.</item>
    <item>Do NOT refactor surrounding database helper functions or error wrappers.</item>
    <item>Do NOT touch files in internal/app/ or internal/pipeline/.</item>
  </non_goals>

  <blast_radius>
    <max_loc>80</max_loc>
    <forbidden_couplings>
      <coupling>internal/cortexiaweb</coupling>
      <coupling>cmd/cortex-ia</coupling>
    </forbidden_couplings>
  </blast_radius>

  <verification_oracle>
    go test -count=1 ./internal/delegation -run TestSQLite_PassiveWALCheckpoint
  </verification_oracle>

  <workload_policy>flexible</workload_policy>
</minion-dispatch>
```

---

## 2. Double-Blind Reviewer Envelope

Notice how the reviewer receives the contract and raw diff, but **strictly zero author commentary** or intermediate chat logs.

```xml
<reviewer-dispatch contract_version="2.0">
  <task_id>task-sqlite-wal-checkpoint-042</task_id>
  
  <specification_contract>
    Intent: Add passive WAL checkpoint trigger when WAL size exceeds 10MB.
    Allowed Files: internal/delegation/sqlite.go, internal/delegation/sqlite_test.go
    Non-Goals: No schema changes, no busy_timeout changes, no refactoring outside WAL logic.
  </specification_contract>

  <diff_to_review>
--- a/internal/delegation/sqlite.go
+++ b/internal/delegation/sqlite.go
@@ -45,6 +45,14 @@ func OpenDB(path string) (*sql.DB, error) {
+	// Auto-checkpoint passive WAL when exceeding threshold
+	if walSize > 10*1024*1024 {
+		_, _ = db.Exec("PRAGMA wal_checkpoint(PASSIVE);")
+	}
  </diff_to_review>

  <oracle_execution_receipt>
    Command: go test -count=1 ./internal/delegation -run TestSQLite_PassiveWALCheckpoint
    Exit Code: 0
    Output: PASS
  </oracle_execution_receipt>
</reviewer-dispatch>
```
