package diagram

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ComponentDiff records changes to one component.
type ComponentDiff struct {
	ID       string   `json:"id"`
	Type     string   `json:"type,omitempty"`
	Label    string   `json:"label,omitempty"`
	OldLabel string   `json:"old_label,omitempty"`
	DiffType string   `json:"diff_type"` // "added", "removed", "modified"
	Changes  []string `json:"changes,omitempty"`
}

// ConnectionDiff records changes to one connection.
type ConnectionDiff struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Label    string   `json:"label,omitempty"`
	DiffType string   `json:"diff_type"` // "added", "removed", "modified"
	Changes  []string `json:"changes,omitempty"`
}

// DeltaReport summarizes the architectural delta between base and head.
type DeltaReport struct {
	BaseTitle           string           `json:"base_title"`
	HeadTitle           string           `json:"head_title"`
	ComponentsAdded     []ComponentDiff  `json:"components_added"`
	ComponentsRemoved   []ComponentDiff  `json:"components_removed"`
	ComponentsModified  []ComponentDiff  `json:"components_modified"`
	ConnectionsAdded    []ConnectionDiff `json:"connections_added"`
	ConnectionsRemoved  []ConnectionDiff `json:"connections_removed"`
	ConnectionsModified []ConnectionDiff `json:"connections_modified"`
	TotalChanges        int              `json:"total_changes"`
	Summary             string           `json:"summary"`
}

// CompareFiles loads two diagram JSON files and computes their architectural delta.
func CompareFiles(basePath, headPath string, outputHtmlPath string) (*DeltaReport, error) {
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("read base diagram %s: %w", basePath, err)
	}
	headData, err := os.ReadFile(headPath)
	if err != nil {
		return nil, fmt.Errorf("read head diagram %s: %w", headPath, err)
	}
	return Compare(baseData, headData, outputHtmlPath)
}

// Compare computes the architectural difference between two JSON specifications.
func Compare(baseData, headData []byte, outputHtmlPath string) (*DeltaReport, error) {
	var base, head DiagramSpecification
	if err := json.Unmarshal(baseData, &base); err != nil {
		return nil, fmt.Errorf("parse base specification: %w", err)
	}
	if err := json.Unmarshal(headData, &head); err != nil {
		return nil, fmt.Errorf("parse head specification: %w", err)
	}

	report := &DeltaReport{
		BaseTitle:           base.Meta.Title,
		HeadTitle:           head.Meta.Title,
		ComponentsAdded:     []ComponentDiff{},
		ComponentsRemoved:   []ComponentDiff{},
		ComponentsModified:  []ComponentDiff{},
		ConnectionsAdded:    []ConnectionDiff{},
		ConnectionsRemoved:  []ConnectionDiff{},
		ConnectionsModified: []ConnectionDiff{},
	}

	baseCompMap := make(map[string]Component)
	for _, c := range base.Components {
		baseCompMap[c.ID] = c
	}
	headCompMap := make(map[string]Component)
	for _, c := range head.Components {
		headCompMap[c.ID] = c
	}

	// 1. Detect removed or modified components
	for id, bComp := range baseCompMap {
		hComp, exists := headCompMap[id]
		if !exists {
			report.ComponentsRemoved = append(report.ComponentsRemoved, ComponentDiff{
				ID:       id,
				Type:     bComp.Type,
				Label:    bComp.Label,
				DiffType: "removed",
			})
			continue
		}

		var changes []string
		if bComp.Label != hComp.Label {
			changes = append(changes, fmt.Sprintf("label changed from %q to %q", bComp.Label, hComp.Label))
		}
		if bComp.Type != hComp.Type {
			changes = append(changes, fmt.Sprintf("type changed from %q to %q", bComp.Type, hComp.Type))
		}
		if bComp.Sublabel != hComp.Sublabel {
			changes = append(changes, fmt.Sprintf("sublabel changed from %q to %q", bComp.Sublabel, hComp.Sublabel))
		}
		if bComp.Variant != hComp.Variant {
			changes = append(changes, fmt.Sprintf("variant changed from %q to %q", bComp.Variant, hComp.Variant))
		}

		if len(changes) > 0 {
			report.ComponentsModified = append(report.ComponentsModified, ComponentDiff{
				ID:       id,
				Type:     hComp.Type,
				Label:    hComp.Label,
				OldLabel: bComp.Label,
				DiffType: "modified",
				Changes:  changes,
			})
		}
	}

	// 2. Detect added components
	for id, hComp := range headCompMap {
		if _, exists := baseCompMap[id]; !exists {
			report.ComponentsAdded = append(report.ComponentsAdded, ComponentDiff{
				ID:       id,
				Type:     hComp.Type,
				Label:    hComp.Label,
				DiffType: "added",
			})
		}
	}

	// 3. Compare connections
	connKey := func(c Connection) string {
		return fmt.Sprintf("%s->%s", c.From, c.To)
	}
	baseConnMap := make(map[string]Connection)
	for _, c := range base.Connections {
		baseConnMap[connKey(c)] = c
	}
	headConnMap := make(map[string]Connection)
	for _, c := range head.Connections {
		headConnMap[connKey(c)] = c
	}

	for k, bConn := range baseConnMap {
		hConn, exists := headConnMap[k]
		if !exists {
			report.ConnectionsRemoved = append(report.ConnectionsRemoved, ConnectionDiff{
				From:     bConn.From,
				To:       bConn.To,
				Label:    bConn.Label,
				DiffType: "removed",
			})
			continue
		}
		var changes []string
		if bConn.Label != hConn.Label {
			changes = append(changes, fmt.Sprintf("label changed from %q to %q", bConn.Label, hConn.Label))
		}
		if bConn.Protocol != hConn.Protocol {
			changes = append(changes, fmt.Sprintf("protocol changed from %q to %q", bConn.Protocol, hConn.Protocol))
		}
		if bConn.Style != hConn.Style {
			changes = append(changes, fmt.Sprintf("style changed from %q to %q", bConn.Style, hConn.Style))
		}
		if len(changes) > 0 {
			report.ConnectionsModified = append(report.ConnectionsModified, ConnectionDiff{
				From:     hConn.From,
				To:       hConn.To,
				Label:    hConn.Label,
				DiffType: "modified",
				Changes:  changes,
			})
		}
	}

	for k, hConn := range headConnMap {
		if _, exists := baseConnMap[k]; !exists {
			report.ConnectionsAdded = append(report.ConnectionsAdded, ConnectionDiff{
				From:     hConn.From,
				To:       hConn.To,
				Label:    hConn.Label,
				DiffType: "added",
			})
		}
	}

	// Sort slices for deterministic output
	sort.Slice(report.ComponentsAdded, func(i, j int) bool { return report.ComponentsAdded[i].ID < report.ComponentsAdded[j].ID })
	sort.Slice(report.ComponentsRemoved, func(i, j int) bool { return report.ComponentsRemoved[i].ID < report.ComponentsRemoved[j].ID })
	sort.Slice(report.ComponentsModified, func(i, j int) bool { return report.ComponentsModified[i].ID < report.ComponentsModified[j].ID })

	report.TotalChanges = len(report.ComponentsAdded) + len(report.ComponentsRemoved) + len(report.ComponentsModified) +
		len(report.ConnectionsAdded) + len(report.ConnectionsRemoved) + len(report.ConnectionsModified)

	report.Summary = fmt.Sprintf("Architecture Delta: +%d/-%d/~%d components, +%d/-%d/~%d routes (Total changes: %d)",
		len(report.ComponentsAdded), len(report.ComponentsRemoved), len(report.ComponentsModified),
		len(report.ConnectionsAdded), len(report.ConnectionsRemoved), len(report.ConnectionsModified),
		report.TotalChanges)

	// Render comparison HTML if requested
	if outputHtmlPath != "" {
		html := generateComparisonHtml(report, base, head)
		if err := os.WriteFile(outputHtmlPath, []byte(html), 0644); err != nil {
			return nil, fmt.Errorf("write comparison HTML %s: %w", outputHtmlPath, err)
		}
	}

	return report, nil
}

func generateComparisonHtml(report *DeltaReport, base, head DiagramSpecification) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Architecture Delta: ` + report.BaseTitle + ` vs ` + report.HeadTitle + `</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 2rem; }
.container { max-width: 1000px; margin: 0 auto; }
.card { background: #1e293b; border-radius: 8px; padding: 1.5rem; margin-bottom: 1.5rem; border: 1px solid #334155; }
h1 { font-size: 1.75rem; margin-bottom: 0.5rem; }
.badge { display: inline-block; padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.85rem; font-weight: 600; margin-right: 0.5rem; }
.badge-add { background: #065f46; color: #34d399; }
.badge-del { background: #881337; color: #fda4af; }
.badge-mod { background: #854d0e; color: #fde047; }
ul { list-style: none; padding-left: 0; }
li { padding: 0.5rem 0; border-bottom: 1px solid #334155; }
.change-detail { font-size: 0.9rem; color: #94a3b8; margin-left: 1.5rem; }
</style>
</head>
<body>
<div class="container">
  <h1>` + report.Summary + `</h1>
  <div class="card">
    <h2>Components Delta</h2>
    <ul>`)

	for _, a := range report.ComponentsAdded {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-add\">+ ADDED</span> <strong>%s</strong> (%s) - %s</li>", a.ID, a.Type, a.Label)
	}
	for _, d := range report.ComponentsRemoved {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-del\">- REMOVED</span> <strong>%s</strong> (%s) - %s</li>", d.ID, d.Type, d.Label)
	}
	for _, m := range report.ComponentsModified {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-mod\">~ MODIFIED</span> <strong>%s</strong> (%s) - %s<div class=\"change-detail\">%s</div></li>",
			m.ID, m.Type, m.Label, strings.Join(m.Changes, "; "))
	}
	if len(report.ComponentsAdded) == 0 && len(report.ComponentsRemoved) == 0 && len(report.ComponentsModified) == 0 {
		sb.WriteString("<li>No component changes detected.</li>")
	}

	sb.WriteString(`</ul>
  </div>
  <div class="card">
    <h2>Connections / Routes Delta</h2>
    <ul>`)

	for _, a := range report.ConnectionsAdded {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-add\">+ ROUTE</span> %s &rarr; %s: %s</li>", a.From, a.To, a.Label)
	}
	for _, d := range report.ConnectionsRemoved {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-del\">- ROUTE</span> %s &rarr; %s: %s</li>", d.From, d.To, d.Label)
	}
	for _, m := range report.ConnectionsModified {
		fmt.Fprintf(&sb, "<li><span class=\"badge badge-mod\">~ ROUTE</span> %s &rarr; %s: %s<div class=\"change-detail\">%s</div></li>",
			m.From, m.To, m.Label, strings.Join(m.Changes, "; "))
	}
	if len(report.ConnectionsAdded) == 0 && len(report.ConnectionsRemoved) == 0 && len(report.ConnectionsModified) == 0 {
		sb.WriteString("<li>No connection changes detected.</li>")
	}

	sb.WriteString(`</ul>
  </div>
</div>
</body>
</html>`)
	return sb.String()
}
