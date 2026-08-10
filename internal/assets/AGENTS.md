# OpenCode Workflow Bundle

- Workflow: `workflow/sdd`
- Version: `1.0.0`
- Profile: `portable-flat`
- Generation fingerprint: `c61e7dc324f29d790c80af76633020b0ae52588710ea884511a1d3793aa11cdb`

## Execution
Delegate each ready phase directly to its role agent. Do not request nested delegation.

## Phases
- `phase/bootstrap` -> `role/bootstrap`
- `phase/investigate` -> `role/investigate` (after: `phase/bootstrap`)
- `phase/propose` -> `role/draft-proposal` (after: `phase/investigate`)
- `phase/design` -> `role/architect` (after: `phase/propose`)
- `phase/spec` -> `role/write-specs` (after: `phase/propose`)
- `phase/tasks` -> `role/decompose` (after: `phase/design`, `phase/spec`)
- `phase/apply` -> `role/implement` (after: `phase/tasks`)
- `phase/verify` -> `role/validate` (after: `phase/apply`)
- `phase/archive` -> `role/finalize` (after: `phase/verify`)
