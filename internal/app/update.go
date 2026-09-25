package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/updater"
)

const updateCheckTimeout = 60 * time.Second

// updateClientFactory is the transport seam shared by every update surface.
// Tests replace it with an in-memory transport so no check reaches the network.
var updateClientFactory = func() *updater.Client { return updater.New("") }

func runUpdate(args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "schedule") {
		return runUpdateSchedule(args[1:])
	}

	checkOnly := false
	scheduled := false
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--check", "-c":
			checkOnly = true
		case "--scheduled":
			scheduled = true
		case "--help", "-h":
			printUpdateHelp()
			return nil
		default:
			return fmt.Errorf("unknown flag for update: %s (use 'cortex-ia update --help' for usage)", arg)
		}
	}

	if !scheduled {
		return runManualUpdate(checkOnly)
	}
	if !checkOnly {
		return errors.New("--scheduled is only valid with --check: the scheduled check is headless and check-only")
	}
	return runScheduledUpdateCheck()
}

func printUpdateHelp() {
	fmt.Println("Usage: cortex-ia update [--check] [--scheduled]")
	fmt.Println("       cortex-ia update schedule <enable|disable|status>")
	fmt.Println("Check for and apply updates from GitHub Releases.")
	fmt.Println("\nOptions:")
	fmt.Println("  --check, -c    Check if an update is available without downloading or applying it")
	fmt.Println("  --scheduled    Headless check-only mode for the OS scheduler; valid only with --check.")
	fmt.Println("                 Records the result in the shared update state and never downloads or applies")
	fmt.Println("\nSubcommands:")
	fmt.Println("  schedule enable    Register the daily check-only update task for the current user")
	fmt.Println("  schedule disable   Remove the managed update task")
	fmt.Println("  schedule status    Report whether the managed update task is registered")
}

// printDevelopmentBuildNotice reports and returns true when the running build
// carries no stable release identity and therefore cannot self-update. Source
// builds report "dev" and repository checkouts report a git-describe string;
// both degrade to this notice instead of a raw version-parser failure.
func printDevelopmentBuildNotice() bool {
	if updater.ClassifyBuild(Version) == updater.ReleaseBuild {
		return false
	}
	fmt.Println(updater.SelfUpdateDisabledNotice(Version))
	return true
}

// newUpdateClient binds the client to the machine-local state root so the
// apply path resolves the persisted floor and raises it after a verified
// replacement. No state is read or written here.
func newUpdateClient(home string) *updater.Client {
	client := updateClientFactory()
	client.StateHome = home
	if state, err := updater.LoadUpdateState(home); err == nil {
		client.AppliedFloor = state.AppliedFloor
	}
	return client
}

// resolveUpdateHome resolves the state root shared with the TUI boot prompt. An
// empty result keeps the check surface read-only instead of writing beside the
// running binary.
func resolveUpdateHome() string {
	home, err := updater.DefaultStateHome()
	if err != nil {
		return ""
	}
	if stateHome, err := filepath.Abs(home); err == nil {
		return filepath.Clean(stateHome)
	}
	return home
}

func runManualUpdate(checkOnly bool) error {
	// Fail closed on trust before any cache short-circuit so a fresh state can
	// never bypass the authenticated-update gate.
	if err := updater.RequireTrust(); err != nil {
		return err
	}

	if printDevelopmentBuildNotice() {
		return nil
	}

	home := resolveUpdateHome()
	state, _ := updater.LoadUpdateState(home)
	client := newUpdateClient(home)

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	if !checkOnly && state.CheckedWithin(time.Now().UTC(), updater.UpdateCheckTTL) && !state.UpdateAvailable() {
		// A fresh state with nothing cached means the previous answer still
		// holds. A pending cached release does not short-circuit: installing it
		// needs the full asset list, which only a network response carries.
		fmt.Printf("cortex-ia is already up to date (%s).\n", Version)
		return nil
	}

	fmt.Printf("Checking for cortex-ia updates (current: %s)...\n", Version)
	check := client.CheckLatest
	if !checkOnly {
		check = client.CheckLatestFresh
	}
	rel, hasUpdate, err := check(ctx, Version)
	if err != nil {
		if errors.Is(err, updater.ErrNoTrustedKey) {
			return err
		}
		return fmt.Errorf("update check failed: %w", err)
	}

	if !hasUpdate {
		fmt.Printf("cortex-ia is already up to date (%s).\n", Version)
		if checkOnly {
			return persistAndWarn(home, rel, false)
		}
		return recordCompletedCheck(home, rel, false)
	}

	fmt.Printf("Found newer release: %s (published %s)\n", rel.TagName, rel.PublishedAt.Format("2006-01-02"))
	if checkOnly {
		if err := persistAndWarn(home, rel, true); err != nil {
			return err
		}
		fmt.Println("Run 'cortex-ia update' to install the newest version.")
		return nil
	}

	if err := recordCompletedCheck(home, rel, true); err != nil {
		return err
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

// runScheduledUpdateCheck is the headless contract registered with the OS
// scheduler: it refreshes the cached update state and exits. It never
// downloads, prompts, or replaces a binary, and a failed check leaves the
// previous state untouched.
func runScheduledUpdateCheck() error {
	if err := updater.RequireTrust(); err != nil {
		return err
	}

	if printDevelopmentBuildNotice() {
		return nil
	}

	home := resolveUpdateHome()
	state, _ := updater.LoadUpdateState(home)
	if state.CheckedWithin(time.Now().UTC(), updater.UpdateCheckTTL) {
		fmt.Printf("No update available (current: %s; checked %s)\n", Version, state.LastCheckedAt.UTC().Format(time.RFC3339))
		return nil
	}

	client := newUpdateClient(home)

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	rel, hasUpdate, err := client.CheckLatest(ctx, Version)
	if err != nil {
		// Preserve the typed fail-closed trust and symmetry failures verbatim;
		// they are the operator-facing contract of the check engine.
		if errors.Is(err, updater.ErrNoTrustedKey) || errors.Is(err, updater.ErrNonCanonicalVersion) {
			return err
		}
		return fmt.Errorf("scheduled update check failed: %w", err)
	}

	if err := persistAndWarn(home, rel, hasUpdate); err != nil {
		return err
	}

	if hasUpdate {
		fmt.Printf("Update available: %s (run 'cortex-ia update' to install)\n", rel.TagName)
		return nil
	}
	fmt.Printf("No update available (current: %s)\n", Version)
	return nil
}

// persistAndWarn records a completed check and surfaces the ambiguous-install
// warning. The signed manifest digest is deliberately left empty: observing it
// requires downloading the manifest, which check-only surfaces must never do.
func persistAndWarn(home string, rel *updater.Release, hasUpdate bool) error {
	state, err := persistCheckResult(home, rel, hasUpdate)
	if err != nil {
		return err
	}
	printDualInstallWarning(state.InstallCandidates)
	return nil
}

func persistCheckResult(home string, rel *updater.Release, hasUpdate bool) (updater.UpdateState, error) {
	if strings.TrimSpace(home) == "" {
		return updater.UpdateState{}, errors.New("cannot resolve the cortex-ia update state home")
	}

	state, _ := updater.LoadUpdateState(home)
	state.LastCheckedAt = time.Now().UTC()
	state.Available = ""
	state.AvailableDigest = ""
	if rel != nil {
		if etag := strings.TrimSpace(rel.ETag); etag != "" {
			state.ReleaseETag = etag
		}
	}
	if execPath := managedExecutablePath(); execPath != "" {
		state.ManagedPath = execPath
	}
	state.InstallCandidates = updater.DetectInstallCandidates(state.ManagedPath)
	if hasUpdate && rel != nil {
		state.Available = rel.TagName
	}

	if err := updater.SaveUpdateStateAtomic(home, state); err != nil {
		return updater.UpdateState{}, err
	}
	return state, nil
}

// recordCompletedCheck persists an automatic apply-path check without the
// install-candidate warning. An unresolvable state home keeps the check
// read-only instead of failing the update.
func recordCompletedCheck(home string, rel *updater.Release, hasUpdate bool) error {
	if strings.TrimSpace(home) == "" {
		return nil
	}
	_, err := persistCheckResult(home, rel, hasUpdate)
	return err
}

// managedExecutablePath resolves the running binary through symlinks so the
// scheduler registration and the recorded managed path agree.
func managedExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		return resolved
	}
	return execPath
}

func printDualInstallWarning(candidates []string) {
	warning := updater.DualInstallWarning(candidates)
	if warning == "" {
		return
	}
	fmt.Printf("Warning: %s\n", warning)
}
