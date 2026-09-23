package tui

import (
	"fmt"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

func (s modelsState) view(width int) string {
	w := width
	if w <= 0 {
		w = modelsDefaultWidth
	}

	switch {
	case s.loading && s.report == nil:
		return strings.Join([]string{
			modelsTitle(),
			"",
			styleDim.Render(styles.SpinnerChar(0) + " Cargando modelos de OpenCode…"),
			"",
			styleDim.Render(modelsDegradedHints),
		}, "\n")
	case s.err != nil:
		return strings.Join([]string{
			modelsTitle(),
			"",
			styleFail.Render("No se pudo leer la configuración de modelos: " + s.err.Error()),
			"",
			styleDim.Render(modelsDegradedHints),
		}, "\n")
	}

	if s.catalogLoading {
		return strings.Join([]string{
			modelsTitle(),
			"",
			styleDim.Render(styles.SpinnerChar(0) + " Cargando catálogo de modelos…"),
			"",
			styleDim.Render(modelsCatalogLoadHints),
		}, "\n")
	}

	switch s.phase {
	case modelsPhaseInput:
		switch s.step {
		case modelsStepEntries:
			return s.pickerView(w)
		case modelsStepVariants:
			return s.variantView(w)
		}
		return s.inputView(w)
	case modelsPhasePreview:
		return s.previewView(w)
	case modelsPhaseReceipt:
		return s.receiptView(w)
	default:
		return s.listView(w)
	}
}

func modelsTitle() string {
	return styleSubtitle.Render("Configuración de modelos")
}

func (s modelsState) listView(width int) string {
	lines := []string{modelsTitle(), s.sourceLine(width), ""}
	agents := s.agents()
	if len(agents) == 0 {
		lines = append(lines, styleDim.Render("Sin agentes en el registro."))
	} else {
		lines = append(lines, styleSubtitle.Render(fmt.Sprintf("%-*s  %-*s  %s",
			modelsAgentCol, "Agente", modelsRefCol, "Modelo / effort", "Fuente")))
		for i, entry := range agents {
			lines = append(lines, s.agentRow(entry, i == s.cursor, width))
		}
	}
	lines = append(lines, "", styleDim.Render(modelsListHints))
	return strings.Join(lines, "\n")
}

func (s modelsState) sourceLine(width int) string {
	if s.report == nil {
		return ""
	}
	installed := "ownership no disponible (hogar sin instalación acreditada)"
	if s.report.Installed {
		installed = "instalación v2 acreditada"
	}
	return styleDim.Render(truncate("config: "+s.report.ConfigPath+" · "+installed, width))
}

func (s modelsState) agentRow(entry modelmgr.AgentEntry, active bool, width int) string {
	prefix := "  "
	if active {
		prefix = "▸ "
	}
	row := fmt.Sprintf("%s%-*s  %-*s  %s",
		prefix, modelsAgentCol, entry.Agent, modelsRefCol, modelsRefLabel(entry), entry.Source)
	if entry.Source == modelmgr.SourceUnset {
		row = styleDim.Render(row)
	}
	if active {
		row = styleSelected.Render(row)
	}
	return truncate(row, width)
}

// modelsRefLabel renders the effective compact reference, naming the unset
// status explicitly so builtins without an assignment stay visible.
func modelsRefLabel(entry modelmgr.AgentEntry) string {
	switch {
	case entry.Model == "":
		if entry.MarkdownPin != "" {
			return entry.MarkdownPin
		}
		return "unset"
	case entry.Variant == "":
		return entry.Model
	default:
		return entry.Model + "#" + entry.Variant
	}
}

func (s modelsState) inputView(width int) string {
	lines := []string{
		modelsTitle(),
		"",
		styleSubtitle.Render("Agente: " + s.agentName()),
		"",
	}
	if s.inputNotice != "" {
		lines = append(lines, styleWarn.Render("⚠ "+s.inputNotice), "")
	}
	lines = append(lines,
		modelsInputLine("Modelo", s.input, s.focus == modelsFieldModel),
		modelsInputLine("Effort", s.effort, s.focus == modelsFieldEffort),
		"",
		styleDim.Render(truncate(modelsInputHints, width)),
	)
	return strings.Join(lines, "\n")
}

func (s modelsState) pickerView(width int) string {
	lines := []string{
		modelsTitle(),
		"",
		styleSubtitle.Render("Agente: " + s.agentName()),
		"",
		fmt.Sprintf("Filtro: %s▌", s.filter),
		"",
	}
	entries := s.filteredEntries()
	if len(entries) == 0 {
		lines = append(lines, styleDim.Render("Sin coincidencias."))
	} else {
		start := s.pickerWindow(len(entries))
		for i := start; i < len(entries) && i < start+modelsPickerRowLimit; i++ {
			lines = append(lines, truncate(s.pickerRow(entries[i], i == s.pickerCursor), width))
		}
	}
	if s.catalog != nil {
		lines = append(lines, "", styleDim.Render("catálogo: "+s.catalog.Source))
	}
	lines = append(lines, "", styleDim.Render(truncate(modelsPickerHints, width)))
	return strings.Join(lines, "\n")
}

// pickerWindow keeps the cursor inside a bounded viewport so a 512-entry
// catalog renders a fixed number of rows around the selection.
func (s modelsState) pickerWindow(total int) int {
	if total <= modelsPickerRowLimit {
		return 0
	}
	start := s.pickerCursor - modelsPickerRowLimit/2
	if start < 0 {
		return 0
	}
	if start+modelsPickerRowLimit > total {
		return total - modelsPickerRowLimit
	}
	return start
}

func (s modelsState) pickerRow(entry modelmgr.CatalogEntry, active bool) string {
	prefix := "  "
	if active {
		prefix = "▸ "
	}
	row := prefix + entry.Provider + "/" + entry.Model
	if len(entry.Variants) > 0 {
		row += "  (" + strings.Join(entry.Variants, ", ") + ")"
	}
	if active {
		return styleSelected.Render(row)
	}
	return row
}

func (s modelsState) variantView(width int) string {
	lines := []string{
		modelsTitle(),
		"",
		styleSubtitle.Render("Agente: " + s.agentName()),
		styleSubtitle.Render("Modelo: " + s.picked.Provider + "/" + s.picked.Model),
		"",
	}
	for i, variant := range s.picked.Variants {
		prefix := "  "
		if i == s.variantCursor {
			prefix = "▸ "
		}
		row := prefix + variant
		if i == s.variantCursor {
			row = styleSelected.Render(row)
		}
		lines = append(lines, truncate(row, width))
	}
	lines = append(lines, "", styleDim.Render(truncate(modelsVariantHints, width)))
	return strings.Join(lines, "\n")
}

func modelsInputLine(label, value string, active bool) string {
	cursor := ""
	if active {
		cursor = "▌"
	}
	return fmt.Sprintf("%-8s %s%s", label+":", value, cursor)
}

func (s modelsState) previewView(width int) string {
	lines := []string{modelsTitle(), "", styleSubtitle.Render(modelsDryRunLabel), ""}
	if s.busy || s.preview == nil {
		lines = append(lines, styleDim.Render(styles.SpinnerChar(0)+" Calculando la vista previa…"))
	} else {
		lines = append(lines, modelsReceiptLines(s.preview)...)
	}
	lines = append(lines, "", styleDim.Render(truncate(modelsPreviewHints, width)))
	return strings.Join(lines, "\n")
}

func (s modelsState) receiptView(width int) string {
	lines := []string{modelsTitle(), "", styleSubtitle.Render("Resultado"), ""}
	switch {
	case s.notice != "":
		lines = append(lines, styleConflict.Render("✖ "+s.notice))
	case s.receipt != nil:
		lines = append(lines, modelsReceiptLines(s.receipt)...)
	default:
		lines = append(lines, styleDim.Render("Sin cambios registrados."))
	}
	lines = append(lines, "", styleDim.Render(truncate(modelsReceiptHints, width)))
	return strings.Join(lines, "\n")
}

func modelsReceiptLines(receipt *install.ModelReceipt) []string {
	lines := []string{
		fmt.Sprintf("Agente:   %s", receipt.Agent),
		fmt.Sprintf("Acción:   %s", receipt.Action),
		fmt.Sprintf("Anterior: %s", modelsOrDash(receipt.Previous)),
		fmt.Sprintf("Valor:    %s", modelsOrDash(receipt.Value)),
	}
	if receipt.ConfigPath != "" {
		lines = append(lines, "Config:   "+receipt.ConfigPath)
	}
	if receipt.BackupID != "" {
		lines = append(lines, "Backup:   "+receipt.BackupID)
	}
	lines = append(lines, fmt.Sprintf("Cambiado: %s · Gestionado: %s", modelsYesNo(receipt.Changed), modelsYesNo(receipt.Managed)))
	for _, warning := range receipt.Warnings {
		lines = append(lines, styleWarn.Render("⚠ "+warning))
	}
	return lines
}

func modelsOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func modelsYesNo(value bool) string {
	if value {
		return "sí"
	}
	return "no"
}
