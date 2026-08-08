# Filesystem Safety Delta Specification

## ADDED Requirements

### REQ-FS-001: Destructive names are single path segments

Any CLI value used to remove a named managed object MUST be a non-empty, single path segment. It MUST reject `.`, `..`, absolute paths, volume-qualified paths, and values containing `/` or `\` before filesystem access.

Primary oracle: `internal/app` table-driven tests using `t.TempDir()` and external sentinel files.

#### Scenario: Valid community skill name

- **Given** a community skill named `go-review` inside the managed skill root
- **When** the user runs `skill remove go-review`
- **Then** only that skill directory is removed

#### Scenario: Traversal-like name

- **Given** a sentinel outside the managed skill root
- **When** the user supplies `..`, `../x`, `..\x`, or an absolute path
- **Then** the command returns a non-zero error and the complete temporary home remains unchanged

#### Scenario: Missing valid name

- **Given** a syntactically valid name that does not exist
- **When** removal is requested
- **Then** the command reports that the managed object was not found without creating or deleting paths

Coverage: missing until adversarial path tests are added.

### REQ-FS-002: Mutations remain beneath trusted roots

Every create, write, rename, restore, or delete operation MUST derive its authority from a trusted root supplied by application code. Containment MUST use canonical absolute paths and `filepath.Rel`, not string prefixes. Existing symlinks or Windows reparse points in any path component MUST cause the operation to fail closed.

Primary oracle: a shared secure-filesystem package test matrix plus integration tests for backup, install, and uninstall.

#### Scenario: Regular path beneath an adapter root

- **Given** a regular directory tree beneath a temporary adapter root
- **When** a managed file is written
- **Then** the write succeeds and no path outside that root changes

#### Scenario: Similar prefix outside the root

- **Given** trusted root `/home/test/.claude` and target `/home/test/.claude-evil/file`
- **When** containment is validated
- **Then** the target is rejected as outside the trusted root

#### Scenario: Parent symlink escape

- **Given** a parent component beneath the managed root that links outside the temporary home
- **When** install, uninstall, or restore attempts a mutation through it
- **Then** the operation fails before mutation and the external sentinel remains unchanged

Coverage: partial on Windows until reparse-point tests run with required privileges.

### REQ-FS-003: Atomic writes preserve existing permissions

Atomic replacement MUST preserve the mode of an existing regular file. It MUST NOT chmod an existing parent directory. New private metadata roots and files MUST default to `0700` and `0600`; newly created public configuration paths MAY use explicitly requested modes.

Primary oracle: `internal/components/filemerge` mode-preservation tests on supported platforms.

#### Scenario: Replace a private file

- **Given** an existing regular file with mode `0600`
- **When** its content is replaced atomically
- **Then** the new content is present and the mode remains `0600`

#### Scenario: Write beneath a private directory

- **Given** an existing parent directory with mode `0700`
- **When** a child configuration file is written
- **Then** the parent remains `0700`

#### Scenario: Atomic rename fails

- **Given** a simulated rename failure
- **When** an atomic write is attempted
- **Then** the previous file remains intact and the temporary file is removed

Coverage: missing for failure injection and Windows mode semantics.
