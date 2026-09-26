package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
)

const providersTestHome = "C:/synthetic/providers-home"

func providersFixtureCatalog() *install.ProviderCatalogReport {
	return &install.ProviderCatalogReport{Providers: []install.ProviderCatalogEntry{{
		ID:   "nan",
		Name: "Nan",
		Models: []install.ProviderCatalogModel{
			{ID: "glm5.3", Name: "GLM 5.3", Efforts: []string{"low", "medium", "high", "max"}},
			{ID: "glm5.3-flash", Name: "GLM 5.3 Flash", Efforts: []string{"low", "medium", "high", "max"}},
			{ID: "qwen3.6", Name: "Qwen 3.6", Efforts: []string{"none", "minimal", "low", "medium", "high", "max"}},
			{ID: "gemma4", Name: "Gemma 4", Efforts: []string{"none", "minimal", "low", "medium", "high", "max"}},
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash"},
			{ID: "qwen3.8-flash", Name: "Qwen 3.8 Flash"},
			{ID: "mimo-v2.5", Name: "MiMo v2.5"},
			{ID: "mimo-v2.6-flash", Name: "MiMo v2.6 Flash"},
		},
	}}}
}

func providersPreviewFixture() *install.ProviderInstallReceipt {
	return &install.ProviderInstallReceipt{
		Action:             "installed",
		Provider:           "nan",
		ConfigPath:         "C:/synthetic/opencode.jsonc",
		ReconciledTwinPath: "C:/synthetic/opencode.json",
		ModelsWritten:      8,
		VariantsWritten:    20,
		DryRun:             true,
		Detail: []string{
			"backup first: capture one verified snapshot before any edit",
			"twin cleanup: remove the stale provider.nan block from non-winning opencode.json",
			"winner write: materialize provider.nan into opencode.jsonc",
		},
	}
}

func providersInstallFixture() *install.ProviderInstallReceipt {
	receipt := providersPreviewFixture()
	receipt.DryRun = false
	receipt.BackupID = "provider-abc123"
	receipt.RollbackCmd = "cortex-ia rollback provider-abc123"
	return receipt
}

func loadedProviders(t *testing.T) providersState {
	t.Helper()
	return newProvidersState(providersTestHome).onLoaded(providersCatalogMsg{report: providersFixtureCatalog()})
}

func stubProvidersPreview(t *testing.T, fn func(id, token string) (*install.ProviderInstallReceipt, error)) {
	t.Helper()
	previous := providersPreview
	providersPreview = func(_, id, token string) (*install.ProviderInstallReceipt, error) {
		return fn(id, token)
	}
	t.Cleanup(func() { providersPreview = previous })
}

func stubProvidersInstall(t *testing.T, fn func(id, token string) (*install.ProviderInstallReceipt, error)) {
	t.Helper()
	previous := providersInstall
	providersInstall = func(_, id, token string) (*install.ProviderInstallReceipt, error) {
		return fn(id, token)
	}
	t.Cleanup(func() { providersInstall = previous })
}

func enterProvidersToken(s providersState, token string) providersState {
	s, _, _ = s.update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range token {
		s, _, _ = s.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return s
}

func TestREQ_PROV_003_TokenMaskRendersBulletsAndNeverRaw(t *testing.T) {
	const secret = "sentinel-token-abc"
	s := enterProvidersToken(loadedProviders(t), secret)
	if s.phase != providersPhaseInput {
		t.Fatalf("phase = %d, want Input", s.phase)
	}
	if s.token.value() != secret {
		t.Fatalf("raw buffer = %q, want the typed token", s.token.value())
	}
	frame := s.view(140)
	if got := strings.Count(frame, maskedBullet); got != len(secret) {
		t.Fatalf("bullet count = %d, want %d", got, len(secret))
	}
	if strings.Contains(frame, secret) || strings.Contains(frame, "sentinel") {
		t.Fatalf("rendered frame leaked raw token characters:\n%s", frame)
	}

	// A fresh Input renders a labelled masked field even before any rune.
	empty := loadedProviders(t)
	empty, _, _ = empty.update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(empty.view(140), providersMaskLabel) {
		t.Fatalf("empty Input must render its labelled masked field")
	}

	s, _, _ = s.update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := strings.Count(s.view(140), maskedBullet); got != len(secret)-1 {
		t.Fatalf("bullet count after backspace = %d, want %d", got, len(secret)-1)
	}

	// Enter with an empty buffer is refused: Input never advances to a preview
	// without a token.
	guard := loadedProviders(t)
	guard, _, _ = guard.update(tea.KeyMsg{Type: tea.KeyEnter})
	guard, _, guardCmd := guard.update(tea.KeyMsg{Type: tea.KeyEnter})
	if guard.phase != providersPhaseInput || guardCmd != nil || guard.notice == "" {
		t.Fatalf("empty token advanced to phase %d (cmd=%v, notice=%q)", guard.phase, guardCmd, guard.notice)
	}
}

func TestREQ_PROV_005_ListRendersCatalogModelsAndEfforts(t *testing.T) {
	frame := loadedProviders(t).view(160)
	for _, want := range []string{
		"nan", "glm5.3", "glm5.3-flash", "qwen3.6", "gemma4", "deepseek-v4-flash",
		"qwen3.8-flash", "mimo-v2.5", "mimo-v2.6-flash",
		"low, medium, high, max", "none, minimal, low, medium, high, max", "adaptive",
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("list frame missing %q:\n%s", want, frame)
		}
	}
}

func TestREQ_PROV_005_PreviewIsInertAndEscapeRewindsToInput(t *testing.T) {
	const secret = "sentinel-preview"
	var installCalls int
	stubProvidersPreview(t, func(id, token string) (*install.ProviderInstallReceipt, error) {
		if id != "nan" || token != secret {
			t.Fatalf("preview seam received id=%q with an altered token", id)
		}
		return providersPreviewFixture(), nil
	})
	stubProvidersInstall(t, func(string, string) (*install.ProviderInstallReceipt, error) {
		installCalls++
		return nil, nil
	})

	s := enterProvidersToken(loadedProviders(t), secret)
	s, _, previewCmd := s.update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.phase != providersPhasePreview || !s.busy || previewCmd == nil {
		t.Fatalf("preview = phase %d busy=%v cmd=%v", s.phase, s.busy, previewCmd)
	}
	s = s.onResult(previewCmd().(providersResultMsg))
	if s.busy || s.preview == nil {
		t.Fatalf("preview result not recorded: busy=%v preview=%v", s.busy, s.preview)
	}
	frame := s.view(160)
	for _, want := range []string{providersDryRunLabel, "nan", "opencode.jsonc", "twin cleanup", "opencode.json", "backup first"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("preview frame missing %q:\n%s", want, frame)
		}
	}
	if installCalls != 0 {
		t.Fatalf("preview issued %d install calls, want 0", installCalls)
	}

	s, action, _ := s.update(tea.KeyMsg{Type: tea.KeyEscape})
	if s.phase != providersPhaseInput || action != providersActionNone {
		t.Fatalf("escape from preview = phase %d action %d, want Input/None", s.phase, action)
	}
	if s.token.value() != secret {
		t.Fatalf("escape must keep the buffered token")
	}
	if installCalls != 0 {
		t.Fatalf("escape issued %d install calls, want 0", installCalls)
	}
}

func TestREQ_PROV_005_ConfirmRunsPhasesAndReceiptHidesToken(t *testing.T) {
	const secret = "sentinel-confirm"
	stubProvidersPreview(t, func(string, string) (*install.ProviderInstallReceipt, error) {
		return providersPreviewFixture(), nil
	})
	stubProvidersInstall(t, func(id, token string) (*install.ProviderInstallReceipt, error) {
		if id != "nan" || token != secret {
			t.Fatalf("install seam received id=%q with an altered token", id)
		}
		return providersInstallFixture(), nil
	})

	s := enterProvidersToken(loadedProviders(t), secret)
	s, _, previewCmd := s.update(tea.KeyMsg{Type: tea.KeyEnter})
	s = s.onResult(previewCmd().(providersResultMsg))

	s, _, installCmd := s.update(tea.KeyMsg{Type: tea.KeyEnter})
	if !s.installing || s.phase != providersPhasePreview {
		t.Fatalf("confirm did not enter Running: installing=%v phase=%d", s.installing, s.phase)
	}
	running := s.view(160)
	for _, phase := range providersRunPhases {
		if !strings.Contains(running, phase) {
			t.Fatalf("running frame missing phase %q:\n%s", phase, running)
		}
	}
	if installCmd == nil {
		t.Fatalf("confirm produced no install command")
	}

	s = s.onResult(installCmd().(providersResultMsg))
	if s.phase != providersPhaseReceipt || s.receipt == nil || s.installing {
		t.Fatalf("install did not land on Receipt: phase=%d installing=%v", s.phase, s.installing)
	}
	frame := s.view(160)
	for _, want := range []string{"installed", "opencode.jsonc", "Modelos:  8", "provider-abc123", "Fases:", "Commit state"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("receipt frame missing %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, secret) || strings.Contains(frame, "sentinel") {
		t.Fatalf("receipt leaked raw token characters:\n%s", frame)
	}
}

func TestREQ_PROV_005_CatalogErrorOffersOnlyEscapeHome(t *testing.T) {
	s := newProvidersState(providersTestHome).onLoaded(providersCatalogMsg{err: errors.New("catálogo inválido")})
	if s.phase != providersPhaseError {
		t.Fatalf("phase = %d, want Error", s.phase)
	}
	frame := s.view(160)
	if !strings.Contains(frame, "catálogo inválido") || strings.Contains(frame, providersMaskLabel) {
		t.Fatalf("error frame must disclose the failure and offer no token input:\n%s", frame)
	}

	s, action, _ := s.update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.phase != providersPhaseError || action != providersActionNone {
		t.Fatalf("enter from Error = phase %d action %d, want Error/None", s.phase, action)
	}
	_, action, _ = s.update(tea.KeyMsg{Type: tea.KeyEscape})
	if action != providersActionHome {
		t.Fatalf("escape from Error = action %d, want the only offered Home action", action)
	}
}
