package delegation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/openspec"
)

type ArchiveOptions struct {
	BoardID   string `json:"board_id"`
	Workspace string `json:"workspace"`
	ChangeID  string `json:"change_id"`
	Workflow  string `json:"workflow"`
	SpecPlane string `json:"spec_plane"`
}

type ArchiveReceipt struct {
	ArchiveID string `json:"archive_id"`
	ArchiveOptions
	TaskIDs        []string `json:"task_ids"`
	BindingSHA256  string   `json:"binding_sha256"`
	ManifestSHA256 string   `json:"manifest_sha256,omitempty"`
	Destination    string   `json:"destination,omitempty"`
	ClosedAt       string   `json:"closed_at"`
	Idempotent     bool     `json:"idempotent"`
}

var archiveChangeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ArchiveChange records intent before moving files. A crash after rename leaves
// a prepared record; the next identical call verifies destination bytes and
// current approval bindings before completing that record. Cortex is logical only.
func (s *Store) ArchiveChange(ctx context.Context, opts ArchiveOptions) (ArchiveReceipt, error) {
	var receipt ArchiveReceipt
	if opts.BoardID == "" || !archiveChangeID.MatchString(opts.ChangeID) || opts.ChangeID == "archive" {
		return receipt, fmt.Errorf("board and valid change ID are required")
	}
	if opts.Workflow != "sdd-lite" && opts.Workflow != "sdd-full" {
		return receipt, fmt.Errorf("only SDD Lite/Full initiatives can be archived")
	}
	if opts.SpecPlane != "openspec" && opts.SpecPlane != "hybrid" && opts.SpecPlane != "cortex" {
		return receipt, fmt.Errorf("invalid specification plane")
	}
	workspace, err := CanonicalWorkspace(opts.Workspace)
	if err != nil {
		return receipt, err
	}
	opts.Workspace = workspace
	raw, _ := json.Marshal(opts)
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:])
	source, destination := "", ""
	if opts.SpecPlane != "cortex" {
		source = filepath.Join(workspace, "openspec", "changes", opts.ChangeID)
		destination = filepath.Join(workspace, "openspec", "changes", "archive", opts.ChangeID+"-"+key[:12])
	}
	// Commit a prepared record independently so rollback after a filesystem move
	// cannot erase the information needed to recover it.
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var priorStatus, priorBinding, priorManifest, priorSource, priorDestination string
		err := conn.QueryRowContext(ctx, `SELECT status,binding_sha256,manifest_sha256,source_path,destination_path FROM change_archives WHERE archive_key=?`, key).Scan(&priorStatus, &priorBinding, &priorManifest, &priorSource, &priorDestination)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		reprepare := err == nil
		if reprepare {
			if priorSource != source || priorDestination != destination {
				return fmt.Errorf("archive paths conflict with durable intent")
			}
			if priorStatus != "prepared" {
				return nil
			}
			if source != "" {
				sourceExists, err := archiveExists(source)
				if err != nil {
					return err
				}
				destinationExists, err := archiveExists(destination)
				if err != nil {
					return err
				}
				if !sourceExists || destinationExists {
					return nil
				}
			}
		}
		manifest := ""
		if source != "" {
			structure, err := validateArchiveStructure(workspace, opts.ChangeID, opts.Workflow)
			if err != nil {
				return err
			}
			for _, id := range structure.TaskIDs {
				var count int
				if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM work_items WHERE id=? AND board_id=?`, id, opts.BoardID).Scan(&count); err != nil {
					return err
				}
				if count != 1 {
					return fmt.Errorf("planned task %s is not materialized in archive board", id)
				}
			}
			manifest, err = archiveManifest(source)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(destination); !os.IsNotExist(err) {
				return fmt.Errorf("archive destination already exists or cannot be inspected")
			}
		}
		binding, err := s.validateArchiveBoard(ctx, conn, opts.BoardID, workspace, opts.ChangeID, opts.Workflow, opts.SpecPlane)
		if err != nil {
			return err
		}
		now := s.timestamp()
		if reprepare {
			if priorBinding == binding.SHA256 && priorManifest != manifest {
				return fmt.Errorf("archive bytes changed without a fresh approval binding")
			}
			_, err = conn.ExecContext(ctx, `UPDATE change_archives SET binding_sha256=?,manifest_sha256=?,updated_at=? WHERE archive_key=? AND status='prepared'`, binding.SHA256, manifest, now, key)
			return err
		}
		_, err = conn.ExecContext(ctx, `INSERT INTO change_archives(archive_key,board_id,workspace,change_id,workflow,spec_plane,binding_sha256,manifest_sha256,source_path,destination_path,status,receipt_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,'prepared','',?,?)`, key, opts.BoardID, workspace, opts.ChangeID, opts.Workflow, opts.SpecPlane, binding.SHA256, manifest, source, destination, now, now)
		return err
	})
	if err != nil {
		return receipt, err
	}
	err = s.immediate(ctx, func(conn *sql.Conn) error {
		var status, bindingSHA, manifest, storedSource, storedDestination, receiptJSON string
		if err := conn.QueryRowContext(ctx, `SELECT status,binding_sha256,manifest_sha256,source_path,destination_path,receipt_json FROM change_archives WHERE archive_key=?`, key).Scan(&status, &bindingSHA, &manifest, &storedSource, &storedDestination, &receiptJSON); err != nil {
			return err
		}
		if storedSource != source || storedDestination != destination {
			return fmt.Errorf("archive paths conflict with durable intent")
		}
		moved := false
		if source != "" {
			sourceExists, err := archiveExists(source)
			if err != nil {
				return err
			}
			destExists, err := archiveExists(destination)
			if err != nil {
				return err
			}
			if sourceExists == destExists {
				return fmt.Errorf("archive requires exactly one of source or destination to exist")
			}
			moved = destExists
			check := source
			if moved {
				check = destination
			}
			actual, err := archiveManifest(check)
			if err != nil {
				return err
			}
			if actual != manifest {
				return fmt.Errorf("archive bytes changed since prepared intent")
			}
			if status == "complete" && !moved {
				return fmt.Errorf("completed archive destination is missing")
			}
		}
		var binding ArchiveBinding
		var err error
		if moved {
			binding, err = s.validateArchiveBoardAt(ctx, conn, opts.BoardID, workspace, opts.ChangeID, opts.Workflow, opts.SpecPlane, source, destination)
		} else {
			binding, err = s.validateArchiveBoard(ctx, conn, opts.BoardID, workspace, opts.ChangeID, opts.Workflow, opts.SpecPlane)
		}
		if err != nil {
			return err
		}
		if binding.SHA256 != bindingSHA {
			return fmt.Errorf("archive approval binding changed; fresh review is required")
		}
		if status == "complete" {
			if err := json.Unmarshal([]byte(receiptJSON), &receipt); err != nil {
				return err
			}
			receipt.Idempotent = true
			return nil
		}
		if status != "prepared" {
			return fmt.Errorf("unknown archive state")
		}
		if source != "" && !moved {
			if _, err := validateArchiveStructure(workspace, opts.ChangeID, opts.Workflow); err != nil {
				return err
			}
			parent := filepath.Dir(destination)
			if err := os.MkdirAll(parent, 0755); err != nil {
				return err
			}
			if err := archiveRealPath(parent); err != nil {
				return fmt.Errorf("archive directory may not traverse symbolic links")
			}
			if _, err := os.Lstat(destination); !os.IsNotExist(err) {
				return fmt.Errorf("archive destination conflicts with prepared intent")
			}
			if err := os.Rename(source, destination); err != nil {
				return fmt.Errorf("move OpenSpec change: %w", err)
			}
			actual, err := archiveManifest(destination)
			if err != nil {
				return err
			}
			if actual != manifest {
				return fmt.Errorf("archive bytes changed during move; prepared recovery required")
			}
		}
		receipt = ArchiveReceipt{ArchiveID: key, ArchiveOptions: opts, TaskIDs: binding.TaskIDs, BindingSHA256: bindingSHA, ManifestSHA256: manifest, Destination: destination, ClosedAt: s.timestamp()}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, `UPDATE change_archives SET status='complete',receipt_json=?,updated_at=? WHERE archive_key=? AND status='prepared'`, string(encoded), receipt.ClosedAt, key)
		return err
	})
	return receipt, err
}

func validateArchiveStructure(workspace, change, workflow string) (openspec.Result, error) {
	phase := "tasks"
	if workflow == "sdd-lite" {
		phase = "integrated"
	}
	result, err := openspec.Validate(workspace, change, openspec.Options{Workflow: workflow, Phase: phase})
	if err != nil {
		return result, err
	}
	if !result.Valid {
		return result, fmt.Errorf("archive structural validation failed: %s: %s", result.Errors[0].Code, result.Errors[0].Message)
	}
	return result, nil
}

func archiveExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("archive path must be a real directory")
	}
	return true, nil
}

// The manifest is path-sensitive and byte-exact, including empty directories.
func archiveManifest(root string) (string, error) {
	if err := archiveRealPath(root); err != nil {
		return "", err
	}
	type entry struct {
		Path   string
		Kind   string
		SHA256 string
	}
	entries := []entry{}
	var total int64
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if len(entries) >= 1000 {
			return fmt.Errorf("archive exceeds 1000 entries")
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive tree contains symbolic link")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		record := entry{Path: filepath.ToSlash(rel), Kind: "directory"}
		if !item.IsDir() {
			info, err := item.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("archive contains nonregular file")
			}
			remaining := int64(16*1024*1024) - total
			if info.Size() > remaining {
				return fmt.Errorf("archive exceeds 16 MiB")
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(file, remaining+1))
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			total += int64(len(data))
			if total > 16*1024*1024 {
				return fmt.Errorf("archive exceeds 16 MiB")
			}
			hash := sha256.Sum256(data)
			record.Kind = "file"
			record.SHA256 = hex.EncodeToString(hash[:])
		}
		entries = append(entries, record)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return strings.Compare(entries[i].Path, entries[j].Path) < 0 })
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func archiveRealPath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive tree may not traverse symbolic links")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}
