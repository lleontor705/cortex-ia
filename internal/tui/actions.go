package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

// Phase sets shown by the Running screen per operation kind.
var (
	installPhases   = []string{"Plan", "Backup", "Apply", "Verify", "Commit"}
	uninstallPhases = []string{"Inspect", "Backup", "Remove", "Verify"}
	rollbackPhases  = []string{"Read manifest", "Verify restorable", "Restore"}
	mcpPhases       = []string{"Backup", "Update config", "Commit state"}
)

// tickMsg advances the Running screen spinner.
type tickMsg time.Time

func spinTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Operation messages carry the typed service receipts back to Update.
type (
	planMsg struct {
		plan *pipeline.Plan
		err  error
	}
	installMsg struct {
		receipt *install.InstallReceipt
		err     error
	}
	doctorMsg struct {
		report *install.DoctorReport
		err    error
	}
	uninstallMsg struct {
		receipt *install.UninstallReceipt
		err     error
	}
	rollbackMsg struct {
		receipt *install.RollbackReceipt
		err     error
	}
	mcpListMsg struct {
		report *install.MCPListReport
		err    error
	}
	mcpMutateMsg struct {
		receipt *install.MCPReceipt
		err     error
		// add distinguishes an add outcome (qualification required for
		// PASS) from a removal outcome.
		add bool
	}
)

// reviewOptions projects the current Review state onto read-only planning
// options: the selection plus the explicit overwrite authorization.
func (m model) reviewOptions() install.Options {
	opts := m.opts
	opts.Overwrite = m.overwrite
	opts.DryRun = true
	return opts
}

// confirmedOptions freezes the reviewed plan into the mutating request:
// DryRun clears and the displayed plan's digest travels as
// ExpectedPlanDigest, so the service re-plans with identical options and
// rejects any drift before a backup or write can begin. The TUI never
// independently authorizes a plan different from the one it rendered.
func (m model) confirmedOptions() install.Options {
	opts := m.reviewOptions()
	opts.DryRun = false
	if m.plan != nil {
		opts.ExpectedPlanDigest = m.plan.Digest
	}
	return opts
}

// planCmd re-derives the read-only plan for the given options.
func planCmd(svc ServiceAPI, opts install.Options) tea.Cmd {
	return func() tea.Msg {
		plan, err := svc.Plan(opts)
		return planMsg{plan: plan, err: err}
	}
}

func doctorCmd(svc ServiceAPI) tea.Cmd {
	return func() tea.Msg {
		report, err := svc.Doctor()
		return doctorMsg{report: report, err: err}
	}
}

func uninstallCmd(svc ServiceAPI) tea.Cmd {
	return func() tea.Msg {
		receipt, err := svc.Uninstall(install.UninstallOptions{})
		return uninstallMsg{receipt: receipt, err: err}
	}
}

func rollbackCmd(svc ServiceAPI) tea.Cmd {
	return func() tea.Msg {
		receipt, err := svc.Rollback("")
		return rollbackMsg{receipt: receipt, err: err}
	}
}

func mcpListCmd(svc ServiceAPI) tea.Cmd {
	return func() tea.Msg {
		report, err := svc.MCPList()
		return mcpListMsg{report: report, err: err}
	}
}

// mcpMutateCmd adds (add=true) or removes one managed preset entry.
func mcpMutateCmd(svc ServiceAPI, name string, add bool) tea.Cmd {
	return func() tea.Msg {
		if add {
			receipt, err := svc.MCPAdd(name, install.MCPOptions{})
			return mcpMutateMsg{receipt: receipt, err: err, add: true}
		}
		receipt, err := svc.MCPRemove(name, install.MCPOptions{})
		return mcpMutateMsg{receipt: receipt, err: err, add: false}
	}
}

// installRunCmd executes the operation the Review screen selected. The
// options already carry the confirmed overwrite authorization and the
// expected digest of the confirmed plan.
func installRunCmd(svc ServiceAPI, mode string, opts install.Options) tea.Cmd {
	opts.DryRun = false
	return func() tea.Msg {
		if mode == "sync" {
			receipt, err := svc.Sync(opts)
			return installMsg{receipt: receipt, err: err}
		}
		receipt, err := svc.Install(opts)
		return installMsg{receipt: receipt, err: err}
	}
}

// --- Result assembly ---

// onInstallDone renders install/sync receipts. PASS derives from the
// receipt, never from a nil error alone: the service must succeed AND
// every MCP the run configured must carry valid qualification evidence.
// Configured-only entries surface as a visible FAIL with a remedy.
func (m model) onInstallDone(msg installMsg) (tea.Model, tea.Cmd) {
	m.advanceRunning()
	res := opResult{title: m.running.title, pass: msg.err == nil && installQualificationPasses(msg.receipt)}
	if msg.receipt != nil {
		res.changed = len(msg.receipt.Changed)
		res.backupID = msg.receipt.BackupID
		res.detail = installDetail(msg.receipt)
		if msg.receipt.Converged {
			res.detail = append(res.detail, "Already converged: zero writes needed.")
		}
	}
	if missing := unqualifiedMCPs(msg.receipt); len(missing) > 0 {
		res.detail = append(res.detail,
			fmt.Sprintf("FAIL: configured without valid qualification evidence: %s", strings.Join(missing, ", ")),
			"remedy: verify the MCP servers are reachable and re-run, or open Doctor / Recovery for per-MCP status.",
		)
	}
	res.rollbackCmd = rollbackCommand(res.backupID)
	res.canRollback = res.backupID != "" && res.pass
	if msg.err != nil {
		if errors.Is(msg.err, pipeline.ErrPlanDrift) {
			res.detail = append(res.detail,
				"plan drift: this home changed after the plan was confirmed; nothing was written.",
				"remedy: go back and review the fresh plan, then confirm it again.",
			)
		}
		res.detail = append(res.detail, "error: "+msg.err.Error())
	}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

// installQualificationPasses reports whether an install/sync receipt earns
// its PASS: a nil receipt never passes, and every configured MCP name must
// appear in the qualified set. An empty configured set (no MCPs selected)
// passes vacuously. Dry-run receipts report no qualified entries and are
// never rendered through this path.
func installQualificationPasses(receipt *install.InstallReceipt) bool {
	if receipt == nil {
		return false
	}
	return len(unqualifiedMCPs(receipt)) == 0
}

// unqualifiedMCPs lists the configured entries missing from the receipt's
// qualified set: present in the managed configuration but lacking the probe
// evidence validated during this run.
func unqualifiedMCPs(receipt *install.InstallReceipt) []string {
	if receipt == nil {
		return nil
	}
	qualified := make(map[string]bool, len(receipt.Qualified))
	for _, name := range receipt.Qualified {
		qualified[name] = true
	}
	var missing []string
	for _, name := range receipt.Configured {
		if !qualified[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func installDetail(receipt *install.InstallReceipt) []string {
	detail := []string{
		fmt.Sprintf("Plan digest: %s", receipt.PlanDigest),
		fmt.Sprintf("Configured MCPs: %s", joinOrDash(receipt.Configured)),
	}
	if len(receipt.Qualified) > 0 {
		detail = append(detail, fmt.Sprintf("Qualified MCPs: %s", strings.Join(receipt.Qualified, ", ")))
	}
	for _, conflict := range receipt.Conflicts {
		detail = append(detail, fmt.Sprintf("conflict %s: %s (%s)", conflict.Target, conflict.Kind, conflict.Reason))
	}
	detail = append(detail, receipt.Warnings...)
	return detail
}

func (m model) onDoctorDone(msg doctorMsg) (tea.Model, tea.Cmd) {
	m.advanceRunning()
	res := opResult{title: "Doctor", pass: msg.err == nil && msg.report != nil && msg.report.Verdict == install.DoctorHealthy}
	if msg.err != nil {
		res.detail = []string{"error: " + msg.err.Error()}
	} else if msg.report != nil {
		res.detail = doctorDetail(msg.report)
		// Recovery: an unhealthy home can offer a rollback to the recorded
		// backup; the service fails closed when none exists.
		res.canRollback = msg.report.Verdict != install.DoctorHealthy
		res.rollbackCmd = rollbackCommand("")
	}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

func doctorDetail(report *install.DoctorReport) []string {
	detail := []string{fmt.Sprintf("Verdict: %s", report.Verdict)}
	ok, missing, drifted := 0, 0, 0
	for _, artifact := range report.Artifacts {
		switch artifact.Status {
		case install.ArtifactOK:
			ok++
		case install.ArtifactMissing:
			missing++
		case install.ArtifactDrifted:
			drifted++
		}
	}
	detail = append(detail, fmt.Sprintf("Artifacts: %d ok, %d missing, %d drifted", ok, missing, drifted))
	for _, check := range report.MCPs {
		detail = append(detail, fmt.Sprintf("MCP %s: %s (expected: %v)", check.Name, check.Status, check.Expected))
	}
	detail = append(detail, report.Findings...)
	return detail
}

func (m model) onUninstallDone(msg uninstallMsg) (tea.Model, tea.Cmd) {
	m.advanceRunning()
	res := opResult{title: "Uninstall", pass: msg.err == nil && msg.receipt != nil && msg.receipt.Complete}
	if msg.receipt != nil {
		res.changed = len(msg.receipt.Removed)
		res.backupID = msg.receipt.BackupID
		res.detail = uninstallDetail(msg.receipt)
	}
	res.rollbackCmd = rollbackCommand(res.backupID)
	res.canRollback = res.backupID != "" && msg.err == nil
	if msg.err != nil {
		res.detail = append(res.detail, "error: "+msg.err.Error())
	}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

func uninstallDetail(receipt *install.UninstallReceipt) []string {
	detail := []string{
		fmt.Sprintf("Removed: %d file(s), %d MCP entr(ies)", len(receipt.Removed), len(receipt.MCPRemoved)),
	}
	if receipt.NotInstalled {
		detail = append(detail, "Nothing installed: nothing was touched.")
	}
	if len(receipt.Preserved) > 0 {
		detail = append(detail, fmt.Sprintf("Preserved co-owned: %s", strings.Join(receipt.Preserved, ", ")))
	}
	for _, item := range receipt.Retained {
		detail = append(detail, fmt.Sprintf("retained %s: %s", item.Target, item.Reason))
	}
	if receipt.StateRemoved {
		detail = append(detail, "v2 state and lock removed.")
	}
	return detail
}

func (m model) onRollbackDone(msg rollbackMsg) (tea.Model, tea.Cmd) {
	m.advanceRunning()
	res := opResult{title: "Rollback", pass: msg.err == nil && msg.receipt != nil && msg.receipt.Verified}
	if msg.receipt != nil {
		res.changed = len(msg.receipt.Restored) + len(msg.receipt.Removed)
		res.backupID = msg.receipt.BackupID
		res.detail = []string{
			fmt.Sprintf("Backup: %s (verified: %v)", msg.receipt.BackupID, msg.receipt.Verified),
			fmt.Sprintf("Restored: %d, removed post-backup: %d", len(msg.receipt.Restored), len(msg.receipt.Removed)),
		}
	}
	if msg.err != nil {
		res.detail = append(res.detail, "error: "+msg.err.Error())
	}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

func (m model) onMCPMutateDone(msg mcpMutateMsg) (tea.Model, tea.Cmd) {
	m.advanceRunning()
	res := opResult{title: m.running.title}
	if msg.add {
		// An add earns PASS only when the receipt reports Installed:
		// configured with accredited ownership AND validated by explicit
		// probe evidence during this call. Configured-only is a FAIL.
		res.pass = msg.err == nil && msg.receipt != nil && msg.receipt.Installed
	} else {
		res.pass = msg.err == nil
	}
	if msg.receipt != nil {
		res.changed = 0
		if msg.receipt.Changed {
			res.changed = 1
		}
		res.backupID = msg.receipt.BackupID
		res.detail = mcpDetail(msg.receipt)
	}
	if msg.add && msg.err == nil && (msg.receipt == nil || !msg.receipt.Installed) {
		res.detail = append(res.detail,
			"FAIL: MCP entry configured without valid qualification evidence (installed=false).",
			"remedy: verify the MCP server is reachable and re-add it, or open Doctor / Recovery.",
		)
	}
	res.rollbackCmd = rollbackCommand(res.backupID)
	res.canRollback = res.backupID != "" && res.pass
	if msg.err != nil {
		res.detail = append(res.detail, "error: "+msg.err.Error())
	}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

func mcpDetail(receipt *install.MCPReceipt) []string {
	detail := []string{
		fmt.Sprintf("Action: %s (dry-run: %v)", receipt.Action, receipt.DryRun),
		fmt.Sprintf("Configured: %v, qualified: %v, installed: %v", receipt.Configured, receipt.Qualified, receipt.Installed),
	}
	detail = append(detail, receipt.Warnings...)
	return detail
}

// rollbackCommand renders the CLI command that restores this backup.
func rollbackCommand(backupID string) string {
	if backupID == "" {
		return "cortex-ia rollback"
	}
	return "cortex-ia rollback " + backupID
}

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	return strings.Join(items, ", ")
}
