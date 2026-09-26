package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
)

// providersFlowSecret carries an uppercase marker so any raw leak is greppable.
const providersFlowSecret = "ZXQ7-SENTINEL-CP06"

type providersSeamCalls struct {
	list      int
	preview   int
	install   int
	lastToken string
}

func stubProvidersFlowSeams(t *testing.T, catalog *install.ProviderCatalogReport) *providersSeamCalls {
	t.Helper()
	calls := &providersSeamCalls{}
	prevList, prevPreview, prevInstall := providersListCatalog, providersPreview, providersInstall
	providersListCatalog = func(string) (*install.ProviderCatalogReport, error) {
		calls.list++
		return catalog, nil
	}
	providersPreview = func(_, _, token string) (*install.ProviderInstallReceipt, error) {
		calls.preview++
		calls.lastToken = token
		return providersPreviewFixture(), nil
	}
	providersInstall = func(_, _, _ string) (*install.ProviderInstallReceipt, error) {
		calls.install++
		return providersInstallFixture(), nil
	}
	t.Cleanup(func() {
		providersListCatalog, providersPreview, providersInstall = prevList, prevPreview, prevInstall
	})
	return calls
}

// newHomeFlowModel pins Home boot over an isolated CORTEX_IA_HOME and temp home.
func newHomeFlowModel(t *testing.T) model {
	t.Helper()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	m.screen, m.cursor = screenHome, 0
	return m
}

func pressCmd(m model, k string) (model, tea.Cmd) {
	updated, cmd := m.Update(key(k))
	return updated.(model), cmd
}

func assertNoRawToken(t *testing.T, phase, frame string) {
	t.Helper()
	if strings.Contains(frame, providersFlowSecret) || strings.Contains(frame, "SENTINEL") {
		t.Fatalf("%s frame rendered raw token characters:\n%s", phase, frame)
	}
}

// snapshotTree fingerprints every path size and mtime under root.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s|%d|%s", path, info.Size(), info.ModTime().UTC()))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestREQ_PROV_006_HotkeysOpenProvidersListPhase(t *testing.T) {
	stubProvidersFlowSeams(t, providersFixtureCatalog())
	for _, hotkey := range []string{"p", "P"} {
		m, cmd := pressCmd(newHomeFlowModel(t), hotkey)
		if m.screen != screenProviders || m.providers.phase != providersPhaseList {
			t.Fatalf("hotkey %q = screen %v phase %d, want providers/List", hotkey, m.screen, m.providers.phase)
		}
		if cmd == nil {
			t.Fatalf("hotkey %q must dispatch the catalog load", hotkey)
		}
		m = drive(t, m, cmd)
		if m.providers.report == nil || !strings.Contains(m.View(), "nan") {
			t.Fatalf("hotkey %q did not render the loaded catalog:\n%s", hotkey, m.View())
		}
	}
}

func TestREQ_PROV_006_NumericDigitsStayFrozen(t *testing.T) {
	stubProvidersFlowSeams(t, providersFixtureCatalog())
	for digit, index := range map[string]int{"1": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6, "8": 7, "9": 8} {
		if got := press(newHomeFlowModel(t), digit).cursor; got != index {
			t.Fatalf("key %s left cursor at %d, want frozen index %d", digit, got, index)
		}
	}
	nine := press(newHomeFlowModel(t), "9")
	if nine.screen != screenModels || nine.providers.report != nil {
		t.Fatalf("key 9 = screen %v providers report %v, want the frozen models screen", nine.screen, nine.providers.report != nil)
	}
	zero := press(newHomeFlowModel(t), "0")
	if zero.screen != screenHome || zero.cursor != 0 || zero.quitting {
		t.Fatalf("key 0 = screen %v cursor %d quitting %v, want an inert Home", zero.screen, zero.cursor, zero.quitting)
	}
}

func TestREQ_PROV_006_ModelsHotkeyStillOpensModels(t *testing.T) {
	for _, hotkey := range []string{"m", "M"} {
		if m := press(newHomeFlowModel(t), hotkey); m.screen != screenModels {
			t.Fatalf("hotkey %q = screen %v, want the models screen", hotkey, m.screen)
		}
	}
}

func TestREQ_PROV_006_EntryNineCursorEnterNavigableAndDescribed(t *testing.T) {
	if len(homeEntries) != len(homeDescriptions) {
		t.Fatalf("homeEntries %d, homeDescriptions %d: three-place registration drifted", len(homeEntries), len(homeDescriptions))
	}
	seen := make(map[string]bool, len(homeDescriptions))
	for i, desc := range homeDescriptions {
		if strings.TrimSpace(desc) == "" || seen[desc] {
			t.Fatalf("homeDescriptions[%d] blank or duplicated: %q", i, desc)
		}
		seen[desc] = true
	}
	if homeEntries[providersEntryIndex] != "Install custom provider" {
		t.Fatalf("homeEntries[%d] = %q", providersEntryIndex, homeEntries[providersEntryIndex])
	}
	stubProvidersFlowSeams(t, providersFixtureCatalog())
	m := newHomeFlowModel(t)
	for i := 0; i < providersEntryIndex; i++ {
		m = press(m, "down")
	}
	if m.cursor != providersEntryIndex {
		t.Fatalf("cursor = %d, want %d", m.cursor, providersEntryIndex)
	}
	frame := m.View()
	if !strings.Contains(frame, "Install custom provider") || !strings.Contains(frame, homeDescriptions[providersEntryIndex]) {
		t.Fatalf("Home must render entry 9 with its description:\n%s", frame)
	}
	m, cmd := pressCmd(m, "enter")
	if m.screen != screenProviders || m.providers.phase != providersPhaseList || cmd == nil {
		t.Fatalf("enter on entry 9 = screen %v phase %d cmd %v, want providers/List/command", m.screen, m.providers.phase, cmd != nil)
	}
}

func TestREQ_PROV_006_ProviderMessagesRouteOnlyOnProvidersScreen(t *testing.T) {
	m := newHomeFlowModel(t)
	if updated, _ := m.Update(providersCatalogMsg{report: providersFixtureCatalog()}); updated.(model).providers.report != nil {
		t.Fatal("catalog message mutated the model while Home was active")
	}
	if updated, _ := m.Update(providersResultMsg{receipt: providersInstallFixture()}); updated.(model).providers.phase != providersPhaseList {
		t.Fatal("result message mutated the model while Home was active")
	}
	m.screen = screenProviders
	m.providers = newProvidersState(m.homeDir)
	if updated, _ := m.Update(providersCatalogMsg{report: providersFixtureCatalog()}); updated.(model).providers.report == nil {
		t.Fatal("catalog message dropped on the providers screen")
	}
	if updated, _ := m.Update(providersResultMsg{receipt: providersInstallFixture()}); updated.(model).providers.phase != providersPhaseReceipt {
		t.Fatal("result message dropped on the providers screen")
	}
}

func TestREQ_PROV_005_FullFlowThroughSeamsIsInertAtPreview(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("CORTEX_IA_HOME", stateRoot)
	calls := stubProvidersFlowSeams(t, providersFixtureCatalog())

	m := sized(newModel(&fakeService{}, t.TempDir(), "vtest"))
	m.screen, m.cursor = screenHome, 0

	m = pressDrive(t, m, "p")
	if m.screen != screenProviders || m.providers.phase != providersPhaseList || calls.list != 1 {
		t.Fatalf("p = screen %v phase %d loads %d, want providers/List/1", m.screen, m.providers.phase, calls.list)
	}
	assertNoRawToken(t, "List", m.View())

	before := snapshotTree(t, stateRoot)

	m, _ = pressCmd(m, "enter")
	if m.providers.phase != providersPhaseInput {
		t.Fatalf("enter = phase %d, want Input", m.providers.phase)
	}
	for _, r := range providersFlowSecret {
		m = press(m, string(r))
	}
	inputFrame := m.View()
	assertNoRawToken(t, "Input", inputFrame)
	if got := strings.Count(inputFrame, maskedBullet); got != len(providersFlowSecret) {
		t.Fatalf("Input bullets = %d, want %d", got, len(providersFlowSecret))
	}

	m, cmd := pressCmd(m, "enter")
	m = drive(t, m, cmd)
	if m.providers.phase != providersPhasePreview || m.providers.preview == nil {
		t.Fatalf("preview = phase %d preview %v", m.providers.phase, m.providers.preview != nil)
	}
	if calls.preview != 1 || calls.install != 0 || calls.lastToken != providersFlowSecret {
		t.Fatalf("preview seam calls = %d install = %d token forwarded = %v", calls.preview, calls.install, calls.lastToken == providersFlowSecret)
	}
	assertNoRawToken(t, "Preview", m.View())
	if after := snapshotTree(t, stateRoot); after != before {
		t.Fatalf("Preview wrote to the state root:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	m, cmd = pressCmd(m, "enter")
	if !m.providers.installing {
		t.Fatal("confirm must enter the Running display")
	}
	runningFrame := m.View()
	for _, phase := range providersRunPhases {
		if !strings.Contains(runningFrame, phase) {
			t.Fatalf("Running frame missing phase %q:\n%s", phase, runningFrame)
		}
	}
	assertNoRawToken(t, "Running", runningFrame)

	m = drive(t, m, cmd)
	if m.providers.phase != providersPhaseReceipt || m.providers.receipt == nil || calls.install != 1 {
		t.Fatalf("receipt = phase %d receipt %v installs %d", m.providers.phase, m.providers.receipt != nil, calls.install)
	}
	receiptFrame := m.View()
	assertNoRawToken(t, "Receipt", receiptFrame)
	for _, want := range []string{"installed", "Modelos:  8", "provider-abc123", "Commit state"} {
		if !strings.Contains(receiptFrame, want) {
			t.Fatalf("Receipt frame missing %q:\n%s", want, receiptFrame)
		}
	}
}
