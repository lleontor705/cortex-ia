package targets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/v2/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/v2/internal/homelock"
)

// hostLockTimeout bounds the wait for the canonical home lock that the
// OpenCode install pipeline also takes, so host and OpenCode mutations of one
// home never interleave.
const hostLockTimeout = 5 * time.Second

// defaultFileMode is the mode for files a bundle creates. Rewrites keep the
// mode already on disk, so a private file never becomes world-readable.
const defaultFileMode fs.FileMode = 0o644

func lockHome(home string) (func(), error) {
	lock, err := homelock.Acquire(home, hostLockTimeout)
	if err != nil {
		return nil, fmt.Errorf("acquire home lock: %w", err)
	}
	return func() { _ = lock.Release() }, nil
}

// undoStep is the inverse of one mutation. At most one of link or backup is
// set: a recreated symlink, or a restored copy. Neither means the path did
// not exist before the run, so its removal is the inverse.
type undoStep struct {
	abs         string
	backup      string
	link        string
	createdDirs []string
}

// newBackupRoot names one run's backup directory. The timestamp alone is not
// unique under coarse clocks, so a random suffix separates runs that share it.
func newBackupRoot(stateDir string, now time.Time) (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate backup id: %w", err)
	}
	return filepath.Join(stateDir, backupDirName, now.UTC().Format(backupStamp)+"-"+hex.EncodeToString(buf[:])), nil
}

// preserve copies abs into this run's backup tree with its source mode, so a
// private file's copy is exactly as private. Each path is copied once.
func (p *plan) preserve(abs string) (string, error) {
	if dest, ok := p.copies[abs]; ok {
		return dest, nil
	}
	rel, ok := insideHome(p.home, abs)
	if !ok {
		return "", fmt.Errorf("backup path %q is outside home", abs)
	}
	if err := p.ensureRoot(); err != nil {
		return "", err
	}
	dest := filepath.Join(p.backupRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", fmt.Errorf("create backup directory for %s: %w", rel, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("back up %s: %w", rel, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("back up %s: %w", rel, err)
	}
	if _, err := filemerge.WriteFileAtomic(dest, data, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("back up %s: %w", rel, err)
	}
	p.copies[abs] = dest
	return dest, nil
}

// ensureRoot creates the run's backup root exclusively and privately; an
// existing root means another run owns it, so this run fails closed.
func (p *plan) ensureRoot() error {
	if p.rootMade {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.backupRoot), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Mkdir(p.backupRoot, 0o700); err != nil {
		return fmt.Errorf("create backup directory %s: %w", p.backupRoot, err)
	}
	p.rootMade = true
	return nil
}

// stage records the inverse of the mutation about to happen on abs. It must
// run before the mutation, so the undo log describes the pre-run state.
func (p *plan) stage(abs string) error {
	step := undoStep{abs: abs}
	info, err := os.Lstat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		step.createdDirs = missingDirsUnder(p.home, abs)
	case err != nil:
		return fmt.Errorf("inspect %s: %w", abs, err)
	case info.Mode()&os.ModeSymlink != 0:
		target, linkErr := os.Readlink(abs)
		if linkErr != nil {
			return fmt.Errorf("read symlink %s: %w", abs, linkErr)
		}
		step.link = target
	default:
		dest, backupErr := p.preserve(abs)
		if backupErr != nil {
			return backupErr
		}
		step.backup = dest
	}
	p.steps = append(p.steps, step)
	return nil
}

// undo reverts every staged step in reverse order. A failed step does not stop
// the others, so one bad path cannot strand the rest in a half-written state.
func (p *plan) undo() error {
	var failures []string
	for i := len(p.steps) - 1; i >= 0; i-- {
		if err := p.steps[i].revert(); err != nil {
			failures = append(failures, err.Error())
		}
	}
	p.steps = nil
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (s undoStep) revert() error {
	switch {
	case s.link != "":
		_ = os.Remove(s.abs)
		if err := os.Symlink(s.link, s.abs); err != nil {
			return fmt.Errorf("restore symlink %s: %w", s.abs, err)
		}
	case s.backup == "":
		if err := os.Remove(s.abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", s.abs, err)
		}
		for _, dir := range s.createdDirs {
			_ = os.Remove(dir)
		}
	default:
		info, err := os.Stat(s.backup)
		if err != nil {
			return fmt.Errorf("read backup for %s: %w", s.abs, err)
		}
		data, err := os.ReadFile(s.backup)
		if err != nil {
			return fmt.Errorf("read backup for %s: %w", s.abs, err)
		}
		if _, err := filemerge.WriteFileAtomic(s.abs, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("restore %s: %w", s.abs, err)
		}
	}
	return nil
}

// abort reverts the run after a failure at or after the first write and
// reports the restoration verdict beside the cause. The backup root is kept.
func (p *plan) abort(cause error) error {
	if err := p.undo(); err != nil {
		return fmt.Errorf("%w; restoring the previous state failed (backups kept in %s): %v", cause, p.backupRoot, err)
	}
	return fmt.Errorf("%w; previous state restored (backups in %s)", cause, p.backupRoot)
}

// missingDirsUnder lists the directories above abs that do not exist yet,
// deepest first, stopping at home so only transaction-created directories are
// ever removed.
func missingDirsUnder(home, abs string) []string {
	var missing []string
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(dir); err == nil || !errors.Is(err, fs.ErrNotExist) {
			break
		}
		if _, ok := insideHome(home, dir); !ok {
			break
		}
		missing = append(missing, dir)
	}
	return missing
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
