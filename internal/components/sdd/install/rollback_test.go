package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackRestoresManagedBytesAndMetadataWhilePreservingUnmanagedEdits(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(t.TempDir(), "backups")
	assetPath := "rules/workflow.md"
	metadataPath := "install/compatibility.json"
	prior := []byte("managed: prior\nuser: before\n")
	installed := []byte("managed: installed\nuser: before\n")
	writeTarget(t, root, assetPath, prior, 0o640)
	writeTarget(t, root, metadataPath, []byte("prior metadata\n"), 0o600)
	priorInfo, err := os.Stat(filepath.Join(root, filepath.FromSlash(assetPath)))
	if err != nil {
		t.Fatal(err)
	}
	priorMode := priorInfo.Mode().Perm()

	receipt, err := NewApplier(root, backupRoot).Apply(Plan{
		Updates: []Effect{{Path: assetPath, SemanticID: "asset/rule/workflow", Content: installed, AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{assetPath, metadataPath}},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	writeTarget(t, root, assetPath, []byte("managed: installed\nuser: after\n"), 0o600)
	writeTarget(t, root, metadataPath, []byte("new metadata\n"), 0o600)

	doctorCalls := 0
	result, err := Rollback(receipt, func() error {
		doctorCalls++
		asset, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(assetPath)))
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(asset, []byte("managed: prior\nuser: after\n")) {
			return errors.New("doctor observed an unhealthy restored asset")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if !result.DoctorPassed || doctorCalls != 1 || len(result.Conflicts) != 0 {
		t.Fatalf("rollback result = %+v, doctor calls = %d", result, doctorCalls)
	}
	assertTarget(t, root, assetPath, "managed: prior\nuser: after\n")
	assertTarget(t, root, metadataPath, "prior metadata\n")
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(assetPath)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != priorMode {
		t.Fatalf("restored mode = %o, want prior mode %o", info.Mode().Perm(), priorMode)
	}
}

func TestRollbackReportsConflictWithoutDiscardingCurrentOrMetadata(t *testing.T) {
	root := t.TempDir()
	assetPath := "agents/implement.md"
	metadataPath := "agents/implement.md.cortex-ia.json"
	writeTarget(t, root, assetPath, []byte("managed: prior\n"), 0o600)
	writeTarget(t, root, metadataPath, []byte("prior metadata\n"), 0o600)

	receipt, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(Plan{
		Updates: []Effect{{Path: assetPath, SemanticID: "asset/agent/implement", Content: []byte("managed: installed\n"), AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{assetPath, metadataPath}},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	writeTarget(t, root, assetPath, []byte("managed: user\n"), 0o600)
	writeTarget(t, root, metadataPath, []byte("current metadata\n"), 0o600)

	doctorCalled := false
	result, err := Rollback(receipt, func() error { doctorCalled = true; return nil })
	if !errors.Is(err, ErrRollbackConflict) {
		t.Fatalf("Rollback() error = %v, want ErrRollbackConflict", err)
	}
	if doctorCalled {
		t.Fatal("doctor must not run for a rollback blocked before mutation")
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want one", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.Path != assetPath || conflict.SemanticID != "asset/agent/implement" ||
		string(conflict.Prior) != "managed: prior\n" ||
		string(conflict.Installed) != "managed: installed\n" ||
		string(conflict.Current) != "managed: user\n" ||
		conflict.PriorRef == "" || conflict.InstalledRef == "" || conflict.CurrentRef == "" {
		t.Fatalf("conflict did not retain all versions and references: %+v", conflict)
	}
	assertTarget(t, root, assetPath, "managed: user\n")
	assertTarget(t, root, metadataPath, "current metadata\n")
}

func TestRollbackRequiresRestoredBundleToPassDoctor(t *testing.T) {
	root := t.TempDir()
	assetPath := "agents/validate.md"
	writeTarget(t, root, assetPath, []byte("prior\n"), 0o600)
	receipt, err := NewApplier(root, filepath.Join(t.TempDir(), "backups")).Apply(Plan{
		Updates: []Effect{{Path: assetPath, SemanticID: "asset/agent/validate", Content: []byte("installed\n"), AfterMode: 0o600}},
		Backup:  BackupScope{Required: true, Paths: []string{assetPath}},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	doctorErr := errors.New("doctor rejected restored bundle")
	result, err := Rollback(receipt, func() error { return doctorErr })
	if !errors.Is(err, doctorErr) || !errors.Is(err, ErrRollbackDoctor) {
		t.Fatalf("Rollback() error = %v, want doctor failure", err)
	}
	if result.DoctorPassed {
		t.Fatalf("rollback result = %+v, want failed doctor", result)
	}
}
