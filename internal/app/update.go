package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

func runUpdate(args []string) error {
	checkOnly := false
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--check", "-c":
			checkOnly = true
		case "--help", "-h":
			fmt.Println("Usage: cortex-ia update [--check]")
			fmt.Println("Check for and apply updates from GitHub Releases.")
			fmt.Println("\nOptions:")
			fmt.Println("  --check, -c    Check if an update is available without downloading or applying it")
			return nil
		default:
			return fmt.Errorf("unknown flag for update: %s (use 'cortex-ia update --help' for usage)", arg)
		}
	}

	client := updater.New("")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("Checking for cortex-ia updates (current: %s)...\n", Version)
	rel, hasUpdate, err := client.CheckLatest(ctx, Version)
	if err != nil {
		if errors.Is(err, updater.ErrNoTrustedKey) {
			return err
		}
		return fmt.Errorf("update check failed: %w", err)
	}

	if !hasUpdate {
		fmt.Printf("cortex-ia is already up to date (%s).\n", Version)
		return nil
	}

	fmt.Printf("Found newer release: %s (published %s)\n", rel.TagName, rel.PublishedAt.Format("2006-01-02"))
	if checkOnly {
		fmt.Println("Run 'cortex-ia update' to install the newest version.")
		return nil
	}

	fmt.Printf("Downloading and applying %s...\n", rel.TagName)
	if err := client.ApplyUpdate(ctx, Version, rel); err != nil {
		if errors.Is(err, updater.ErrNoTrustedKey) {
			return err
		}
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Successfully updated cortex-ia to %s!\n", rel.TagName)
	return nil
}
