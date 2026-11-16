package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/v2/internal/targets"
)

// hostChangeLimit caps the change lines rendered per host before the rest are
// summarized.
const hostChangeLimit = 8

// hostRun is one multi-host operation in flight. The OpenCode step (when
// selected) runs through the existing Review or uninstall flow; its opResult
// is kept here until the extra hosts finish, so one Result screen reports all.
type hostRun struct {
	active           bool
	uninstall        bool
	openCodeSelected bool
	openCode         *opResult
	title            string
	pending          []targets.TargetID
	outcomes         []hostOutcome
}

// hostOutcome is the typed projection of one extra host's run.
type hostOutcome struct {
	id       targets.TargetID
	label    string
	success  bool
	message  string
	changes  []string
	err      string
	conflict bool
}

// hostStepMsg reports one finished extra host back to Update.
type hostStepMsg struct {
	outcome hostOutcome
}

// hostConflictMarker is the text internal/targets uses when an apply refuses
// to overwrite user-owned files; the hint in the Result screen keys off it.
const hostConflictMarker = "would be overwritten; nothing was written"

func newHostOutcome(id targets.TargetID, res *targets.TargetResult, err error) hostOutcome {
	o := hostOutcome{id: id, label: targets.TargetLabel(id)}
	if res != nil {
		o.success = res.Success
		o.message = res.Message
		o.changes = res.Changes
	}
	switch {
	case err != nil:
		o.success = false
		o.err = err.Error()
		o.conflict = strings.Contains(o.err, hostConflictMarker)
	case res == nil:
		o.err = "no result reported"
	}
	return o
}

func hostStepCmd(hosts TargetAPI, id targets.TargetID, uninstall bool) tea.Cmd {
	return func() tea.Msg {
		if uninstall {
			res, err := hosts.Uninstall(id)
			return hostStepMsg{outcome: newHostOutcome(id, res, err)}
		}
		res, err := hosts.Install(id, targets.ApplyOptions{})
		return hostStepMsg{outcome: newHostOutcome(id, res, err)}
	}
}

// startHostStep opens the Running screen with one phase per remaining host and
// dispatches the first of them; each finished host chains the next.
func (m model) startHostStep() (tea.Model, tea.Cmd) {
	phases := make([]string, 0, len(m.hostRun.pending))
	for _, id := range m.hostRun.pending {
		phases = append(phases, targets.TargetLabel(id))
	}
	next := m.hostRun.pending[0]
	m.hostRun.pending = m.hostRun.pending[1:]
	return m.startRunning(m.hostRun.title, phases, hostStepCmd(m.hosts, next, m.hostRun.uninstall))
}

func (m model) onHostStep(msg hostStepMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenRunning || !m.hostRun.active {
		return m, nil
	}
	m.advanceRunning()
	m.hostRun.outcomes = append(m.hostRun.outcomes, msg.outcome)
	if len(m.hostRun.pending) > 0 {
		next := m.hostRun.pending[0]
		m.hostRun.pending = m.hostRun.pending[1:]
		return m, hostStepCmd(m.hosts, next, m.hostRun.uninstall)
	}
	return m.finishHostRun()
}

// finishOpStep receives the OpenCode step's result. Without extra hosts it is
// the untouched single-host path; otherwise the extra hosts run only after the
// OpenCode step passed.
func (m model) finishOpStep(res opResult) (tea.Model, tea.Cmd) {
	if !m.hostRun.active {
		m.result = res
		m.resultScroll = 0
		m.screen = screenResult
		return m, nil
	}
	m.hostRun.openCode = &res
	if res.pass && len(m.hostRun.pending) > 0 {
		return m.startHostStep()
	}
	return m.finishHostRun()
}

// finishHostRun assembles the Result screen: the OpenCode receipt (or a
// placeholder when it did not run), then one section per extra host, then the
// hosts skipped because an earlier step did not succeed.
func (m model) finishHostRun() (tea.Model, tea.Cmd) {
	run := m.hostRun
	res := opResult{title: run.title, pass: true}
	skipReason := "OpenCode step did not succeed"
	switch {
	case run.openCode != nil:
		res = *run.openCode
	case run.openCodeSelected:
		res = opResult{title: run.title, pass: false, detail: []string{"OpenCode: review cancelled; nothing was changed."}}
		skipReason = "OpenCode review cancelled"
	}
	for _, o := range run.outcomes {
		res.detail = append(res.detail, hostOutcomeLines(o)...)
		res.changed += len(o.changes)
		res.pass = res.pass && o.success
	}
	for _, id := range run.pending {
		res.detail = append(res.detail, fmt.Sprintf("%s: skipped (%s)", targets.TargetLabel(id), skipReason))
		res.pass = false
	}
	m.hostRun = hostRun{}
	m.result = res
	m.resultScroll = 0
	m.screen = screenResult
	return m, nil
}

// hostOutcomeLines renders one extra host's section of the Result screen.
func hostOutcomeLines(o hostOutcome) []string {
	verdict := "FAIL"
	if o.success {
		verdict = "PASS"
	}
	lines := []string{fmt.Sprintf("%s: %s", o.label, verdict)}
	for _, line := range strings.Split(o.message, "\n") {
		if line != "" {
			lines = append(lines, "  "+line)
		}
	}
	for i, change := range o.changes {
		if i == hostChangeLimit {
			lines = append(lines, fmt.Sprintf("  … and %d more", len(o.changes)-hostChangeLimit))
			break
		}
		lines = append(lines, "  "+change)
	}
	if o.err != "" {
		for i, line := range strings.Split(o.err, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if i == 0 {
				lines = append(lines, "  error: "+strings.TrimSpace(line))
				continue
			}
			lines = append(lines, "  "+line)
		}
	}
	if o.conflict {
		lines = append(lines, fmt.Sprintf("  Re-run with: cortex-ia install --target %s --overwrite", o.id))
	}
	return lines
}
