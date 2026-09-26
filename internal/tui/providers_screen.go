package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

const (
	providersDefaultWidth = 80
	providersIDCol        = 16

	providersListHints     = "↑/↓ proveedor · enter token · r recargar · esc volver · q salir"
	providersInputHints    = "enter previsualizar · esc cancelar"
	providersPreviewHints  = "enter confirmar · esc volver al token"
	providersReceiptHints  = "enter volver a la lista · q salir"
	providersDegradedHints = "esc volver a Home · q salir"
	providersDryRunLabel   = "Vista previa (dry-run, sin escribir)"
	providersMaskLabel     = "Token (enmascarado)"
	providersRunningLabel  = "Instalando provider"
	providersEmptyToken    = "Introduce un token antes de continuar."
)

// providersRunPhases is the provider transaction order. The Running display
// lists them in this order and the Receipt records them as executed.
var providersRunPhases = []string{"Backup", "Update config", "Commit state"}

// providersPhase is the screen-local step of one provider install. The view
// renders the phase, and no other screen shares these fields.
type providersPhase int

const (
	providersPhaseList providersPhase = iota
	providersPhaseInput
	providersPhasePreview
	providersPhaseReceipt
	providersPhaseError
)

// providersAction is the screen-level navigation a key press requests; the
// wiring layer interprets it because state cannot change the active screen.
type providersAction int

const (
	providersActionNone providersAction = iota
	providersActionHome
	providersActionQuit
)

// The provider seams wrap install.New(homeDir) exactly like the models-screen
// seams, so the screen drives the transactional service and never writes
// configuration itself. Tests replace them to run over temporary homes; the
// token is a transient argument and the receipt never carries it.
var (
	providersListCatalog = func(homeDir string) (*install.ProviderCatalogReport, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ProviderCatalog()
	}
	providersPreview = func(homeDir, providerID, token string) (*install.ProviderInstallReceipt, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ProviderPreview(install.ProviderInstallOptions{ProviderID: providerID, Token: token})
	}
	providersInstall = func(homeDir, providerID, token string) (*install.ProviderInstallReceipt, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ProviderInstall(install.ProviderInstallOptions{ProviderID: providerID, Token: token})
	}
)

// providersCatalogMsg carries the catalog listing result.
type providersCatalogMsg struct {
	report *install.ProviderCatalogReport
	err    error
}

// providersResultMsg carries one preview or install receipt. preview marks the
// inert dry-run so the state never mistakes it for a committed install.
type providersResultMsg struct {
	receipt *install.ProviderInstallReceipt
	err     error
	preview bool
}

// providersState keeps the custom-provider screen self-contained so it renders
// and reacts to keys without sharing fields with the rest of the model.
type providersState struct {
	homeDir string
	loading bool
	err     error
	report  *install.ProviderCatalogReport
	cursor  int

	phase providersPhase
	// token holds the masked secret in screen memory only. It is read once, as
	// the service call argument, and is never rendered.
	token maskedInput
	// busy is true while a preview or install call is in flight; installing
	// distinguishes the mutating Running display from the inert dry-run.
	busy       bool
	installing bool
	preview    *install.ProviderInstallReceipt
	receipt    *install.ProviderInstallReceipt
	notice     string
}

func newProvidersState(homeDir string) providersState {
	return providersState{homeDir: homeDir, loading: true, phase: providersPhaseList}
}

func providersLoadCmd(homeDir string) tea.Cmd {
	return func() tea.Msg {
		report, err := providersListCatalog(homeDir)
		return providersCatalogMsg{report: report, err: err}
	}
}

func providersPreviewCmd(homeDir, providerID, token string) tea.Cmd {
	return func() tea.Msg {
		receipt, err := providersPreview(homeDir, providerID, token)
		return providersResultMsg{receipt: receipt, err: err, preview: true}
	}
}

func providersInstallCmd(homeDir, providerID, token string) tea.Cmd {
	return func() tea.Msg {
		receipt, err := providersInstall(homeDir, providerID, token)
		return providersResultMsg{receipt: receipt, err: err}
	}
}

func (s providersState) onLoaded(msg providersCatalogMsg) providersState {
	s.loading = false
	s.err = msg.err
	s.report = msg.report
	if msg.err != nil {
		s.phase = providersPhaseError
		return s
	}
	if s.cursor >= len(s.providers()) {
		s.cursor = 0
	}
	return s
}

func (s providersState) onResult(msg providersResultMsg) providersState {
	s.busy = false
	s.installing = false
	if msg.err != nil {
		s.preview, s.receipt = nil, nil
		s.notice = msg.err.Error()
		s.phase = providersPhaseReceipt
		return s
	}
	if msg.preview {
		s.preview = msg.receipt
		return s
	}
	s.preview = nil
	s.receipt = msg.receipt
	s.notice = ""
	s.phase = providersPhaseReceipt
	return s
}

func (s providersState) providers() []install.ProviderCatalogEntry {
	if s.report == nil {
		return nil
	}
	return s.report.Providers
}

func (s providersState) selected() (install.ProviderCatalogEntry, bool) {
	entries := s.providers()
	if s.cursor < 0 || s.cursor >= len(entries) {
		return install.ProviderCatalogEntry{}, false
	}
	return entries[s.cursor], true
}

func (s providersState) providerID() string {
	if entry, ok := s.selected(); ok {
		return entry.ID
	}
	return ""
}

func (s providersState) update(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return s, providersActionQuit, nil
	}
	switch s.phase {
	case providersPhaseError:
		return s.updateError(msg)
	case providersPhaseInput:
		return s.updateInput(msg)
	case providersPhasePreview:
		return s.updatePreview(msg)
	case providersPhaseReceipt:
		return s.updateReceipt(msg)
	default:
		return s.updateList(msg)
	}
}

// updateError is the degrade surface: a failed catalog load offers only Escape
// back to Home, so the Input phase is never reachable with an empty catalog.
func (s providersState) updateError(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, providersActionQuit, nil
	case "esc":
		return s, providersActionHome, nil
	}
	return s, providersActionNone, nil
}

func (s providersState) updateList(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, providersActionQuit, nil
	case "esc":
		return s, providersActionHome, nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.providers())-1 {
			s.cursor++
		}
	case "r":
		s.loading = true
		s.err = nil
		return s, providersActionNone, providersLoadCmd(s.homeDir)
	case "enter":
		if _, ok := s.selected(); ok {
			s.phase = providersPhaseInput
			s.token = maskedInput{}
			s.notice = ""
			s.preview, s.receipt = nil, nil
		}
	}
	return s, providersActionNone, nil
}

func (s providersState) updateInput(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		s.phase = providersPhaseList
		s.notice = ""
		return s, providersActionNone, nil
	case tea.KeyBackspace:
		s.token.backspace()
		return s, providersActionNone, nil
	case tea.KeyEnter:
		if _, ok := s.selected(); !ok {
			return s, providersActionNone, nil
		}
		if s.token.empty() {
			s.notice = providersEmptyToken
			return s, providersActionNone, nil
		}
		s.phase = providersPhasePreview
		s.busy = true
		s.preview = nil
		s.notice = ""
		return s, providersActionNone, providersPreviewCmd(s.homeDir, s.providerID(), s.token.value())
	}
	if text := providersTypedText(msg); text != "" {
		s.token.insert(text)
	}
	return s, providersActionNone, nil
}

func (s providersState) updatePreview(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, providersActionQuit, nil
	case "esc":
		if s.busy {
			return s, providersActionNone, nil
		}
		s.preview = nil
		s.phase = providersPhaseInput
		return s, providersActionNone, nil
	case "enter":
		if s.busy {
			return s, providersActionNone, nil
		}
		s.busy = true
		s.installing = true
		return s, providersActionNone, providersInstallCmd(s.homeDir, s.providerID(), s.token.value())
	}
	return s, providersActionNone, nil
}

func (s providersState) updateReceipt(msg tea.KeyMsg) (providersState, providersAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, providersActionQuit, nil
	case "enter", "esc":
		s.notice = ""
		s.receipt, s.preview = nil, nil
		s.loading = true
		s.err = nil
		s.phase = providersPhaseList
		return s, providersActionNone, providersLoadCmd(s.homeDir)
	}
	return s, providersActionNone, nil
}

// providersTypedText returns the literal text a key press contributes, or ""
// for keys that carry no rune.
func providersTypedText(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes {
		return string(msg.Runes)
	}
	if msg.Type == tea.KeySpace {
		return " "
	}
	return ""
}

func (s providersState) view(width int) string {
	w := width
	if w <= 0 {
		w = providersDefaultWidth
	}
	if s.phase == providersPhaseError {
		return s.errorView(w)
	}
	if s.loading && s.report == nil {
		return strings.Join([]string{
			providersTitle(),
			"",
			styleDim.Render(styles.SpinnerChar(0) + " Cargando proveedores…"),
			"",
			styleDim.Render(truncate(providersDegradedHints, w)),
		}, "\n")
	}
	switch s.phase {
	case providersPhaseInput:
		return s.inputView(w)
	case providersPhasePreview:
		return s.previewView(w)
	case providersPhaseReceipt:
		return s.receiptView(w)
	default:
		return s.listView(w)
	}
}

func providersTitle() string { return styleSubtitle.Render("Instalar proveedor personalizado") }

func (s providersState) errorView(width int) string {
	return strings.Join([]string{
		providersTitle(),
		"",
		styleFail.Render("No se pudo leer el catálogo de proveedores: " + s.err.Error()),
		"",
		styleDim.Render(truncate(providersDegradedHints, width)),
	}, "\n")
}

func (s providersState) listView(width int) string {
	lines := []string{providersTitle(), ""}
	entries := s.providers()
	if len(entries) == 0 {
		lines = append(lines, styleDim.Render("Sin proveedores en el catálogo."))
	} else {
		for i, entry := range entries {
			lines = append(lines, s.providerBlock(entry, i == s.cursor)...)
		}
	}
	lines = append(lines, "", styleDim.Render(truncate(providersListHints, width)))
	return strings.Join(lines, "\n")
}

// providerBlock renders one provider row followed by its runtime package keys,
// API endpoint, and every catalog model with its limits, modalities and
// effort/variant vocabulary, so the List phase discloses what an install writes.
func (s providersState) providerBlock(entry install.ProviderCatalogEntry, active bool) []string {
	prefix := "  "
	if active {
		prefix = "▸ "
	}
	header := prefix + fmt.Sprintf("%-*s  %s", providersIDCol, entry.ID, entry.Name)
	if active {
		header = styleSelected.Render(header)
	}
	return append([]string{header}, providersEntryLines(entry)...)
}

// providersEntryLines renders the catalog facts of one provider: runtime
// package keys, API endpoint, and every declared model. Everything is read from
// the loaded catalog JSON, so editing the file changes the Installer with no
// code change.
func providersEntryLines(entry install.ProviderCatalogEntry) []string {
	lines := make([]string, 0, len(entry.Models)+3)
	if entry.NPM != "" {
		lines = append(lines, "    npm:      "+entry.NPM)
	}
	if entry.Package != "" {
		lines = append(lines, "    package:  "+entry.Package)
	}
	if entry.BaseURL != "" {
		lines = append(lines, "    baseURL:  "+entry.BaseURL)
	}
	for _, model := range entry.Models {
		lines = append(lines, "    "+providersModelLine(model))
	}
	return lines
}

// providersModelLine names every catalog-owned model fact: id, display name,
// numeric limits, modalities, and the effort/variant vocabulary. A catalog
// variant's id is its reasoning effort, so one vocabulary discloses both.
func providersModelLine(model install.ProviderCatalogModel) string {
	facts := make([]string, 0, 3)
	if model.Limit != nil {
		facts = append(facts, fmt.Sprintf("limit %d/%d", model.Limit.Context, model.Limit.Output))
	}
	if model.Modalities != nil {
		facts = append(facts, fmt.Sprintf("modalities %s→%s",
			strings.Join(model.Modalities.Input, ", "), strings.Join(model.Modalities.Output, ", ")))
	}
	facts = append(facts, "efforts/variants "+providersEfforts(model))
	return model.ID + "  " + model.Name + "  ·  " + strings.Join(facts, "  ·  ")
}

// providersEfforts names the variant vocabulary the installer authors, falling
// back to the model's effort list and then to the adaptive posture so an empty
// vocabulary never renders a misleading blank.
func providersEfforts(model install.ProviderCatalogModel) string {
	vocabulary := model.Efforts
	if len(model.Variants) > 0 {
		vocabulary = make([]string, 0, len(model.Variants))
		for _, variant := range model.Variants {
			vocabulary = append(vocabulary, variant.ID)
		}
	}
	if len(vocabulary) == 0 {
		return "adaptive"
	}
	return strings.Join(vocabulary, ", ")
}

func (s providersState) inputView(width int) string {
	lines := []string{
		providersTitle(),
		"",
		styleSubtitle.Render("Proveedor: " + s.providerID()),
		"",
	}
	if s.notice != "" {
		lines = append(lines, styleWarn.Render("⚠ "+s.notice), "")
	}
	lines = append(lines,
		providersMaskLabel+": "+s.token.mask()+"▌",
		"",
		styleDim.Render(truncate(providersInputHints, width)),
	)
	return strings.Join(lines, "\n")
}

func (s providersState) previewView(width int) string {
	lines := []string{providersTitle(), "", styleSubtitle.Render(providersDryRunLabel), ""}
	hints := providersPreviewHints
	switch {
	case s.installing:
		lines = append(lines, s.runningLines()...)
		hints = providersRunningLabel
	case s.busy || s.preview == nil:
		lines = append(lines, styleDim.Render(styles.SpinnerChar(0)+" Calculando la vista previa…"))
	default:
		lines = append(lines, providersCatalogDisclosure(s.preview)...)
		lines = append(lines, providersReceiptLines(s.preview)...)
	}
	lines = append(lines, "", styleDim.Render(truncate(hints, width)))
	return strings.Join(lines, "\n")
}

// runningLines is the Running display: the transaction phases in execution
// order. The receipt records them as executed without ever naming the token.
func (s providersState) runningLines() []string {
	lines := make([]string, 0, len(providersRunPhases))
	for _, phase := range providersRunPhases {
		lines = append(lines, styleDim.Render(maskedBullet+" "+phase))
	}
	return lines
}

func (s providersState) receiptView(width int) string {
	lines := []string{providersTitle(), "", styleSubtitle.Render("Resultado"), ""}
	switch {
	case s.notice != "":
		lines = append(lines, styleConflict.Render("✖ "+s.notice))
	case s.receipt != nil:
		lines = append(lines, providersReceiptLines(s.receipt)...)
		lines = append(lines, "Fases:    "+strings.Join(providersRunPhases, " → "))
	default:
		lines = append(lines, styleDim.Render("Sin cambios registrados."))
	}
	lines = append(lines, "", styleDim.Render(truncate(providersReceiptHints, width)))
	return strings.Join(lines, "\n")
}

// providersCatalogDisclosure renders the catalog content a preview carries plus
// the resolved runtime key, so the operator confirms against exactly what the
// install will emit.
func providersCatalogDisclosure(receipt *install.ProviderInstallReceipt) []string {
	if receipt.Entry == nil {
		return nil
	}
	lines := []string{styleSubtitle.Render("Catálogo")}
	if receipt.Entry.Name != "" {
		lines = append(lines, "    nombre:   "+receipt.Entry.Name)
	}
	lines = append(lines, providersEntryLines(*receipt.Entry)...)
	if receipt.RuntimeKey != "" {
		lines = append(lines,
			"    container: "+receipt.Container,
			fmt.Sprintf("    runtime:   %s %s", receipt.RuntimeMember, receipt.RuntimeKey),
		)
	}
	return append(lines, "")
}

func providersReceiptLines(receipt *install.ProviderInstallReceipt) []string {
	lines := []string{
		fmt.Sprintf("Acción:   %s", receipt.Action),
		fmt.Sprintf("Provider: %s", receipt.Provider),
	}
	if receipt.ConfigPath != "" {
		lines = append(lines, "Config:   "+receipt.ConfigPath)
	}
	lines = append(lines, fmt.Sprintf("Modelos:  %d · Variantes: %d", receipt.ModelsWritten, receipt.VariantsWritten))
	if receipt.ReconciledTwinPath != "" {
		lines = append(lines, "Twin:     "+receipt.ReconciledTwinPath+" (bloque eliminado)")
	}
	if receipt.BackupID != "" {
		lines = append(lines, "Backup:   "+receipt.BackupID)
	}
	if receipt.RollbackCmd != "" {
		lines = append(lines, "Rollback: "+receipt.RollbackCmd)
	}
	for _, detail := range receipt.Detail {
		lines = append(lines, styleDim.Render(detail))
	}
	return lines
}
