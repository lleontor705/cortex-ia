package targets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// File is one destination file written by a Bundle.
type File struct {
	Path    string // absolute destination
	Content []byte
}

// JSONSet sets one key path in a JSON file owned by the user.
type JSONSet struct {
	Path  string // absolute JSON file
	Key   []string
	Value any
}

// Bundle is everything one host needs installed.
type Bundle struct {
	Target    TargetID
	Home      string
	StateDir  string
	Files     []File
	JSONSets  []JSONSet
	OwnedDirs []string
	Validate  func() (string, error)
	Notes     []string
}

// ApplyOptions controls Apply. Overwrite replaces files and JSON values that
// cortex-ia did not write; without it such paths are reported as conflicts.
type ApplyOptions struct {
	DryRun    bool
	Overwrite bool
}

const (
	manifestFileName = "manifest.json"
	backupDirName    = "backups"
	backupStamp      = "20060102T150405.000000000Z"
)

type manifest struct {
	Target      TargetID       `json:"target"`
	InstalledAt string         `json:"installed_at"`
	Files       []manifestFile `json:"files"`
	JSONSets    []manifestJSON `json:"json_sets"`
}

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type manifestJSON struct {
	Path   string   `json:"path"`
	Key    []string `json:"key"`
	SHA256 string   `json:"sha256"`
}

type pendingWrite struct {
	abs     string
	content []byte
}

type pendingJSON struct {
	abs     string
	overlay []byte
	remove  [][]string
}

type pendingBackup struct {
	abs string
	rel string
}

type plan struct {
	home       string
	backupRoot string
	writes     []pendingWrite
	jsons      []pendingJSON
	removes    []string
	backups    []pendingBackup
	backupSeen map[string]bool
	copies     map[string]string
	steps      []undoStep
	rootMade   bool
	changes    []string
	conflicts  []string
	unchanged  int
}

// Installed reports whether a cortex-ia manifest exists for target under home.
func Installed(home string, target TargetID) bool {
	h, err := resolveHome(home)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(stateDirFor(h, target), manifestFileName))
	return err == nil
}

// Apply plans the bundle against the current filesystem and, unless the plan
// has conflicts or DryRun is set, writes it. Pre-existing files are backed up
// before they are changed.
func Apply(b Bundle, opts ApplyOptions) (*TargetResult, error) {
	if err := b.prepare(); err != nil {
		return nil, err
	}
	if !opts.DryRun {
		release, err := lockHome(b.Home)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	mPath := filepath.Join(b.StateDir, manifestFileName)
	prev, err := readManifest(mPath)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p, err := planApply(b, prev, opts, now)
	if err != nil {
		return nil, err
	}
	res := &TargetResult{Target: b.Target, Success: true, Changes: p.changes}
	if len(p.conflicts) > 0 {
		return nil, fmt.Errorf("%s: %d path(s) differ from what cortex-ia wrote and would be overwritten; nothing was written (re-run with Overwrite):\n  - %s",
			TargetLabel(b.Target), len(p.conflicts), strings.Join(p.conflicts, "\n  - "))
	}
	summary := fmt.Sprintf("%d file(s), %d setting(s)", len(b.Files), len(b.JSONSets))
	if opts.DryRun {
		res.Message = fmt.Sprintf("Dry run: %s would be installed (%s)", TargetLabel(b.Target), summary)
		return finishResult(res, b), nil
	}

	// Every write after the first one is covered by the undo log: any later
	// failure, including a failed validation, restores the pre-run state and
	// leaves the previous manifest in place.
	if err := p.run(); err != nil {
		return nil, p.abort(err)
	}
	m, err := buildManifest(b, now)
	if err != nil {
		return nil, p.abort(err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, p.abort(fmt.Errorf("encode manifest: %w", err))
	}
	if err := p.stage(mPath); err != nil {
		return nil, p.abort(err)
	}
	mode, err := filemerge.ExistingMode(mPath, defaultFileMode)
	if err != nil {
		return nil, p.abort(err)
	}
	if _, err := filemerge.WriteFileAtomic(mPath, append(data, '\n'), mode); err != nil {
		return nil, p.abort(fmt.Errorf("write manifest: %w", err))
	}

	res.Message = fmt.Sprintf("Installed %s (%s)", TargetLabel(b.Target), summary)
	if b.Validate != nil {
		out, verr := b.Validate()
		if out != "" {
			res.Message += "\n" + out
		}
		if verr != nil {
			res.Success = false
			return finishResult(res, b), p.abort(fmt.Errorf("%s validation failed: %w", TargetLabel(b.Target), verr))
		}
	}
	return finishResult(res, b), nil
}

// Remove deletes the files and JSON keys recorded in the manifest, keeping any
// that were modified since cortex-ia wrote them.
func Remove(b Bundle, dryRun bool) (*TargetResult, error) {
	if err := b.prepare(); err != nil {
		return nil, err
	}
	if !dryRun {
		release, err := lockHome(b.Home)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	mPath := filepath.Join(b.StateDir, manifestFileName)
	prev, err := readManifest(mPath)
	if err != nil {
		return nil, err
	}
	res := &TargetResult{Target: b.Target, Success: true}
	if prev == nil {
		res.Message = fmt.Sprintf("%s is not installed", TargetLabel(b.Target))
		return res, nil
	}

	p, err := newPlan(b.Home, b.StateDir, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	for _, pf := range prev.Files {
		abs, err := joinHome(b.Home, pf.Path)
		if err != nil {
			return nil, err
		}
		cur, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", pf.Path, err)
		}
		if sha256Hex(cur) != pf.SHA256 {
			p.changes = append(p.changes, "kept modified file "+pf.Path)
			continue
		}
		p.backup(abs)
		p.removes = append(p.removes, abs)
		p.changes = append(p.changes, "remove "+pf.Path)
	}
	for _, pj := range prev.JSONSets {
		abs, err := joinHome(b.Home, pj.Path)
		if err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", pj.Path, err)
		}
		obj, err := filemerge.DecodeJSONObject(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pj.Path, err)
		}
		cur, found, blocked := lookupKey(obj, pj.Key)
		if !found || blocked {
			continue
		}
		curJSON, err := json.Marshal(cur)
		if err != nil {
			return nil, err
		}
		label := dotted(pj.Path, pj.Key)
		if sha256Hex(curJSON) != pj.SHA256 {
			p.changes = append(p.changes, "kept modified key "+label)
			continue
		}
		p.backup(abs)
		p.jsons = append(p.jsons, pendingJSON{abs: abs, remove: [][]string{pj.Key}})
		p.changes = append(p.changes, "remove "+label)
	}
	res.Changes = p.changes

	if dryRun {
		res.Message = fmt.Sprintf("Dry run: %s would be removed", TargetLabel(b.Target))
		return res, nil
	}
	if err := p.run(); err != nil {
		return nil, p.abort(err)
	}
	if err := p.stage(mPath); err != nil {
		return nil, p.abort(err)
	}
	if err := os.Remove(mPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, p.abort(fmt.Errorf("remove manifest: %w", err))
	}
	for _, dir := range b.OwnedDirs {
		removeEmptyDirs(dir)
	}
	res.Message = fmt.Sprintf("Removed %s", TargetLabel(b.Target))
	return res, nil
}

func planApply(b Bundle, prev *manifest, opts ApplyOptions, now time.Time) (*plan, error) {
	p, err := newPlan(b.Home, b.StateDir, now)
	if err != nil {
		return nil, err
	}
	prevFiles := map[string]manifestFile{}
	prevJSON := map[string]manifestJSON{}
	if prev != nil {
		for _, pf := range prev.Files {
			prevFiles[pf.Path] = pf
		}
		for _, pj := range prev.JSONSets {
			prevJSON[keyID(pj.Path, pj.Key)] = pj
		}
	}

	if err := p.planFiles(b, prevFiles, opts); err != nil {
		return nil, err
	}
	if err := p.planJSON(b, prevJSON, opts); err != nil {
		return nil, err
	}
	return p, p.planStale(b, prev, prevFiles, prevJSON)
}

func (p *plan) planFiles(b Bundle, prevFiles map[string]manifestFile, opts ApplyOptions) error {
	for _, f := range b.Files {
		abs := filepath.Clean(f.Path)
		rel, _ := insideHome(b.Home, abs)
		if isSymlink(abs) && !opts.Overwrite {
			if cur, readErr := os.ReadFile(abs); readErr == nil && bytes.Equal(cur, f.Content) {
				p.unchanged++
			} else {
				p.conflicts = append(p.conflicts, rel+" (is a symlink; cortex-ia will not replace it without --overwrite)")
			}
			continue
		}
		cur, err := os.ReadFile(abs)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			p.writes = append(p.writes, pendingWrite{abs: abs, content: f.Content})
			p.changes = append(p.changes, "create "+rel)
		case err != nil:
			return fmt.Errorf("read %s: %w", rel, err)
		case bytes.Equal(cur, f.Content):
			p.unchanged++
		default:
			prev, had := prevFiles[rel]
			ownedDir := underAny(abs, b.OwnedDirs)
			recorded := had && prev.SHA256 == sha256Hex(cur)
			if !ownedDir && !recorded && !opts.Overwrite {
				p.conflicts = append(p.conflicts, rel+" (file differs from the copy cortex-ia wrote)")
				continue
			}
			p.backup(abs)
			p.writes = append(p.writes, pendingWrite{abs: abs, content: f.Content})
			p.changes = append(p.changes, "update "+rel)
		}
	}
	return nil
}

func (p *plan) planJSON(b Bundle, prevJSON map[string]manifestJSON, opts ApplyOptions) error {
	for _, s := range b.JSONSets {
		abs := filepath.Clean(s.Path)
		rel, _ := insideHome(b.Home, abs)
		label := dotted(rel, s.Key)
		raw, err := os.ReadFile(abs)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		obj := map[string]any{}
		if exists {
			if obj, err = filemerge.DecodeJSONObject(raw); err != nil {
				return fmt.Errorf("parse %s: %w", rel, err)
			}
		}
		desired, err := json.Marshal(s.Value)
		if err != nil {
			return fmt.Errorf("encode %s: %w", label, err)
		}

		cur, found, blocked := lookupKey(obj, s.Key)
		var remove [][]string
		switch {
		case !found && !blocked:
		case !blocked:
			curJSON, err := json.Marshal(cur)
			if err != nil {
				return err
			}
			if bytes.Equal(curJSON, desired) {
				p.unchanged++
				continue
			}
			prev, had := prevJSON[keyID(rel, s.Key)]
			recorded := had && prev.SHA256 == sha256Hex(curJSON)
			if !recorded && !opts.Overwrite {
				p.conflicts = append(p.conflicts, label+" (value differs from the one cortex-ia set)")
				continue
			}
			remove = [][]string{s.Key}
		default:
			if !opts.Overwrite {
				p.conflicts = append(p.conflicts, label+" (a parent key is not an object)")
				continue
			}
		}
		if isSymlink(abs) && !opts.Overwrite {
			p.conflicts = append(p.conflicts, label+" (settings file is a symlink; cortex-ia will not replace it without --overwrite)")
			continue
		}
		if exists {
			p.backup(abs)
		}
		overlay, err := json.Marshal(nestKey(s.Key, s.Value))
		if err != nil {
			return fmt.Errorf("encode %s: %w", label, err)
		}
		p.jsons = append(p.jsons, pendingJSON{abs: abs, overlay: overlay, remove: remove})
		p.changes = append(p.changes, "set "+label)
	}
	return nil
}

// planStale removes files and keys that the previous manifest recorded but the
// new bundle no longer contains, again only when they are still unmodified.
func (p *plan) planStale(b Bundle, prev *manifest, prevFiles map[string]manifestFile, prevJSON map[string]manifestJSON) error {
	if prev == nil {
		return nil
	}
	wanted := map[string]bool{}
	for _, f := range b.Files {
		rel, _ := insideHome(b.Home, filepath.Clean(f.Path))
		wanted[rel] = true
	}
	for _, pf := range prevFiles {
		if wanted[pf.Path] {
			continue
		}
		abs, err := joinHome(b.Home, pf.Path)
		if err != nil {
			return err
		}
		cur, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", pf.Path, err)
		}
		if sha256Hex(cur) != pf.SHA256 {
			p.changes = append(p.changes, "kept modified file "+pf.Path)
			continue
		}
		p.backup(abs)
		p.removes = append(p.removes, abs)
		p.changes = append(p.changes, "remove "+pf.Path)
	}
	wantedJSON := map[string]bool{}
	for _, s := range b.JSONSets {
		rel, _ := insideHome(b.Home, filepath.Clean(s.Path))
		wantedJSON[keyID(rel, s.Key)] = true
	}
	for id, pj := range prevJSON {
		if wantedJSON[id] {
			continue
		}
		abs, err := joinHome(b.Home, pj.Path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", pj.Path, err)
		}
		obj, err := filemerge.DecodeJSONObject(raw)
		if err != nil {
			return fmt.Errorf("parse %s: %w", pj.Path, err)
		}
		cur, found, blocked := lookupKey(obj, pj.Key)
		if !found || blocked {
			continue
		}
		curJSON, err := json.Marshal(cur)
		if err != nil {
			return err
		}
		label := dotted(pj.Path, pj.Key)
		if sha256Hex(curJSON) != pj.SHA256 {
			p.changes = append(p.changes, "kept modified key "+label)
			continue
		}
		p.backup(abs)
		p.jsons = append(p.jsons, pendingJSON{abs: abs, remove: [][]string{pj.Key}})
		p.changes = append(p.changes, "remove "+label)
	}
	return nil
}

func newPlan(home, stateDir string, now time.Time) (*plan, error) {
	backupRoot, err := newBackupRoot(stateDir, now)
	if err != nil {
		return nil, err
	}
	return &plan{
		home:       home,
		backupRoot: backupRoot,
		backupSeen: map[string]bool{},
		copies:     map[string]string{},
	}, nil
}

// backup records one pre-change copy of an existing file. Each path is backed
// up at most once per run.
func (p *plan) backup(abs string) {
	if p.backupSeen[abs] {
		return
	}
	p.backupSeen[abs] = true
	rel, _ := insideHome(p.home, abs)
	p.backups = append(p.backups, pendingBackup{abs: abs, rel: rel})
	dest := filepath.Join(p.backupRoot, filepath.FromSlash(rel))
	p.changes = append(p.changes, "backup → "+relSlash(p.home, dest))
}

// run executes the plan in order: backups first, then file writes, JSON
// mutations, and deletions. Each mutation is staged in the undo log first.
func (p *plan) run() error {
	for _, bk := range p.backups {
		if _, err := p.preserve(bk.abs); err != nil {
			return err
		}
	}
	for _, w := range p.writes {
		mode, err := filemerge.ExistingMode(w.abs, defaultFileMode)
		if err != nil {
			return err
		}
		if err := p.stage(w.abs); err != nil {
			return err
		}
		if _, err := filemerge.WriteFileAtomic(w.abs, w.content, mode); err != nil {
			return err
		}
	}
	for _, j := range p.jsons {
		if err := p.stage(j.abs); err != nil {
			return err
		}
		if _, err := filemerge.MutateJSONFile(j.abs, filemerge.JSONMutation{Overlay: j.overlay, RemovePaths: j.remove}); err != nil {
			return err
		}
	}
	for _, r := range p.removes {
		if err := p.stage(r); err != nil {
			return err
		}
		if err := os.Remove(r); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", r, err)
		}
	}
	return nil
}

func (b *Bundle) prepare() error {
	if b.Target == "" {
		return errors.New("bundle target is required")
	}
	home, err := resolveHome(b.Home)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(home) {
		return fmt.Errorf("bundle home %q must be absolute", home)
	}
	b.Home = filepath.Clean(home)
	if b.StateDir == "" {
		b.StateDir = stateDirFor(b.Home, b.Target)
	}
	if _, ok := insideHome(b.Home, b.StateDir); !ok {
		return fmt.Errorf("state dir %q is outside home %q", b.StateDir, b.Home)
	}
	seen := make(map[string]bool, len(b.Files))
	for _, f := range b.Files {
		rel, ok := insideHome(b.Home, f.Path)
		if !ok {
			return fmt.Errorf("file path %q is outside home %q", f.Path, b.Home)
		}
		if seen[rel] {
			return fmt.Errorf("duplicate file destination %q", rel)
		}
		seen[rel] = true
	}
	for _, s := range b.JSONSets {
		if _, ok := insideHome(b.Home, s.Path); !ok {
			return fmt.Errorf("JSON path %q is outside home %q", s.Path, b.Home)
		}
		if len(s.Key) == 0 {
			return fmt.Errorf("JSON set %q has an empty key", s.Path)
		}
		for _, k := range s.Key {
			if k == "" {
				return fmt.Errorf("JSON set %q has an empty key segment", s.Path)
			}
		}
	}
	for _, d := range b.OwnedDirs {
		if _, ok := insideHome(b.Home, d); !ok {
			return fmt.Errorf("owned directory %q is outside home %q", d, b.Home)
		}
	}
	return nil
}

func finishResult(res *TargetResult, b Bundle) *TargetResult {
	if len(b.Notes) > 0 {
		res.Message += "\n" + strings.Join(b.Notes, "\n")
	}
	return res
}

func buildManifest(b Bundle, now time.Time) (manifest, error) {
	m := manifest{Target: b.Target, InstalledAt: now.Format(time.RFC3339), Files: []manifestFile{}, JSONSets: []manifestJSON{}}
	for _, f := range b.Files {
		rel, _ := insideHome(b.Home, filepath.Clean(f.Path))
		m.Files = append(m.Files, manifestFile{Path: rel, SHA256: sha256Hex(f.Content)})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	for _, s := range b.JSONSets {
		rel, _ := insideHome(b.Home, filepath.Clean(s.Path))
		canon, err := json.Marshal(s.Value)
		if err != nil {
			return manifest{}, fmt.Errorf("encode %s: %w", dotted(rel, s.Key), err)
		}
		m.JSONSets = append(m.JSONSets, manifestJSON{Path: rel, Key: s.Key, SHA256: sha256Hex(canon)})
	}
	return m, nil
}

func readManifest(path string) (*manifest, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s (remove it to reset tracking): %w", path, err)
	}
	return &m, nil
}

func stateDirFor(home string, target TargetID) string {
	return filepath.Join(home, ".cortex-ia", "targets", string(target))
}

func resolveHome(home string) (string, error) {
	if home != "" {
		return home, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return h, nil
}

// insideHome returns the slash-separated path of p relative to home, or false
// when p is not absolute or escapes home.
func insideHome(home, p string) (string, bool) {
	if !filepath.IsAbs(p) {
		return "", false
	}
	rel, err := filepath.Rel(home, filepath.Clean(p))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// joinHome resolves a manifest-recorded slash path beneath home, rejecting any
// value that would escape it.
func joinHome(home, rel string) (string, error) {
	abs := filepath.Join(home, filepath.FromSlash(rel))
	if _, ok := insideHome(home, abs); !ok {
		return "", fmt.Errorf("manifest path %q escapes home", rel)
	}
	return abs, nil
}

func relSlash(home, p string) string {
	rel, err := filepath.Rel(home, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}

func underAny(abs string, dirs []string) bool {
	for _, d := range dirs {
		if _, ok := insideHome(filepath.Clean(d), abs); ok {
			return true
		}
	}
	return false
}

// lookupKey reports the value at key, whether it exists, and whether a parent
// on the path exists but is not an object (which blocks a safe write).
func lookupKey(obj map[string]any, key []string) (value any, found bool, blocked bool) {
	var cur any = obj
	for i, k := range key {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, true
		}
		v, ok := m[k]
		if !ok {
			return nil, false, false
		}
		if i == len(key)-1 {
			return v, true, false
		}
		cur = v
	}
	return nil, false, false
}

func nestKey(key []string, value any) map[string]any {
	out := value
	for i := len(key) - 1; i >= 0; i-- {
		out = map[string]any{key[i]: out}
	}
	return out.(map[string]any)
}

func removeEmptyDirs(dir string) {
	if _, err := os.Stat(dir); err != nil {
		return
	}
	var dirs []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func keyID(path string, key []string) string {
	return path + "\x00" + strings.Join(key, "\x00")
}

func dotted(rel string, key []string) string {
	return rel + ":" + strings.Join(key, ".")
}
