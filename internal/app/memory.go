package app

import (
	"fmt"
	"os"

	"github.com/lleontor705/cortex-ia/internal/components/cortex"
)

func runMemory(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia memory sync [--export|--import]")
	}

	switch args[0] {
	case "sync":
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		if len(args) > 1 && args[1] == "--import" {
			manifest, err := cortex.ImportProjectMemory(cwd)
			if err != nil {
				return fmt.Errorf("import memory failed: %w", err)
			}
			fmt.Printf("Successfully imported project memory for %q (exported at %s)\n", manifest.Project, manifest.ExportedAt.Format("2006-01-02 15:04:05"))
			return nil
		}

		err = cortex.ExportProjectMemory(cwd, "current-project")
		if err != nil {
			return fmt.Errorf("export memory failed: %w", err)
		}
		fmt.Printf("Successfully exported project memory to .cortex/ in %s\n", cwd)
		return nil

	default:
		return fmt.Errorf("unknown memory command: %s (usage: cortex-ia memory sync [--export|--import])", args[0])
	}
}
