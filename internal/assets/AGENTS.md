# OpenCode Multi-Workflow Bundle

- Primary Engine: Multi-Workflow Dispatcher
- Version: `1.1.0`
- Supported Workflows: `workflow/sdd`, `workflow/fast-tdd`, `workflow/hotfix`, `workflow/spike`, `workflow/review`
- Profile: `portable-flat`

## Execution Model
Delegate each ready phase directly to its role agent. Do not request nested delegation.

## Supported Workflows

### 1. Spec-Driven Development (`workflow/sdd`)
For cross-cutting, multi-domain, architectural changes (>2 files):
- `phase/bootstrap` -> `role/bootstrap`
- `phase/investigate` -> `role/investigate` (after: `phase/bootstrap`)
- `phase/propose` -> `role/draft-proposal` (after: `phase/investigate`)
- `phase/design` -> `role/architect` (after: `phase/propose`)
- `phase/spec` -> `role/write-specs` (after: `phase/propose`)
- `phase/tasks` -> `role/decompose` (after: `phase/design`, `phase/spec`)
- `phase/apply` -> `role/implement` (after: `phase/tasks`)
- `phase/verify` -> `role/validate` (after: `phase/apply`)
- `phase/archive` -> `role/finalize` (after: `phase/verify`)

### 2. Fast-TDD Micro-Loop (`workflow/fast-tdd`)
For localized features, pure functions, and unit bugfixes (<=2 files):
- `phase/tdd-loop` -> `role/implement` (skill: `fast-tdd`)

### 3. Hotfix / Emergency Patch (`workflow/hotfix`)
For critical defect resolution with strict diff boundaries (<=50 lines):
- `phase/hotfix` -> `role/implement` (skill: `hotfix-triage`)

### 4. Technical Spike (`workflow/spike`)
For exploratory prototyping and performance benchmarking:
- `phase/spike` -> `role/investigate` (skill: `spike-prototype`)

### 5. Adversarial Code Review (`workflow/review`)
For independent security, regression, and quality auditing:
- `phase/review` -> `role/reviewer` (skill: `code-review-adversary`)
