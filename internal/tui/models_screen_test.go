package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
)

const modelsTestHome = "C:/synthetic/home"

func modelsFixtureReport() *install.ModelListReport {
	return &install.ModelListReport{
		ConfigPath: "C:/synthetic/opencode.jsonc",
		Installed:  true,
		Agents: []modelmgr.AgentEntry{
			{Agent: "build", Model: "anthropic/claude-sonnet-4-5", Variant: "high", Source: modelmgr.SourceManaged, Managed: true},
			{Agent: "explore", Model: "openai/gpt-5.2", Source: modelmgr.SourceConfig},
			{Agent: "plan", Source: modelmgr.SourceUnset},
			{Agent: "reviewer", Source: modelmgr.SourceMarkdown, MarkdownPin: "openai/gpt-5.2"},
		},
	}
}

func loadedModels(t *testing.T) modelsState {
	t.Helper()
	return newModelsState(modelsTestHome).onLoaded(modelsLoadedMsg{report: modelsFixtureReport()})
}

func stubModelsLoad(t *testing.T, report *install.ModelListReport, seenHome *string) {
	t.Helper()
	previous := modelsLoadReport
	modelsLoadReport = func(homeDir string) (*install.ModelListReport, error) {
		if seenHome != nil {
			*seenHome = homeDir
		}
		return report, nil
	}
	t.Cleanup(func() { modelsLoadReport = previous })
}

func stubModelsSet(t *testing.T, fn func(modelmgr.Desired, install.ModelOptions) (*install.ModelReceipt, error)) {
	t.Helper()
	previous := modelsSetRef
	modelsSetRef = func(_ string, desired modelmgr.Desired, opts install.ModelOptions) (*install.ModelReceipt, error) {
		return fn(desired, opts)
	}
	t.Cleanup(func() { modelsSetRef = previous })
}

func typeModelsText(state modelsState, text string) modelsState {
	for _, r := range text {
		state, _, _ = state.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return state
}

func writeModelsConfig(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("seed config dir: %v", err)
	}
	body := "{\n  // user comment\n  \"agents\": {\"plan\": {\"model\": \"anthropic/claude-sonnet-4-5#high\"}}\n}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return path
}

func TestModelsScreenListsAgentsSourcesAndUnset(t *testing.T) {
	out := loadedModels(t).view(120)
	for _, want := range []string{
		"Configuración de modelos", "config: C:/synthetic/opencode.jsonc",
		"build", "anthropic/claude-sonnet-4-5#high", "managed",
		"explore", "openai/gpt-5.2", "config",
		"plan", "unset",
		"reviewer", "markdown",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("list view missing %q in:\n%s", want, out)
		}
	}
}

func TestModelsLoadCmdUsesInjectedSeam(t *testing.T) {
	var seen string
	stubModelsLoad(t, modelsFixtureReport(), &seen)
	msg, ok := modelsLoadCmd(modelsTestHome)().(modelsLoadedMsg)
	if !ok || msg.report == nil || msg.err != nil {
		t.Fatalf("load message = %+v, want an injected report", msg)
	}
	if seen != modelsTestHome {
		t.Fatalf("loader home = %q, want %q", seen, modelsTestHome)
	}
}

func TestModelsNavigationActionsAndCursorBounds(t *testing.T) {
	s := loadedModels(t)
	s, action, _ := s.update(key("down"))
	if s.cursor != 1 || action != modelsActionNone {
		t.Fatalf("down: cursor = %d action = %v", s.cursor, action)
	}
	for i := 0; i < 10; i++ {
		s, _, _ = s.update(key("down"))
	}
	if s.cursor != len(modelsFixtureReport().Agents)-1 {
		t.Fatalf("cursor below agents: %d", s.cursor)
	}
	if _, action, _ := s.update(key("esc")); action != modelsActionHome {
		t.Fatalf("esc action = %v, want home", action)
	}
	if _, action, _ := s.update(key("q")); action != modelsActionQuit {
		t.Fatalf("q action = %v, want quit", action)
	}
	if _, action, _ := s.update(tea.KeyMsg{Type: tea.KeyCtrlC}); action != modelsActionQuit {
		t.Fatalf("ctrl+c action = %v, want quit", action)
	}
	if _, action, cmd := s.update(key("r")); action != modelsActionNone || cmd == nil {
		t.Fatalf("r must request a reload: action = %v cmd = %v", action, cmd)
	}
}

func TestModelsPreviewThenConfirmPersistsThroughServiceSeam(t *testing.T) {
	var calls []install.ModelOptions
	setAgent := ""
	stubModelsSet(t, func(desired modelmgr.Desired, opts install.ModelOptions) (*install.ModelReceipt, error) {
		calls = append(calls, opts)
		setAgent = desired.Agent
		return &install.ModelReceipt{
			Agent: desired.Agent, Action: "set", Previous: "", Value: desired.Compact(),
			ConfigPath: "C:/synthetic/opencode.jsonc", BackupID: "bkp-9", Changed: true, Managed: true,
		}, nil
	})

	stubModelsCatalog(t, modelmgr.Catalog{Source: modelmgr.CatalogSourceNone}, nil)

	s := loadedModels(t)
	s, _, _ = s.update(key("down"))
	s, _, _ = s.update(key("down")) // plan: an unset builtin
	s, _, catalogCmd := s.update(key("enter"))
	if s.phase != modelsPhaseInput {
		t.Fatalf("enter on a row must open the editor, phase = %v", s.phase)
	}
	s = settleModelsCatalog(t, s, catalogCmd)
	s = typeModelsText(s, "openai/gpt-5.2")
	s, _, _ = s.update(tea.KeyMsg{Type: tea.KeyTab})
	s = typeModelsText(s, "high")

	s, _, cmd := s.update(key("enter"))
	if cmd == nil || len(calls) != 0 {
		t.Fatalf("editor enter must dispatch the dry-run preview, calls = %d cmd = %v", len(calls), cmd)
	}
	s = s.onMutated(cmd().(modelsMutatedMsg))
	if len(calls) != 1 || !calls[0].DryRun {
		t.Fatalf("preview calls = %+v, want one dry-run", calls)
	}
	if out := s.view(120); !strings.Contains(out, "Vista previa") || !strings.Contains(out, "openai/gpt-5.2#high") {
		t.Fatalf("preview view = %q", out)
	}

	s, _, cmd = s.update(key("enter"))
	if cmd == nil {
		t.Fatal("confirm must dispatch the persistence command")
	}
	s = s.onMutated(cmd().(modelsMutatedMsg))
	if len(calls) != 2 || calls[1].DryRun {
		t.Fatalf("confirm calls = %+v, want a real (non dry-run) persistence", calls)
	}
	if setAgent != "plan" {
		t.Fatalf("persisted agent = %q, want plan", setAgent)
	}
	out := s.view(120)
	if !strings.Contains(out, "Resultado") || !strings.Contains(out, "bkp-9") || !strings.Contains(out, "Gestionado: sí") {
		t.Fatalf("receipt view = %q", out)
	}
}

func TestModelsTypedConflictRendersInScreen(t *testing.T) {
	stubModelsSet(t, func(desired modelmgr.Desired, _ install.ModelOptions) (*install.ModelReceipt, error) {
		return nil, &modelmgr.ConflictError{
			Agent: desired.Agent, Kind: modelmgr.ConflictUnknownAgent,
			Sources: []string{"builtins", "config agents", "markdown"},
		}
	})

	stubModelsCatalog(t, modelmgr.Catalog{Source: modelmgr.CatalogSourceNone}, nil)

	s := loadedModels(t)
	s, _, catalogCmd := s.update(key("enter"))
	s = settleModelsCatalog(t, s, catalogCmd)
	s = typeModelsText(s, "ghost/nowhere")
	s, _, cmd := s.update(key("enter"))
	s = s.onMutated(cmd().(modelsMutatedMsg))

	out := s.view(120)
	if !strings.Contains(out, "Resultado") || !strings.Contains(out, "not a known agent") || !strings.Contains(out, "builtins") {
		t.Fatalf("conflict view = %q", out)
	}
	s, _, cmd = s.update(key("esc"))
	if s.phase != modelsPhaseList || cmd == nil {
		t.Fatalf("the screen must stay usable after a conflict: phase = %v cmd = %v", s.phase, cmd)
	}
}

// TestModelsDefaultLoaderReadsTempHomeThroughService proves the default seam
// reaches the registry through the install service read path and never writes.
func TestModelsDefaultLoaderReadsTempHomeThroughService(t *testing.T) {
	home := t.TempDir()
	path := writeModelsConfig(t, home)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded config: %v", err)
	}

	report, err := modelsLoadReport(home)
	if err != nil {
		t.Fatalf("listing must work on a home without v2 metadata: %v", err)
	}
	out := newModelsState(home).onLoaded(modelsLoadedMsg{report: report}).view(120)
	for _, want := range []string{"plan", "anthropic/claude-sonnet-4-5#high", "build", "unset"} {
		if !strings.Contains(out, want) {
			t.Errorf("temp-home view missing %q in:\n%s", want, out)
		}
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(original) {
		t.Fatalf("listing rewrote the config: err = %v", err)
	}
}

// TestModelsDefaultPersistenceFailsClosedWithoutInstallation proves persistence
// flows through the service: an unaccredited home is rejected without a write.
func TestModelsDefaultPersistenceFailsClosedWithoutInstallation(t *testing.T) {
	home := t.TempDir()
	path := writeModelsConfig(t, home)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded config: %v", err)
	}

	desired := modelmgr.Desired{Agent: "plan", Provider: "openai", Model: "gpt-5.2"}
	if _, err := modelsSetRef(home, desired, install.ModelOptions{}); !errors.Is(err, install.ErrNotInstalled) {
		t.Fatalf("persistence without an installation = %v, want ErrNotInstalled", err)
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(original) {
		t.Fatalf("a failed persistence must not touch the config: err = %v", err)
	}
	s := newModelsState(home).onLoaded(modelsLoadedMsg{report: modelsFixtureReport()}).onMutated(modelsMutatedMsg{err: install.ErrNotInstalled})
	if out := s.view(120); !strings.Contains(out, "Resultado") || !strings.Contains(out, "run install first") {
		t.Fatalf("failure view = %q", out)
	}
}

// TestModelsScreenReachableFromHomeAndReturns covers the model-level wiring:
// the screen is reachable, loads through the injected seam, and returns Home.
func TestModelsScreenReachableFromHomeAndReturns(t *testing.T) {
	var seen string
	stubModelsLoad(t, modelsFixtureReport(), &seen)
	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))

	m = pressDrive(t, m, "m")
	if m.screen != screenModels || seen == "" {
		t.Fatalf("'m' must open the models screen and load through the seam: screen = %v home = %q", m.screen, seen)
	}
	if out := m.models.view(m.contentWidth()); !strings.Contains(out, "Configuración de modelos") || !strings.Contains(out, "explore") {
		t.Fatalf("models view = %q", out)
	}
	if m = press(m, "esc"); m.screen != screenHome {
		t.Fatalf("esc must return Home, got %v", m.screen)
	}
}
