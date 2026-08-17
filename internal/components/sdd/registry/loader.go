package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Load-stage rule names (see normalize.go for the normalize-stage rules).
// Diagnostics cite these instead of restating the verification sequence so
// reports stay stable across policy revisions.
const (
	// RuleConfigProvenance guards local provenance of the declaring
	// configuration file.
	RuleConfigProvenance = "config-source-local-provenance"
	// RuleSkillProvenance guards local provenance of one declared custom
	// skill source.
	RuleSkillProvenance = "skill-source-local-provenance"
	// RuleSkillContainment guards containment of a verified source beneath
	// the canonical configuration root. It blocks directory traversal and
	// symlink escape because containment always inspects fully resolved
	// paths.
	RuleSkillContainment = "skill-source-containment"
	// RuleSkillSingleSkillMD guards the exactly-one-regular-SKILL.md
	// structure of a verified custom skill directory.
	RuleSkillSingleSkillMD = "skill-source-single-skill-md"
)

// SkillMDName is the one declaration file name accepted inside a verified
// custom skill directory (design D2). The match is case-sensitive so the
// fact is identical on every platform.
const SkillMDName = "SKILL.md"

// Known-safe remediations cited by load-stage diagnostics. They only ever
// name actions on the declared local configuration; the loader never
// invents remediations that touch anything else.
const (
	remediationConfigSource  = "point the configuration at an existing regular file on local disk"
	remediationSkillSource   = "declare an existing local directory as the custom skill path"
	remediationContainment   = "place the custom skill directory inside the configuration directory"
	remediationSingleSkillMD = "add exactly one regular SKILL.md file to the custom skill directory"
)

// Loaded is the outcome of verifying and reading every source declared by
// a Request. It carries the immutable bytes read from resolved handles
// plus record-only evidence; it never grants authority, permission, or
// trust (design D3).
type Loaded struct {
	// HasConfigSource reports whether the request declared a
	// configuration file at all.
	HasConfigSource bool
	// ConfigSource records the verification outcome of the declared
	// configuration file; meaningful only when HasConfigSource is true.
	ConfigSource Evidence
	// Sources holds exactly one entry per declared custom skill path, in
	// declaration order, verified or not, so the recorded provenance
	// covers every declared local source.
	Sources []LoadedSource
}

// LoadedSource pairs one declared custom skill path with the immutable
// bytes read from its resolved handle.
type LoadedSource struct {
	// DeclarationIndex is the index of the declaring entry in
	// RegistrySelection.CustomSkillPaths.
	DeclarationIndex int
	// Evidence records the verification outcome for this source. It is a
	// recorded fact, never an authority grant.
	Evidence Evidence
	// Raw is the content of the single regular SKILL.md inside the
	// verified skill directory, read from the resolved handle so later
	// stages never re-read the filesystem. It is nil whenever
	// Evidence.Verified is false.
	Raw []byte
}

// Load verifies the local provenance of every source declared in req from
// canonical filesystem facts (design D3): the configuration file and each
// declared custom skill directory must resolve through
// Abs+Lstat+EvalSymlinks, must be a regular file and a directory
// respectively, every read target must remain strictly beneath the
// canonical configuration root — which blocks directory traversal and
// symlink escape — and content is only ever read through opened handles on
// those resolved paths.
//
// The returned Evidence records are facts for the audit trail; they never
// authorize anything. Failures produce ErrorUntrusted diagnostics
// (provenance or containment) or ErrorInvalid diagnostics (skill directory
// structure), never a filesystem mutation.
func Load(req Request) (Loaded, Diagnostics) {
	loaded := Loaded{Sources: make([]LoadedSource, 0, len(req.Selection.CustomSkillPaths))}
	var diags Diagnostics

	root := ""
	if declared := req.Selection.ConfigFile; declared != "" {
		loaded.HasConfigSource = true
		evidence, resolved, diag := loadConfigSource(declared)
		loaded.ConfigSource = evidence
		if diag != nil {
			// Without a verified configuration source there is no
			// canonical root, so no declared skill path can have local
			// provenance: fail fast with the single deterministic cause.
			return loaded, append(diags, *diag)
		}
		root = filepath.Dir(resolved)
	}

	for index, declared := range req.Selection.CustomSkillPaths {
		source, diag := loadSkillSource(root, declared, index)
		loaded.Sources = append(loaded.Sources, source)
		if diag != nil {
			diags = append(diags, *diag)
		}
	}
	return loaded, diags
}

// loadConfigSource verifies one declared configuration file and returns
// its evidence, its resolved path (which defines the canonical
// configuration root), and a diagnostic on failure.
func loadConfigSource(declared string) (Evidence, string, *Diagnostic) {
	evidencePath := filepath.ToSlash(filepath.Clean(declared))
	fail := func(cause error) (Evidence, string, *Diagnostic) {
		evidence := Evidence{
			Kind:               EvidenceConfigSource,
			Verified:           false,
			ConfigRelativePath: evidencePath,
		}
		return evidence, "", loadDiagnostic(ErrorUntrusted, RuleConfigProvenance, remediationConfigSource, 0, cause)
	}

	abs, resolved, err := resolveCanonical(declared)
	if err != nil {
		return fail(err)
	}
	evidencePath = filepath.ToSlash(filepath.Base(resolved))
	content, regular, err := readRegularFrom(resolved)
	if err != nil {
		return fail(err)
	}
	if !regular {
		return fail(fmt.Errorf("resolved configuration source %q is not a regular file", abs))
	}
	evidence := Evidence{
		Kind:               EvidenceConfigSource,
		Verified:           true,
		ConfigRelativePath: evidencePath,
		ContentSHA256:      digestContent(content),
	}
	return evidence, resolved, nil
}

// loadSkillSource verifies one declared custom skill path against the
// canonical configuration root and reads the immutable SKILL.md bytes from
// the resolved handle. Relative declarations anchor at the verified
// configuration root — the directory of the resolved configuration file —
// never at the process working directory, while absolute declarations keep
// their explicit spelling (design D5). An empty root means no local
// provenance anchor exists, so every declaration is rejected as untrusted.
func loadSkillSource(root, declared string, index int) (LoadedSource, *Diagnostic) {
	evidencePath := filepath.ToSlash(filepath.Clean(declared))
	fail := func(class ErrorClass, rule, remediation string, cause error) (LoadedSource, *Diagnostic) {
		source := LoadedSource{
			DeclarationIndex: index,
			Evidence: Evidence{
				Kind:               EvidenceSkillSource,
				Verified:           false,
				ConfigRelativePath: evidencePath,
			},
		}
		return source, loadDiagnostic(class, rule, remediation, index, cause)
	}

	if declared == "" {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, errors.New("custom skill path declaration is empty"))
	}
	if root == "" {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, errors.New("no verified local configuration source anchors the custom skill path"))
	}
	// Anchor relative declarations at the verified canonical configuration
	// root so resolution never depends on the process working directory.
	// Absolute declarations keep their declared spelling: their trust
	// behavior (verified then containment-checked beneath the same root)
	// is unchanged.
	anchored := declared
	if !filepath.IsAbs(anchored) {
		anchored = filepath.Join(root, anchored)
	}
	_, resolved, err := resolveCanonical(anchored)
	if err != nil {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, err)
	}
	rel, contained := relativeBeneath(root, resolved)
	if !contained {
		return fail(ErrorUntrusted, RuleSkillContainment, remediationContainment, errors.New("resolved custom skill path escapes the canonical configuration root"))
	}
	evidencePath = rel

	// Regularity and directory listing facts come from the opened handle
	// on the resolved path, never from the declared spelling.
	dir, err := os.Open(resolved)
	if err != nil {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("open resolved skill directory: %w", err))
	}
	info, statErr := dir.Stat()
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	switch {
	case statErr != nil:
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("stat resolved skill directory: %w", statErr))
	case !info.IsDir():
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, errors.New("resolved custom skill source is not a directory"))
	case readErr != nil:
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("read resolved skill directory: %w", readErr))
	case closeErr != nil:
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("close resolved skill directory: %w", closeErr))
	}
	if !slices.ContainsFunc(entries, func(entry os.DirEntry) bool { return entry.Name() == SkillMDName }) {
		return fail(ErrorInvalid, RuleSkillSingleSkillMD, remediationSingleSkillMD, fmt.Errorf("custom skill directory declares no %s", SkillMDName))
	}

	// The single SKILL.md is itself a read target: it must resolve, stay
	// beneath the root, and be a regular file before its bytes are read
	// through the resolved handle.
	resolvedSkillMD, err := filepath.EvalSymlinks(filepath.Join(resolved, SkillMDName))
	if err != nil {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("resolve %s: %w", SkillMDName, err))
	}
	if _, mdContained := relativeBeneath(root, resolvedSkillMD); !mdContained {
		return fail(ErrorUntrusted, RuleSkillContainment, remediationContainment, fmt.Errorf("resolved %s escapes the canonical configuration root", SkillMDName))
	}
	content, regular, err := readRegularFrom(resolvedSkillMD)
	if err != nil {
		return fail(ErrorUntrusted, RuleSkillProvenance, remediationSkillSource, fmt.Errorf("read %s: %w", SkillMDName, err))
	}
	if !regular {
		return fail(ErrorInvalid, RuleSkillSingleSkillMD, remediationSingleSkillMD, fmt.Errorf("custom skill directory %s is not a regular file", SkillMDName))
	}

	return LoadedSource{
		DeclarationIndex: index,
		Evidence: Evidence{
			Kind:               EvidenceSkillSource,
			Verified:           true,
			ConfigRelativePath: rel,
			ContentSHA256:      digestContent(content),
		},
		Raw: content,
	}, nil
}

// loadDiagnostic builds a load-stage diagnostic for the declaring custom
// skill path index (0 for the configuration source, which has no
// declaration of its own). The skill ID stays nil because the load stage
// never parses IDs: an unverified source is never given an invented one.
func loadDiagnostic(class ErrorClass, rule, remediation string, index int, cause error) *Diagnostic {
	return &Diagnostic{
		Class:            class,
		Stage:            StageLoad,
		Rule:             rule,
		DeclarationIndex: index,
		SafeRemediation:  remediation,
		Cause:            fmt.Errorf("rule %s: %w", rule, cause),
	}
}

// resolveCanonical performs the design D3 canonical fact sequence on a
// declared path: lexical absolutization, an lstat of the declared
// spelling, then full symlink resolution. Every step must succeed before
// any trust decision is made. It makes no containment or regularity
// decision; those belong to the callers.
func resolveCanonical(declared string) (abs, resolved string, err error) {
	abs, err = filepath.Abs(declared)
	if err != nil {
		return "", "", fmt.Errorf("make declared path absolute: %w", err)
	}
	if _, err = os.Lstat(abs); err != nil {
		return abs, "", fmt.Errorf("lstat declared path: %w", err)
	}
	resolved, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, "", fmt.Errorf("resolve symlinks: %w", err)
	}
	return abs, resolved, nil
}

// relativeBeneath returns the slash-separated path of target relative to
// root when target is strictly beneath root, and ok=false when target
// equals, escapes, or cannot be related to root. Both inputs must already
// be fully resolved: because containment inspects canonical paths only,
// lexical traversal (".." spellings) and symlink escape both fail here.
func relativeBeneath(root, target string) (rel string, ok bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", false
	}
	if relative == "." || relative == ".." || filepath.IsAbs(relative) ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// readRegularFrom opens resolved and reads its content through the opened
// handle only. It distinguishes a structural non-regular fact
// (regular=false with nil error) from an I/O failure so callers classify
// invalid structure versus unverifiable provenance.
func readRegularFrom(resolved string) (content []byte, regular bool, err error) {
	handle, err := os.Open(resolved)
	if err != nil {
		return nil, false, fmt.Errorf("open resolved source: %w", err)
	}
	info, statErr := handle.Stat()
	content, readErr := io.ReadAll(handle)
	closeErr := handle.Close()
	switch {
	case statErr != nil:
		return nil, false, fmt.Errorf("stat resolved handle: %w", statErr)
	case !info.Mode().IsRegular():
		return nil, false, nil
	case readErr != nil:
		return nil, false, fmt.Errorf("read resolved handle: %w", readErr)
	case closeErr != nil:
		return nil, false, fmt.Errorf("close resolved handle: %w", closeErr)
	}
	return content, true, nil
}

// digestContent hashes the exact bytes read from a resolved handle as a
// bare lowercase hex string. It deliberately digests raw bytes: content
// canonicalization (UTF-8/LF) is normalize-stage policy and produces the
// separate Skill.ContentSHA256.
func digestContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
