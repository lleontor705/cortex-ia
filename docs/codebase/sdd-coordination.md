# SDD Task Coordination with Cortex-IA Work

This document specifies the technical runtime coordination between **OpenCode Agents**, **OpenSpec Specifications**, and the **Cortex-IA Work Engine**.

---

## 1. Work Item Lifecycle State Machine

```text
       ┌──────────────┐
       │   backlog    │ (dependencies unresolved)
       └──────┬───────┘
              │ (all dependencies done)
              ▼
       ┌──────────────┐
       │    ready     │ ◄──────────────────────────────┐
       └──────┬───────┘                                │
              │ (cortex-ia work claim ...)             │
              ▼                                        │ (cortex-ia work retry <id>)
       ┌──────────────┐                                │
       │ in_progress  │                                │
       └──────┬───────┘                                │
              │ (cortex-ia work transition --to in_review)
              ▼                                        │
       ┌──────────────┐                                │
       │  in_review   │                                │
       └──────┬───────┘                                │
              │                                        │
      ┌───────┴────────┐                               │
      ▼                ▼                               │
┌───────────┐    ┌───────────┐                         │
│   done    │    │  blocked  │ ────────────────────────┘
└───────────┘    └───────────┘
(PASS approval)  (FAIL / timeout / recover)
```

---

## 2. Command Execution Patterns

### A. Planner Decomposition
When breaking down an OpenSpec proposal into executable tasks:
```bash
# 1. Initialize Board
cortex-ia board create bootstrap-control-plane "Bootstrap Control Plane"

# 2. Add Task Nodes
cortex-ia work create task-1.1 "Scaffold monorepo" --board bootstrap-control-plane
cortex-ia work create task-1.2 "Domain logic" --board bootstrap-control-plane --depends task-1.1
```

### B. Implementer Lifecycle
```bash
# 1. Check readiness
cortex-ia work status task-1.1

# 2. Claim task (returns claim_token and revision)
cortex-ia work claim task-1.1 --owner implement-agent-1 --ttl 15m

# 3. Reserve exclusive file leases
cortex-ia work lease task-1.1 --claim-token <tok> --path "cmd/server/main.go" --ttl 15m

# 4. Extend lease while editing (heartbeat)
cortex-ia work lease-renew --path "cmd/server/main.go" --lease-token <lease-tok> --ttl 15m

# 5. Transition to review upon test passing
cortex-ia work transition task-1.1 --claim-token <tok> --to in_review
```

### C. Reviewer Verification
```bash
# 1. Run independent test suite
go test ./...

# 2. Record PASS approval (atomically marks done and unlocks task-1.2)
cortex-ia work approve task-1.1 --reviewer reviewer-agent --verdict PASS --evidence "Unit test suite green"
```
