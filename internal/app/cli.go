package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/mcpmanager"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// newService builds the install.Service for the current user's home. The
// service owns every install, sync, doctor, rollback, uninstall, and MCP
// decision; the CLI only parses intent and renders receipts.
func newService() (*install.Service, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	service, err := install.New(homeDir)
	if err != nil {
		return nil, err
	}
	return service, nil
}

// runFlags is the parsed flag set shared by the mutating commands.
type runFlags struct {
	DryRun    bool
	Overwrite bool
}

// parseRunFlags accepts exactly the flags allowed on install, sync, and
// uninstall. Any other argument — including retired flags that slipped past
// preflight — is rejected with the valid surface named.
func parseRunFlags(args []string, command string, allowOverwrite bool) (runFlags, error) {
	var flags runFlags
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--dry-run":
			flags.DryRun = true
		case "--overwrite":
			if !allowOverwrite {
				return flags, fmt.Errorf("unknown flag: %s (cortex-ia %s supports only --dry-run)", arg, command)
			}
			flags.Overwrite = true
		default:
			if strings.HasPrefix(arg, "-") {
				return flags, fmt.Errorf("unknown flag: %s (cortex-ia %s supports only --dry-run and --overwrite)", arg, command)
			}
			return flags, fmt.Errorf("unexpected argument: %s (cortex-ia %s takes no positional arguments)", arg, command)
		}
	}
	return flags, nil
}

// stdinIsInteractive reports whether standard input is an interactive
// terminal (a character device) rather than a pipe, a redirected file, or a
// closed stream.
func stdinIsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// confirmDestructive requires an interactive terminal and an explicit
// affirmative answer before a destructive operation may proceed. Piped or
// redirected input, EOF, and anything but an explicit yes fail closed: the
// caller returns the error and nothing is written.
func confirmDestructive(action string) error {
	if !stdinIsInteractive() {
		return fmt.Errorf("%s requires an interactive terminal for explicit confirmation; refusing to proceed (nothing was written)", action)
	}
	fmt.Printf("%s is destructive.\n", action)
	fmt.Print("Proceed? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("%s: no confirmation was read; refusing to proceed (nothing was written)", action)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("%s: confirmation declined; refusing to proceed (nothing was written)", action)
	}
	return nil
}

// runInstall installs the embedded OpenCode asset set and the default
// managed MCP selection. The real run applies exactly the previewed plan:
// the preview's plan digest travels as ExpectedPlanDigest, the service
// re-plans with identical options, and any drift aborts with a typed
// stale-plan error before a single write. Conflicting unmanaged files fail
// closed unless --overwrite is given and confirmed on an interactive
// terminal.
func runInstall(args []string) error {
	flags, err := parseRunFlags(args, "install", true)
	if err != nil {
		return err
	}
	service, err := newService()
	if err != nil {
		return err
	}

	opts := install.DefaultOptions()
	opts.Version = Version
	return previewAndApply("install", flags, opts, service.Install)
}

// runSync reconciles an installed home with the current embedded asset set,
// removing stale owned artifacts. Conflicts fail closed exactly like
// install, and the real run is bound to the previewed plan digest exactly
// like install.
func runSync(args []string) error {
	flags, err := parseRunFlags(args, "sync", true)
	if err != nil {
		return err
	}
	service, err := newService()
	if err != nil {
		return err
	}

	opts := install.DefaultOptions()
	opts.Version = Version
	return previewAndApply("sync", flags, opts, service.Sync)
}

// previewAndApply runs the shared install/sync flow. The preview uses the
// final effective options — including --overwrite — so the digest, the
// conflict list, and every overwrite effect are exactly what the real call
// re-plans and compares. Unauthorized conflicts fail closed; authorized
// overwrites are listed with the plan digest and confirmed interactively
// before the digest-bound apply. Stale plans and home-lock contention are
// rendered as typed outcomes; nothing is written on any refusal, and the
// prompts run entirely before the service acquires the home lock.
func previewAndApply(command string, flags runFlags, opts install.Options, apply func(install.Options) (*install.InstallReceipt, error)) error {
	previewOpts := opts
	previewOpts.DryRun = true
	previewOpts.Overwrite = flags.Overwrite
	preview, err := apply(previewOpts)
	if err != nil {
		return err
	}
	if flags.DryRun {
		printInstallReceipt(command+" (dry-run)", preview)
		return nil
	}
	if len(preview.Conflicts) > 0 {
		printConflicts(preview.Conflicts)
		if !flags.Overwrite {
			return fmt.Errorf("%s: unmanaged conflicting files present; nothing was written (re-run with --overwrite to replace them after confirmation)", command)
		}
	}
	if overwrites := overwriteTargets(preview); len(overwrites) > 0 {
		printOverwrites(overwrites)
		fmt.Printf("  Plan digest: %s\n", preview.PlanDigest)
		if err := confirmDestructive(fmt.Sprintf("%s --overwrite (replace the %d unmanaged file(s) listed above)", command, len(overwrites))); err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
	}

	applyOpts := opts
	applyOpts.Overwrite = flags.Overwrite
	applyOpts.ExpectedPlanDigest = preview.PlanDigest
	receipt, err := apply(applyOpts)
	if err != nil {
		var drift *pipeline.PlanDriftError
		switch {
		case errors.As(err, &drift):
			return fmt.Errorf("%s: the confirmed plan is stale; nothing was written (preview again and re-confirm): %w", command, err)
		case errors.Is(err, install.ErrHomeBusy):
			return fmt.Errorf("%s: another process holds this home's lock; nothing was written: %w", command, err)
		}
		printInstallReceipt(command, receipt)
		return err
	}
	printInstallReceipt(command, receipt)
	return nil
}

// overwriteTargets lists every destination the plan replaces through an
// explicitly authorized overwrite, in plan order.
func overwriteTargets(receipt *install.InstallReceipt) []string {
	if receipt == nil || receipt.Plan == nil {
		return nil
	}
	var targets []string
	for _, effect := range receipt.Plan.Effects {
		if effect.Kind == pipeline.EffectOverwrite {
			targets = append(targets, effect.Dest)
		}
	}
	return targets
}

// printOverwrites shows the exact files an authorized overwrite will
// replace; the same list is shown in the preview receipt.
func printOverwrites(targets []string) {
	fmt.Printf("Files the authorized overwrite will replace (%d):\n", len(targets))
	for _, target := range targets {
		fmt.Printf("  %s\n", target)
	}
}

// printConflicts lists the fail-closed blockers read-only, as reported by the
// engine plan.
func printConflicts(conflicts []pipeline.Conflict) {
	fmt.Println("Conflicting unmanaged files (preview is read-only):")
	for _, conflict := range conflicts {
		fmt.Printf("  %s: %s (%s)\n", conflict.Target, conflict.Kind, conflict.Reason)
	}
}

func printInstallReceipt(title string, receipt *install.InstallReceipt) {
	if receipt == nil {
		return
	}
	fmt.Printf("%s — plan %s\n", title, receipt.PlanDigest)
	if receipt.DryRun {
		fmt.Println("  Dry-run: nothing was written.")
	}
	if receipt.Converged {
		fmt.Println("  Already converged: zero writes needed.")
	}
	fmt.Printf("  Configured MCPs: %s\n", strings.Join(defaultStringSlice(receipt.Configured), ", "))
	if len(receipt.Qualified) > 0 {
		fmt.Printf("  Qualified MCPs: %s\n", strings.Join(receipt.Qualified, ", "))
	}
	fmt.Printf("  Changed: %d\n", len(receipt.Changed))
	for _, change := range receipt.Changed {
		fmt.Printf("    %s\n", change)
	}
	if overwrites := overwriteTargets(receipt); len(overwrites) > 0 {
		fmt.Printf("  Overwrites: %d\n", len(overwrites))
		for _, target := range overwrites {
			fmt.Printf("    %s\n", target)
		}
	}
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s (verified: %v)\n", receipt.BackupID, receipt.BackupVerified)
	}
	if receipt.TransactionID != "" {
		fmt.Printf("  Transaction: %s\n", receipt.TransactionID)
	}
	if receipt.Restored {
		fmt.Printf("  Failed apply restored from backup (error: %s)\n", receipt.RestoreError)
	}
	for _, warning := range receipt.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
}

// runMCP dispatches the managed MCP subcommands. Add accepts catalog
// presets and custom local/remote servers through the typed desired
// contract; every ownership decision belongs to the service and its
// manager.
func runMCP(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cortex-ia mcp <add|list|remove> [name] [options] (see 'cortex-ia help')")
	}

	switch strings.ToLower(args[0]) {
	case "add":
		spec, err := parseMCPAdd(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runMCPAdd(service, spec)
	case "remove":
		name, dryRun, err := parseMCPRemove(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runMCPRemove(service, name, dryRun)
	case "list":
		asJSON, err := parseMCPList(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runMCPList(service, asJSON)
	default:
		return fmt.Errorf("unknown mcp action: %s (use: add, list, remove)", args[0])
	}
}

// mcpAddSpec is the parsed `mcp add` command line. Exactly one kind flag
// (--preset, --local, --remote) must be present; env assignments bind to
// local servers and header assignments to remote ones.
type mcpAddSpec struct {
	name    string
	preset  bool
	local   bool
	remote  bool
	url     string
	env     []string
	headers []string
	command []string
	dryRun  bool
}

// parseMCPAdd parses `mcp add <name> [flags] [-- command...]`. Everything
// after the "--" separator is captured verbatim as the local command vector;
// no flag parsing happens inside it.
func parseMCPAdd(args []string) (mcpAddSpec, error) {
	var spec mcpAddSpec
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return spec, fmt.Errorf("usage: cortex-ia mcp add <name> (--preset | --local [--env KEY=VALUE]... -- <command...> | --remote <url> [--header KEY=VALUE]...) [--dry-run]")
	}
	spec.name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch strings.ToLower(arg) {
		case "--dry-run":
			spec.dryRun = true
		case "--preset":
			spec.preset = true
		case "--local":
			spec.local = true
		case "--remote":
			if spec.remote {
				return spec, fmt.Errorf("flag --remote may be given only once")
			}
			spec.remote = true
			if i+1 >= len(rest) {
				return spec, fmt.Errorf("flag --remote requires a URL argument")
			}
			spec.url = rest[i+1]
			i++
		case "--env":
			if i+1 >= len(rest) {
				return spec, fmt.Errorf("flag --env requires a KEY=VALUE argument")
			}
			spec.env = append(spec.env, rest[i+1])
			i++
		case "--header":
			if i+1 >= len(rest) {
				return spec, fmt.Errorf("flag --header requires a KEY=VALUE argument")
			}
			spec.headers = append(spec.headers, rest[i+1])
			i++
		case "--":
			spec.command = append(spec.command, rest[i+1:]...)
			return spec, nil
		default:
			return spec, fmt.Errorf("unknown argument: %s (cortex-ia mcp add supports --preset, --local, --remote <url>, --env KEY=VALUE, --header KEY=VALUE, --dry-run, and '--' before the command vector)", arg)
		}
	}
	return spec, nil
}

// desired converts the parsed spec into the typed service contract. Exactly
// one kind flag is enforced here; the remaining field-level rules are the
// service's own typed validation, which never echoes assignment values.
func (s mcpAddSpec) desired() (mcpmanager.Desired, error) {
	kinds := 0
	for _, on := range []bool{s.preset, s.local, s.remote} {
		if on {
			kinds++
		}
	}
	if kinds != 1 {
		return mcpmanager.Desired{}, fmt.Errorf("mcp add %q: exactly one of --preset, --local, or --remote is required (they are mutually exclusive)", s.name)
	}
	desired := mcpmanager.Desired{Name: s.name, Env: s.env, Headers: s.headers}
	switch {
	case s.preset:
		desired.Kind = mcpmanager.DesiredPreset
		desired.Preset = s.name
	case s.local:
		desired.Kind = mcpmanager.DesiredLocal
		desired.Command = s.command
	case s.remote:
		desired.Kind = mcpmanager.DesiredRemote
		desired.URL = s.url
	}
	if err := desired.Validate(); err != nil {
		return desired, fmt.Errorf("mcp add %q: %w", s.name, err)
	}
	return desired, nil
}

// runMCPAdd registers the desired MCP server through the service. Malformed
// requests fail closed before any state access; ownership conflicts fail
// closed through the typed manager errors, which never contain secrets.
func runMCPAdd(service *install.Service, spec mcpAddSpec) error {
	desired, err := spec.desired()
	if err != nil {
		return err
	}

	receipt, err := service.MCPAddDesired(desired, install.MCPOptions{DryRun: spec.dryRun})
	if err != nil {
		var conflict *mcpmanager.ConflictError
		if errors.As(err, &conflict) {
			// The typed conflict message is the whole diagnosis: it names
			// the server and digests only, never configuration secrets.
			return fmt.Errorf("mcp add %q failed closed (nothing was written): %w", spec.name, conflict)
		}
		return err
	}

	label := fmt.Sprintf("mcp add %s", receipt.Name)
	if receipt.DryRun {
		label += " (dry-run)"
	}
	fmt.Printf("%s — config %s\n", label, receipt.ConfigPath)
	fmt.Printf("  Action: %s\n", defaultString(receipt.Action, "none"))
	fmt.Printf("  Configured: %v  Qualified: %v  Installed: %v\n", receipt.Configured, receipt.Qualified, receipt.Installed)
	fmt.Printf("  Config changed: %v\n", receipt.Changed)
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s\n", receipt.BackupID)
	}
	for _, warning := range receipt.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	return nil
}

// parseMCPRemove parses `mcp remove <name> [--dry-run]`.
func parseMCPRemove(args []string) (string, bool, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", false, fmt.Errorf("usage: cortex-ia mcp remove <name> [--dry-run] (real removal is destructive and asks for interactive confirmation)")
	}
	name := args[0]
	dryRun := false
	for _, arg := range args[1:] {
		if strings.ToLower(arg) == "--dry-run" {
			dryRun = true
			continue
		}
		return name, false, fmt.Errorf("unknown flag: %s (cortex-ia mcp remove supports only --dry-run)", arg)
	}
	return name, dryRun, nil
}

// runMCPRemove deregisters a managed MCP entry. The real run is destructive:
// it requires an interactive terminal and an explicit confirmation, and
// fails closed on piped or closed input before the service is called.
func runMCPRemove(service *install.Service, name string, dryRun bool) error {
	if !dryRun {
		if err := confirmDestructive(fmt.Sprintf("mcp remove %s (delete the managed MCP entry from the OpenCode config)", name)); err != nil {
			return fmt.Errorf("mcp remove: %w", err)
		}
	}

	receipt, err := service.MCPRemove(name, install.MCPOptions{DryRun: dryRun})
	if err != nil {
		var conflict *mcpmanager.ConflictError
		if errors.As(err, &conflict) {
			return fmt.Errorf("mcp remove %q failed closed (nothing was written): %w", name, conflict)
		}
		return err
	}

	label := fmt.Sprintf("mcp remove %s", receipt.Name)
	if receipt.DryRun {
		label += " (dry-run)"
	}
	fmt.Printf("%s — config %s\n", label, receipt.ConfigPath)
	fmt.Printf("  Action: %s\n", defaultString(receipt.Action, "none"))
	fmt.Printf("  Config changed: %v\n", receipt.Changed)
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s\n", receipt.BackupID)
	}
	for _, warning := range receipt.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	return nil
}

// parseMCPList parses `mcp list [--json]`.
func parseMCPList(args []string) (bool, error) {
	asJSON := false
	for _, arg := range args {
		if strings.ToLower(arg) == "--json" {
			asJSON = true
			continue
		}
		return false, fmt.Errorf("unknown argument: %s (cortex-ia mcp list takes no arguments besides --json)", arg)
	}
	return asJSON, nil
}

// mcpListEntryJSON is the sanitized, allow-listed JSON projection of one
// managed MCP entry: identity evidence only (name, status, digest, entry
// type, and env/header variable NAMES). Values, commands, and URLs are never
// representable here.
type mcpListEntryJSON struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Digest      string   `json:"digest,omitempty"`
	Type        string   `json:"type,omitempty"`
	EnvNames    []string `json:"env_names,omitempty"`
	HeaderNames []string `json:"header_names,omitempty"`
}

// mcpListJSON is the sanitized JSON projection of the whole listing.
type mcpListJSON struct {
	Installed  bool               `json:"installed"`
	ConfigPath string             `json:"config_path"`
	Entries    []mcpListEntryJSON `json:"entries"`
	Unknown    []string           `json:"unknown,omitempty"`
}

// runMCPList prints the ownership status of every managed preset plus the
// unknown entries found in the configuration. Only names, statuses, digests,
// entry types, and env/header variable names are shown — never entry
// contents, commands, URLs, or secret values. With --json a valid,
// sanitized JSON document is printed and nothing else.
func runMCPList(service *install.Service, asJSON bool) error {
	report, err := service.MCPList()
	if err != nil {
		return err
	}
	if asJSON {
		view := mcpListJSON{Installed: report.Installed, ConfigPath: report.ConfigPath, Unknown: report.Unknown, Entries: []mcpListEntryJSON{}}
		for _, entry := range report.Entries {
			view.Entries = append(view.Entries, mcpListEntryJSON{
				Name:        entry.Name,
				Status:      string(entry.Status),
				Digest:      entry.Digest,
				Type:        entry.Type,
				EnvNames:    entry.EnvNames,
				HeaderNames: entry.HeaderNames,
			})
		}
		encoded, err := json.MarshalIndent(view, "", "  ")
		if err != nil {
			return fmt.Errorf("encode sanitized MCP listing: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Printf("MCP configuration: %s\n", report.ConfigPath)
	fmt.Printf("  Install-accredited: %v\n", report.Installed)
	fmt.Println("  Managed presets:")
	for _, entry := range report.Entries {
		fmt.Printf("    %-24s %s\n", entry.Name, entry.Status)
	}
	if len(report.Unknown) > 0 {
		fmt.Println("  Unknown entries (informational, never touched):")
		for _, name := range report.Unknown {
			fmt.Printf("    %s\n", name)
		}
	}
	return nil
}

// runDoctor renders the read-only health report and exits non-zero on a
// degraded or blocked verdict.
func runDoctor() error {
	service, err := newService()
	if err != nil {
		return err
	}
	report, err := service.Doctor()
	if err != nil {
		return err
	}

	fmt.Printf("cortex-ia doctor — home %s\n", report.HomeDir)
	fmt.Printf("  Verdict: %s\n", report.Verdict)
	if report.OpencodeRoot != "" {
		fmt.Printf("  OpenCode root: %s\n", report.OpencodeRoot)
		fmt.Printf("  Selection: cortex=%v context7=%v; work-control=builtin\n",
			report.Selection.Cortex, report.Selection.Context7)
	}
	if counts := artifactCounts(report.Artifacts); len(counts) > 0 {
		fmt.Printf("  Artifacts: %s\n", counts)
	}
	if len(report.MCPs) > 0 {
		fmt.Println("  Managed MCPs:")
		for _, mcp := range report.MCPs {
			fmt.Printf("    %-24s %s (expected: %v)\n", mcp.Name, mcp.Status, mcp.Expected)
		}
	}
	if len(report.UnknownMCPs) > 0 {
		fmt.Printf("  Unknown MCPs: %s\n", strings.Join(report.UnknownMCPs, ", "))
	}
	for _, finding := range report.Findings {
		fmt.Printf("  - %s\n", finding)
	}

	switch report.Verdict {
	case install.DoctorDegraded, install.DoctorBlocked:
		return fmt.Errorf("doctor verdict: %s", report.Verdict)
	}
	return nil
}

// artifactCounts summarizes per-artifact statuses without listing paths.
func artifactCounts(checks []install.ArtifactCheck) string {
	var ok, missing, drifted, irregular int
	for _, check := range checks {
		switch check.Status {
		case install.ArtifactOK:
			ok++
		case install.ArtifactMissing:
			missing++
		case install.ArtifactDrifted:
			drifted++
		case install.ArtifactIrregular:
			irregular++
		}
	}
	parts := make([]string, 0, 4)
	if ok > 0 {
		parts = append(parts, fmt.Sprintf("%d ok", ok))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if drifted > 0 {
		parts = append(parts, fmt.Sprintf("%d drifted", drifted))
	}
	if irregular > 0 {
		parts = append(parts, fmt.Sprintf("%d irregular", irregular))
	}
	return strings.Join(parts, ", ")
}

// runRollback restores the recorded backup, or the one named on the command
// line. The real run is destructive and requires interactive confirmation;
// piped or closed input fails closed before the service is called.
func runRollback(args []string) error {
	service, err := newService()
	if err != nil {
		return err
	}

	if len(args) == 1 && (strings.EqualFold(args[0], "list") || strings.EqualFold(args[0], "--list")) {
		manifests, err := service.ListBackups()
		if err != nil {
			return err
		}
		if len(manifests) == 0 {
			fmt.Println("No backups found.")
			return nil
		}
		fmt.Printf("Available Backups (%d):\n", len(manifests))
		for _, m := range manifests {
			archiveInfo := ""
			if m.ArchiveFile != "" && m.ArchiveSize > 0 {
				archiveInfo = fmt.Sprintf(" [compressed: %s]", backup.FormatBytes(m.ArchiveSize))
			}
			fmt.Printf("  %-38s %s%s\n", m.ID, m.DisplayLabel(), archiveInfo)
		}
		fmt.Println("\nTo restore a backup, run: cortex-ia rollback <backup-id>")
		return nil
	}

	var backupID string
	switch len(args) {
	case 0:
	case 1:
		if strings.HasPrefix(args[0], "-") {
			return fmt.Errorf("unknown flag: %s (cortex-ia rollback takes an optional backup ID or 'list')", args[0])
		}
		backupID = args[0]
	default:
		return fmt.Errorf("unexpected argument: %s (cortex-ia rollback takes at most one backup ID)", args[1])
	}
	if err := confirmDestructive("rollback (restore the recorded backup; managed files written after it are removed or reverted)"); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	receipt, err := service.Rollback(backupID)
	if err != nil {
		return err
	}

	fmt.Printf("Rollback complete — backup %s (verified: %v)\n", receipt.BackupID, receipt.Verified)
	fmt.Printf("  Files restored: %d\n", len(receipt.Restored))
	fmt.Printf("  Files removed: %d\n", len(receipt.Removed))
	return nil
}

// runRecover dispatches the recovery surface: with no arguments or "list"
// it prints the pending journals read-only; with a journal ID it restores
// exactly that one journal after an interactive confirmation bound to the
// exact ID.
func runRecover(args []string) error {
	var journalID string
	list := true
	switch len(args) {
	case 0:
	case 1:
		if strings.EqualFold(args[0], "list") {
			break
		}
		if strings.HasPrefix(args[0], "-") {
			return fmt.Errorf("unknown flag: %s (cortex-ia recover takes 'list' or exactly one journal ID; see 'cortex-ia recover list')", args[0])
		}
		list = false
		journalID = args[0]
	default:
		return fmt.Errorf("unexpected argument: %s (cortex-ia recover takes 'list' or exactly one journal ID)", args[1])
	}

	service, err := newService()
	if err != nil {
		return err
	}
	if list {
		return runRecoverList(service)
	}
	return runRecoverExecute(service, journalID)
}

// runRecoverList prints every pending journal candidate with its
// recoverability classification. The listing is read-only: it never takes
// the home lock and never mutates anything.
func runRecoverList(service *install.Service) error {
	candidates, err := service.PendingJournals()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		fmt.Println("No pending recovery journals.")
		return nil
	}
	fmt.Printf("Pending recovery journals (%d):\n", len(candidates))
	for _, candidate := range candidates {
		status := "recoverable"
		if !candidate.Recoverable {
			status = "not recoverable"
		}
		fmt.Printf("  %s  [%s]  state=%s", candidate.ID, status, candidate.State)
		if candidate.BackupID != "" {
			fmt.Printf("  backup=%s", candidate.BackupID)
		}
		fmt.Println()
		if candidate.Reason != "" {
			fmt.Printf("    %s\n", candidate.Reason)
		}
		for _, target := range candidate.Targets {
			fmt.Printf("    target %s\n", target)
		}
	}
	fmt.Println("Restore one with: cortex-ia recover <journal-id> (requires an interactive terminal and typing the exact journal ID).")
	return nil
}

// runRecoverExecute restores one pending journal. Recovery is destructive:
// the candidate is resolved read-only first, then the user must confirm on
// an interactive terminal by typing the exact journal ID, and only then is
// the service called — so the home lock is never held during a prompt, and
// piped or closed input fails closed before any write.
func runRecoverExecute(service *install.Service, journalID string) error {
	candidates, err := service.PendingJournals()
	if err != nil {
		return err
	}
	var candidate *install.JournalCandidate
	for i := range candidates {
		if candidates[i].ID == journalID {
			candidate = &candidates[i]
			break
		}
	}
	if candidate == nil {
		return fmt.Errorf("recover %q: %w (run 'cortex-ia recover list' for the pending journals of this home)", journalID, install.ErrJournalNotRecoverable)
	}
	if !candidate.Recoverable {
		return fmt.Errorf("recover %q failed closed (nothing was written): %w: %s", journalID, install.ErrJournalNotRecoverable, candidate.Reason)
	}
	fmt.Printf("Journal %s (state %s) declares %d target(s):\n", candidate.ID, candidate.State, len(candidate.Targets))
	for _, target := range candidate.Targets {
		fmt.Printf("  %s\n", target)
	}
	if err := confirmRecovery(journalID); err != nil {
		return fmt.Errorf("recover: %w", err)
	}

	receipt, err := service.Recover(journalID, install.RecoverOptions{Confirmed: true})
	if err != nil {
		if errors.Is(err, install.ErrJournalNotRecoverable) || errors.Is(err, install.ErrConfirmationRequired) {
			return fmt.Errorf("recover %q failed closed (nothing was written): %w", journalID, err)
		}
		return err
	}
	printRecoveryReceipt(receipt)
	return nil
}

// confirmRecovery demands the strongest explicit confirmation: on an
// interactive terminal the user must type the exact journal ID being
// recovered. Any mismatch, piped or redirected input, or EOF refuses and
// writes nothing.
func confirmRecovery(journalID string) error {
	if !stdinIsInteractive() {
		return fmt.Errorf("recover %s requires an interactive terminal for explicit confirmation; refusing to proceed (nothing was written)", journalID)
	}
	fmt.Printf("Recovering journal %s is destructive: its journaled preimages are restored over the current files.\n", journalID)
	fmt.Print("Type the journal ID exactly to confirm (anything else refuses): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("recover %s: no confirmation was read; refusing to proceed (nothing was written)", journalID)
	}
	if strings.TrimSpace(line) != journalID {
		return fmt.Errorf("recover %s: confirmation did not match the journal ID; refusing to proceed (nothing was written)", journalID)
	}
	return nil
}

// printRecoveryReceipt renders the typed recovery outcome.
func printRecoveryReceipt(receipt *install.RecoveryReceipt) {
	fmt.Printf("Recovery complete — journal %s\n", receipt.JournalID)
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s\n", receipt.BackupID)
	}
	fmt.Printf("  Disposition: %s\n", receipt.Disposition)
	fmt.Printf("  Restored: %d  Verified: %v\n", len(receipt.Restored), receipt.Verified)
	if receipt.RestoreError != "" {
		fmt.Printf("  Restore error: %s (journal retained for a safe retry)\n", receipt.RestoreError)
	}
}

// runUninstall removes the accredited installation. Every deletion is
// ownership- and digest-accredited by the service; anything unverifiable is
// retained and reported. The real run is destructive and requires
// interactive confirmation; piped or closed input fails closed before the
// service is called.
func runUninstall(args []string) error {
	flags, err := parseRunFlags(args, "uninstall", false)
	if err != nil {
		return err
	}
	service, err := newService()
	if err != nil {
		return err
	}
	if !flags.DryRun {
		if err := confirmDestructive("uninstall (remove the accredited cortex-ia installation)"); err != nil {
			return fmt.Errorf("uninstall: %w", err)
		}
	}

	receipt, err := service.Uninstall(install.UninstallOptions{DryRun: flags.DryRun})
	if err != nil {
		return err
	}

	title := "uninstall"
	if receipt.DryRun {
		title += " (dry-run)"
	}
	fmt.Printf("cortex-ia %s\n", title)
	if receipt.NotInstalled {
		fmt.Println("  No cortex-ia installation metadata found; nothing to remove.")
		return nil
	}
	fmt.Printf("  Removed: %d\n", len(receipt.Removed))
	fmt.Printf("  MCP entries removed: %s\n", strings.Join(defaultStringSlice(receipt.MCPRemoved), ", "))
	fmt.Printf("  Already absent: %d\n", len(receipt.AlreadyAbsent))
	fmt.Printf("  Directories pruned: %d\n", len(receipt.RemovedDirs))
	for _, preserved := range receipt.Preserved {
		fmt.Printf("  Preserved (co-owned): %s\n", preserved)
	}
	for _, retained := range receipt.Retained {
		fmt.Printf("  Retained: %s — %s\n", retained.Target, retained.Reason)
	}
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s\n", receipt.BackupID)
	}
	fmt.Printf("  State removed: %v  Complete: %v\n", receipt.StateRemoved, receipt.Complete)
	return nil
}

// presetNames renders the managed preset catalog for usage messages.
func presetNames() string {
	names := make([]string, 0, 4)
	for _, preset := range mcpmanager.Presets() {
		names = append(names, preset.Name)
	}
	return strings.Join(names, ", ")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{"(none)"}
	}
	return values
}
