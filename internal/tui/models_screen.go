package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
)

const (
	modelsDefaultWidth = 80
	modelsAgentCol     = 14
	modelsRefCol       = 34

	modelsListHints     = "↑/↓ agente · enter editar · u quitar · r recargar · esc volver · q salir"
	modelsInputHints    = "enter previsualizar · tab cambia campo · esc cancelar"
	modelsPreviewHints  = "enter confirmar · esc volver al editor"
	modelsReceiptHints  = "enter volver a la lista · q salir"
	modelsDegradedHints = "esc volver a Home · q salir"
	modelsDryRunLabel   = "Vista previa (dry-run, sin escribir)"

	modelsCatalogLoadHints = "esc cancelar · q salir"
	modelsPickerHints      = "escribe para filtrar · ↑/↓ mover · enter elegir · esc entrada libre"
	modelsVariantHints     = "↑/↓ effort · enter confirmar · esc entrada libre · ← volver"
	// Disclosure copy for the free-form fallback. The catalog is an aid, never
	// a gate: an absent or truncated catalog must not block the manual path.
	modelsNoCatalogNotice = "Sin catálogo de modelos disponible; escribe la referencia manualmente."
	modelsTruncatedNotice = "Catálogo truncado; escribe la referencia manualmente."
	modelsPickerRowLimit  = 10
)

// modelsStep is the Input-phase sub-step. The outer phase machine stays
// List→Input→Preview→Receipt; the picker is the acquisition stage of Input.
type modelsStep int

const (
	modelsStepFreeForm modelsStep = iota
	modelsStepEntries
	modelsStepVariants
)

// modelsPhase is the screen-local step of one configuration intent. The view
// renders the phase, and no other screen shares these fields.
type modelsPhase int

const (
	modelsPhaseList modelsPhase = iota
	modelsPhaseInput
	modelsPhasePreview
	modelsPhaseReceipt
)

// modelsField selects which free-form input receives typed runes.
type modelsField int

const (
	modelsFieldModel modelsField = iota
	modelsFieldEffort
)

// modelsAction is the screen-level navigation a key press requests. The state
// cannot change the model's active screen, so the wiring layer interprets it.
type modelsAction int

const (
	modelsActionNone modelsAction = iota
	modelsActionHome
	modelsActionQuit
)

// The model seams wrap the install service the CLI uses, so the screen loads
// and persists exclusively through the transactional service path and never
// writes configuration itself. Tests replace them to run over temporary homes.
var (
	modelsLoadReport = func(homeDir string) (*install.ModelListReport, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ModelList()
	}
	modelsSetRef = func(homeDir string, desired modelmgr.Desired, opts install.ModelOptions) (*install.ModelReceipt, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ModelSet(desired, opts)
	}
	modelsUnsetRef = func(homeDir, agent string, opts install.ModelOptions) (*install.ModelReceipt, error) {
		service, err := install.New(homeDir)
		if err != nil {
			return nil, err
		}
		return service.ModelUnset(agent, opts)
	}
	// modelsLoadCatalog fetches the selectable catalog behind the same
	// injectable seam discipline as modelsLoadReport. Acquisition never fails
	// closed: exhausting every tier yields Catalog{Source: none}, so the screen
	// always has a usable result. Tests replace it so no daemon, network, or
	// opencode2 binary is ever required.
	modelsLoadCatalog = func(homeDir string) modelmgr.Catalog {
		return modelmgr.New(homeDir).Catalog(modelmgr.CatalogOptions{
			RunCommand: func(name string, args ...string) ([]byte, error) {
				return exec.Command(name, args...).CombinedOutput()
			},
		})
	}
)

// modelsState keeps the models configuration screen self-contained so it
// renders and reacts to keys without sharing fields with the rest of the model.
type modelsState struct {
	homeDir string
	report  *install.ModelListReport
	err     error
	loading bool
	cursor  int
	phase   modelsPhase
	focus   modelsField
	input   string
	effort  string
	desired modelmgr.Desired
	// pendingUnset records that the previewed intent is a removal, so confirm
	// dispatches the unset mutation instead of a set.
	pendingUnset bool
	// busy is true while a dry-run preview or a mutation is in flight.
	busy    bool
	preview *install.ModelReceipt
	receipt *install.ModelReceipt
	notice  string
	// Catalog acquisition state. The cache is load-once per screen session:
	// catalogFetched flips after the first fetch, successful or not.
	catalog        *modelmgr.Catalog
	catalogFetched bool
	catalogLoading bool
	step           modelsStep
	filter         string
	pickerCursor   int
	picked         modelmgr.CatalogEntry
	variantCursor  int
	inputNotice    string
}

func newModelsState(homeDir string) modelsState {
	return modelsState{homeDir: homeDir, loading: true, phase: modelsPhaseList}
}

// modelsLoadedMsg carries either the agent registry report or a catalog
// acquisition result. The model-level wiring routes this message type into the
// screen and is frozen for this change, so the two asynchronous loads share it
// instead of adding a second top-level message case.
type modelsLoadedMsg struct {
	report  *install.ModelListReport
	err     error
	catalog *modelmgr.Catalog
}

type modelsMutatedMsg struct {
	receipt *install.ModelReceipt
	err     error
	preview bool
}

func modelsLoadCmd(homeDir string) tea.Cmd {
	return func() tea.Msg {
		report, err := modelsLoadReport(homeDir)
		return modelsLoadedMsg{report: report, err: err}
	}
}

func modelsLoadCatalogCmd(homeDir string) tea.Cmd {
	return func() tea.Msg {
		catalog := modelsLoadCatalog(homeDir)
		return modelsLoadedMsg{catalog: &catalog}
	}
}

func modelsPreviewCmd(homeDir string, desired modelmgr.Desired) tea.Cmd {
	return func() tea.Msg {
		receipt, err := modelsSetRef(homeDir, desired, install.ModelOptions{DryRun: true})
		return modelsMutatedMsg{receipt: receipt, err: err, preview: true}
	}
}

func modelsPersistCmd(homeDir string, desired modelmgr.Desired) tea.Cmd {
	return func() tea.Msg {
		receipt, err := modelsSetRef(homeDir, desired, install.ModelOptions{})
		return modelsMutatedMsg{receipt: receipt, err: err}
	}
}

func modelsUnsetCmd(homeDir, agent string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		receipt, err := modelsUnsetRef(homeDir, agent, install.ModelOptions{DryRun: dryRun})
		return modelsMutatedMsg{receipt: receipt, err: err, preview: dryRun}
	}
}

func (s modelsState) onLoaded(msg modelsLoadedMsg) modelsState {
	if msg.catalog != nil {
		return s.onCatalogLoaded(*msg.catalog)
	}
	s.loading = false
	s.err = msg.err
	if msg.report != nil {
		s.report = msg.report
	}
	if s.cursor >= len(s.agents()) {
		s.cursor = 0
	}
	return s
}

// onCatalogLoaded caches the acquisition result and, when the user is still in
// the Input phase, resolves the acquisition step: the picker for a usable
// catalog, or the free-form fallback with its disclosure notice otherwise.
func (s modelsState) onCatalogLoaded(catalog modelmgr.Catalog) modelsState {
	s.catalogLoading = false
	s.catalogFetched = true
	loaded := catalog
	s.catalog = &loaded
	if s.phase != modelsPhaseInput {
		return s
	}
	if !s.pickerAvailable() {
		s.step = modelsStepFreeForm
		s.inputNotice = s.catalogNotice()
		return s
	}
	s.step = modelsStepEntries
	s.filter = ""
	s.pickerCursor = 0
	return s
}

// pickerAvailable reports whether the cached catalog can drive the picker. A
// truncated catalog falls back to free-form because the selection surface
// would be incomplete.
func (s modelsState) pickerAvailable() bool {
	return s.catalog != nil && s.catalog.Source != modelmgr.CatalogSourceNone && !s.catalog.Truncated
}

func (s modelsState) catalogNotice() string {
	if s.catalog == nil || s.catalog.Source == modelmgr.CatalogSourceNone {
		return modelsNoCatalogNotice
	}
	return modelsTruncatedNotice
}

// filteredEntries narrows the catalog by a case-insensitive substring match
// across the composed provider/model reference.
func (s modelsState) filteredEntries() []modelmgr.CatalogEntry {
	if s.catalog == nil {
		return nil
	}
	query := strings.ToLower(s.filter)
	entries := make([]modelmgr.CatalogEntry, 0, len(s.catalog.Entries))
	for _, entry := range s.catalog.Entries {
		if query != "" && !strings.Contains(strings.ToLower(entry.Provider+"/"+entry.Model), query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// entryCursor preselects the picker row matching an agent's current model so
// re-editing does not lose the effective selection.
func (s modelsState) entryCursor(model string) int {
	for i, entry := range s.filteredEntries() {
		if entry.Provider+"/"+entry.Model == model {
			return i
		}
	}
	return 0
}

func (s modelsState) onMutated(msg modelsMutatedMsg) modelsState {
	s.busy = false
	if msg.err != nil {
		s.preview, s.receipt = nil, nil
		s.pendingUnset = false
		s.notice = modelsFailure(msg.err)
		s.phase = modelsPhaseReceipt
		return s
	}
	if msg.preview {
		s.preview = msg.receipt
		return s
	}
	s.preview = nil
	s.receipt = msg.receipt
	s.pendingUnset = false
	s.notice = ""
	s.phase = modelsPhaseReceipt
	return s
}

func (s modelsState) agents() []modelmgr.AgentEntry {
	if s.report == nil {
		return nil
	}
	return s.report.Agents
}

func (s modelsState) selected() (modelmgr.AgentEntry, bool) {
	agents := s.agents()
	if s.cursor < 0 || s.cursor >= len(agents) {
		return modelmgr.AgentEntry{}, false
	}
	return agents[s.cursor], true
}

func (s modelsState) agentName() string {
	if entry, ok := s.selected(); ok {
		return entry.Agent
	}
	return s.desired.Agent
}

func (s modelsState) reload() (modelsState, tea.Cmd) {
	s.loading = true
	s.err = nil
	return s, modelsLoadCmd(s.homeDir)
}

// modelsFailure renders any mutation failure as in-screen text. A typed
// conflict keeps its agent and resolution sources, so every rejected intent
// stays actionable instead of crashing the program.
func modelsFailure(err error) string {
	var conflict *modelmgr.ConflictError
	if errors.As(err, &conflict) {
		return conflict.Error()
	}
	return err.Error()
}

func (s modelsState) update(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return s, modelsActionQuit, nil
	}
	switch s.phase {
	case modelsPhaseInput:
		return s.updateInput(msg)
	case modelsPhasePreview:
		return s.updatePreview(msg)
	case modelsPhaseReceipt:
		return s.updateReceipt(msg)
	default:
		return s.updateList(msg)
	}
}

func (s modelsState) updateList(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, modelsActionQuit, nil
	case "esc":
		return s, modelsActionHome, nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.agents())-1 {
			s.cursor++
		}
	case "r":
		updated, cmd := s.reload()
		return updated, modelsActionNone, cmd
	case "enter":
		if entry, ok := s.selected(); ok {
			updated, cmd := s.beginEdit(entry)
			return updated, modelsActionNone, cmd
		}
	case "u":
		if entry, ok := s.selected(); ok {
			updated, cmd := s.beginUnset(entry)
			return updated, modelsActionNone, cmd
		}
	}
	return s, modelsActionNone, nil
}

// beginEdit prefills the free-form fields with the effective value so editing
// an existing assignment never starts from a blank reference, then resolves the
// Input-phase acquisition step: fetch the catalog once, open the picker, or
// fall back to free-form.
func (s modelsState) beginEdit(entry modelmgr.AgentEntry) (modelsState, tea.Cmd) {
	s.phase = modelsPhaseInput
	s.focus = modelsFieldModel
	s.notice = ""
	s.preview, s.receipt = nil, nil
	s.pendingUnset = false
	s.input, s.effort = entry.Model, entry.Variant
	if s.input == "" && entry.MarkdownPin != "" {
		if desired, err := modelmgr.ParseModelRef(entry.Agent, entry.MarkdownPin); err == nil {
			s.input, s.effort = desired.Provider+"/"+desired.Model, desired.Variant
		}
	}
	s.filter, s.inputNotice = "", ""
	s.step, s.pickerCursor, s.variantCursor = modelsStepFreeForm, 0, 0
	if !s.catalogFetched {
		s.catalogLoading = true
		return s, modelsLoadCatalogCmd(s.homeDir)
	}
	if s.pickerAvailable() {
		s.step = modelsStepEntries
		s.pickerCursor = s.entryCursor(entry.Model)
		return s, nil
	}
	s.inputNotice = s.catalogNotice()
	return s, nil
}

func (s modelsState) beginUnset(entry modelmgr.AgentEntry) (modelsState, tea.Cmd) {
	if entry.Source == modelmgr.SourceUnset {
		s.notice = fmt.Sprintf("el agente %q ya está sin modelo", entry.Agent)
		s.phase = modelsPhaseReceipt
		return s, nil
	}
	s.phase = modelsPhasePreview
	s.pendingUnset = true
	s.busy = true
	s.preview, s.receipt = nil, nil
	s.notice = ""
	return s, modelsUnsetCmd(s.homeDir, entry.Agent, true)
}

func (s modelsState) updateInput(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	if s.catalogLoading {
		switch {
		case msg.Type == tea.KeyEscape:
			s.catalogLoading = false
			s.step = modelsStepFreeForm
			s.phase = modelsPhaseList
		case msg.String() == "q":
			return s, modelsActionQuit, nil
		}
		return s, modelsActionNone, nil
	}
	switch s.step {
	case modelsStepEntries:
		return s.updatePicker(msg)
	case modelsStepVariants:
		return s.updateVariants(msg)
	default:
		return s.updateFreeForm(msg)
	}
}

// modelsTypedText returns the literal text a key press contributes, or "" for
// keys that carry no rune.
func modelsTypedText(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes {
		return string(msg.Runes)
	}
	if msg.Type == tea.KeySpace {
		return " "
	}
	return ""
}

// submitInput runs the unchanged validation → dry-run preview path for the
// current free-form fields, whether they were typed or composed by the picker.
func (s modelsState) submitInput() (modelsState, modelsAction, tea.Cmd) {
	desired, err := modelmgr.ParseDesired(s.agentName(), s.input, s.effort)
	if err != nil {
		s.notice = modelsFailure(err)
		s.phase = modelsPhaseReceipt
		return s, modelsActionNone, nil
	}
	s.desired = desired
	s.phase = modelsPhasePreview
	s.busy = true
	s.preview = nil
	return s, modelsActionNone, modelsPreviewCmd(s.homeDir, desired)
}

func (s modelsState) updateFreeForm(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		s.phase = modelsPhaseList
		s.notice, s.inputNotice, s.filter = "", "", ""
		s.step = modelsStepFreeForm
		return s, modelsActionNone, nil
	case tea.KeyTab:
		if s.focus == modelsFieldModel {
			s.focus = modelsFieldEffort
		} else {
			s.focus = modelsFieldModel
		}
		return s, modelsActionNone, nil
	case tea.KeyBackspace:
		return s.trimField(), modelsActionNone, nil
	case tea.KeyEnter:
		return s.submitInput()
	}
	if text := modelsTypedText(msg); text != "" {
		return s.appendField(text), modelsActionNone, nil
	}
	return s, modelsActionNone, nil
}

func (s modelsState) updatePicker(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return s.optOutFreeForm(), modelsActionNone, nil
	case tea.KeyEnter:
		entries := s.filteredEntries()
		if s.pickerCursor < 0 || s.pickerCursor >= len(entries) {
			return s, modelsActionNone, nil
		}
		entry := entries[s.pickerCursor]
		if len(entry.Variants) == 0 {
			return s.chooseEntry(entry, "")
		}
		s.picked, s.variantCursor, s.step = entry, 0, modelsStepVariants
		return s, modelsActionNone, nil
	case tea.KeyUp:
		if s.pickerCursor > 0 {
			s.pickerCursor--
		}
		return s, modelsActionNone, nil
	case tea.KeyDown:
		if s.pickerCursor < len(s.filteredEntries())-1 {
			s.pickerCursor++
		}
		return s, modelsActionNone, nil
	case tea.KeyBackspace:
		s.filter = trimLastRune(s.filter)
		s.pickerCursor = 0
		return s, modelsActionNone, nil
	}
	if text := modelsTypedText(msg); text != "" {
		s.filter += text
		s.pickerCursor = 0
	}
	return s, modelsActionNone, nil
}

func (s modelsState) updateVariants(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return s.optOutFreeForm(), modelsActionNone, nil
	case tea.KeyBackspace, tea.KeyLeft:
		s.step = modelsStepEntries
		return s, modelsActionNone, nil
	case tea.KeyEnter:
		if s.variantCursor < 0 || s.variantCursor >= len(s.picked.Variants) {
			return s, modelsActionNone, nil
		}
		return s.chooseEntry(s.picked, s.picked.Variants[s.variantCursor])
	case tea.KeyUp:
		if s.variantCursor > 0 {
			s.variantCursor--
		}
	case tea.KeyDown:
		if s.variantCursor < len(s.picked.Variants)-1 {
			s.variantCursor++
		}
	}
	return s, modelsActionNone, nil
}

// optOutFreeForm leaves the picker for the manual editor. Opting out is always
// available, so a populated catalog can never trap the user.
func (s modelsState) optOutFreeForm() modelsState {
	s.step = modelsStepFreeForm
	s.inputNotice = ""
	return s
}

// chooseEntry composes the selected provider/model#variant into the free-form
// fields and runs the unchanged validation → dry-run preview path.
func (s modelsState) chooseEntry(entry modelmgr.CatalogEntry, variant string) (modelsState, modelsAction, tea.Cmd) {
	s.input = entry.Provider + "/" + entry.Model
	s.effort = variant
	s.step = modelsStepFreeForm
	s.inputNotice = ""
	return s.submitInput()
}

func (s modelsState) updatePreview(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, modelsActionQuit, nil
	case "esc":
		if s.busy {
			return s, modelsActionNone, nil
		}
		s.pendingUnset = false
		s.preview = nil
		if s.desired.Agent == "" {
			s.phase = modelsPhaseList
			return s, modelsActionNone, nil
		}
		s.phase = modelsPhaseInput
	case "enter":
		if s.busy {
			return s, modelsActionNone, nil
		}
		s.busy = true
		if s.pendingUnset {
			return s, modelsActionNone, modelsUnsetCmd(s.homeDir, s.agentName(), false)
		}
		return s, modelsActionNone, modelsPersistCmd(s.homeDir, s.desired)
	}
	return s, modelsActionNone, nil
}

func (s modelsState) updateReceipt(msg tea.KeyMsg) (modelsState, modelsAction, tea.Cmd) {
	switch msg.String() {
	case "q":
		return s, modelsActionQuit, nil
	case "enter", "esc":
		s.notice = ""
		s.receipt, s.preview = nil, nil
		s.phase = modelsPhaseList
		updated, cmd := s.reload()
		return updated, modelsActionNone, cmd
	}
	return s, modelsActionNone, nil
}

func (s modelsState) appendField(text string) modelsState {
	if s.focus == modelsFieldModel {
		s.input += text
		return s
	}
	s.effort += text
	return s
}

func (s modelsState) trimField() modelsState {
	if s.focus == modelsFieldModel {
		s.input = trimLastRune(s.input)
		return s
	}
	s.effort = trimLastRune(s.effort)
	return s
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}
