package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/uninstall"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// runUninstall is the CLI entrypoint for `cortex-ia uninstall`.
//
// Flags:
//
//	--agent <id>       repeat to scope to specific agents
//	--component <id>   repeat to scope to specific components
//	--all              wipe every managed change and clear state.json
//	--dry-run          show planned ops without writing
//	--yes              skip the destructive-action confirmation prompt
//	--no-backup        skip the pre-uninstall snapshot (not recommended)
func runUninstall(args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	sel := uninstall.Selection{}
	skipBackup := false
	autoConfirm := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			i++
			sel.Agents = append(sel.Agents, model.AgentID(args[i]))
		case "--component":
			if i+1 >= len(args) {
				return fmt.Errorf("--component requires a value")
			}
			i++
			sel.Components = append(sel.Components, model.ComponentID(args[i]))
		case "--all":
			sel.All = true
		case "--dry-run":
			sel.DryRun = true
		case "--yes", "-y":
			autoConfirm = true
		case "--no-backup":
			skipBackup = true
		default:
			return fmt.Errorf("uninstall: unknown flag %q", args[i])
		}
	}

	svc := uninstall.NewService(homeDir)

	plan, err := svc.PathsToBackup(sel)
	if err != nil {
		return err
	}
	if len(plan) == 0 && !sel.All {
		fmt.Println("Nothing to uninstall — no managed files match the selection.")
		return nil
	}

	if sel.DryRun {
		fmt.Printf("Would touch %d file(s):\n", len(plan))
		for _, p := range plan {
			fmt.Printf("  %s\n", p)
		}
		return nil
	}

	if !autoConfirm {
		fmt.Println("About to uninstall:")
		fmt.Printf("  Agents:     %v\n", agentsLabel(sel))
		fmt.Printf("  Components: %v\n", componentsLabel(sel))
		fmt.Printf("  Files:      %d\n", len(plan))
		fmt.Print("Proceed? [y/N] ")
		var answer string
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Pre-uninstall snapshot for rollback.
	var snapshotID string
	if !skipBackup && len(plan) > 0 {
		snapshotID, err = createUninstallSnapshot(homeDir, plan)
		if err != nil {
			return fmt.Errorf("create uninstall snapshot: %w", err)
		}
		fmt.Printf("Snapshot created: %s\n", snapshotID)
	}

	res, err := svc.Apply(sel)
	if err != nil {
		fmt.Printf("Uninstall failed; rollback with `cortex-ia rollback --backup %s`.\n", snapshotID)
		return err
	}

	res.BackupID = snapshotID
	printUninstallReport(res)
	return nil
}

func createUninstallSnapshot(homeDir string, paths []string) (string, error) {
	id := time.Now().Format("20060102-150405-uninstall")
	root := filepath.Join(homeDir, ".cortex-ia", "backups", id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	snap, err := backup.NewSnapshotter().Create(root, paths)
	if err != nil {
		return "", err
	}

	// Stamp the manifest so rollback can identify uninstall snapshots.
	snap.Source = backup.BackupSourceUninstall
	snap.CreatedByVersion = Version
	if err := backup.WriteManifest(filepath.Join(root, backup.ManifestFilename), snap); err != nil {
		return "", err
	}
	return snap.ID, nil
}

func agentsLabel(sel uninstall.Selection) string {
	if len(sel.Agents) == 0 {
		return "all installed"
	}
	out := ""
	for i, id := range sel.Agents {
		if i > 0 {
			out += ", "
		}
		out += string(id)
	}
	return out
}

func componentsLabel(sel uninstall.Selection) string {
	if len(sel.Components) == 0 {
		return "all managed"
	}
	out := ""
	for i, id := range sel.Components {
		if i > 0 {
			out += ", "
		}
		out += string(id)
	}
	return out
}

func printUninstallReport(res uninstall.Result) {
	fmt.Println()
	fmt.Println("Uninstall complete.")
	if res.BackupID != "" {
		fmt.Printf("  Snapshot:    %s\n", res.BackupID)
	}
	if len(res.RemovedFiles) > 0 {
		fmt.Printf("  Removed:     %d file(s)\n", len(res.RemovedFiles))
		for _, p := range res.RemovedFiles {
			fmt.Printf("    - %s\n", p)
		}
	}
	if len(res.RemovedDirs) > 0 {
		fmt.Printf("  Cleaned:     %d directory(ies)\n", len(res.RemovedDirs))
	}
	if len(res.ChangedFiles) > 0 {
		fmt.Printf("  Edited:      %d file(s) (markers stripped)\n", len(res.ChangedFiles))
	}
	if len(res.SkippedNonEmpty) > 0 {
		fmt.Printf("  Skipped:     %d directory(ies) with user content (left alone)\n", len(res.SkippedNonEmpty))
	}
	if len(res.AgentsRemoved) > 0 {
		fmt.Printf("  Agents:      %v removed from state\n", res.AgentsRemoved)
	}
}
