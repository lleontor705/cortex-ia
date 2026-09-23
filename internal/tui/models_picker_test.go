package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
)

func modelsFixtureCatalog(source string) modelmgr.Catalog {
	return modelmgr.Catalog{
		Source: source,
		Entries: []modelmgr.CatalogEntry{
			{Provider: "anthropic", Model: "claude-sonnet-4-5", Variants: []string{"high", "low"}},
			{Provider: "openai", Model: "gpt-5.2"},
		},
	}
}

func stubModelsCatalog(t *testing.T, catalog modelmgr.Catalog, calls *int) {
	t.Helper()
	previous := modelsLoadCatalog
	modelsLoadCatalog = func(string) modelmgr.Catalog {
		if calls != nil {
			*calls++
		}
		return catalog
	}
	t.Cleanup(func() { modelsLoadCatalog = previous })
}

func settleModelsCatalog(t *testing.T, state modelsState, cmd tea.Cmd) modelsState {
	t.Helper()
	if cmd == nil {
		t.Fatal("entering edit must dispatch a catalog fetch")
	}
	msg, ok := cmd().(modelsLoadedMsg)
	if !ok || msg.catalog == nil {
		t.Fatalf("catalog command message = %#v", cmd())
	}
	return state.onLoaded(msg)
}

func openPicker(t *testing.T, catalog modelmgr.Catalog) modelsState {
	t.Helper()
	stubModelsCatalog(t, catalog, nil)
	s := loadedModels(t)
	s, _, cmd := s.update(key("enter"))
	return settleModelsCatalog(t, s, cmd)
}

func TestModelsCatalogPickerComposesVariantAndCaches(t *testing.T) {
	var calls int
	stubModelsCatalog(t, modelsFixtureCatalog(modelmgr.CatalogSourceDaemon), &calls)
	stubModelsSet(t, func(desired modelmgr.Desired, _ install.ModelOptions) (*install.ModelReceipt, error) {
		return &install.ModelReceipt{Agent: desired.Agent, Action: "set", Value: desired.Compact()}, nil
	})

	s := loadedModels(t)
	s, _, cmd := s.update(key("enter"))
	if !s.catalogLoading {
		t.Fatal("the first edit must show the catalog loading state")
	}
	if out := s.view(120); !strings.Contains(out, "Cargando catálogo") {
		t.Fatalf("in-flight view must reuse the spinner: %q", out)
	}
	s = settleModelsCatalog(t, s, cmd)
	if s.step != modelsStepEntries {
		t.Fatalf("a usable catalog must open the picker, step = %v", s.step)
	}

	s = typeModelsText(s, "SONNET")
	if out := s.view(120); !strings.Contains(out, "anthropic/claude-sonnet-4-5") || strings.Contains(out, "gpt-5.2") {
		t.Fatalf("type-to-filter must narrow case-insensitively: %q", out)
	}
	s, _, _ = s.update(key("enter"))
	if s.step != modelsStepVariants || !strings.Contains(s.view(120), "low") {
		t.Fatalf("an entry with variants must open variant selection, step = %v", s.step)
	}
	s, _, _ = s.update(key("down"))
	s, _, cmd = s.update(key("enter"))
	msg := cmd().(modelsMutatedMsg)
	if !msg.preview || msg.receipt == nil || msg.receipt.Value != "anthropic/claude-sonnet-4-5#low" {
		t.Fatalf("composed dry-run preview = %+v", msg)
	}
	s = s.onMutated(msg)

	s, _, _ = s.update(key("esc"))
	s, _, _ = s.update(key("esc"))
	s, _, cmd = s.update(key("enter"))
	if cmd != nil || s.step != modelsStepEntries {
		t.Fatalf("re-entering edit must reuse the cache: cmd = %v step = %v", cmd, s.step)
	}
	if calls != 1 {
		t.Fatalf("catalog fetches = %d, want exactly 1 per session", calls)
	}
}

func TestModelsCatalogFallbackDisclosesAbsenceAndTruncation(t *testing.T) {
	s := openPicker(t, modelmgr.Catalog{Source: modelmgr.CatalogSourceNone})
	if s.step != modelsStepFreeForm || !strings.Contains(s.view(120), "Sin catálogo") {
		t.Fatalf("Source none must open free-form with a notice, step = %v view = %q", s.step, s.view(120))
	}

	truncated := modelsFixtureCatalog(modelmgr.CatalogSourceOpencode2)
	truncated.Truncated = true
	s = openPicker(t, truncated)
	if s.step != modelsStepFreeForm || !strings.Contains(s.view(120), "truncado") {
		t.Fatalf("a truncated catalog must disclose and open free-form, step = %v view = %q", s.step, s.view(120))
	}
}

func TestModelsPickerOptOutReachesFreeForm(t *testing.T) {
	s := openPicker(t, modelsFixtureCatalog(modelmgr.CatalogSourceDaemon))
	s, _, _ = s.update(key("esc"))
	if s.step != modelsStepFreeForm {
		t.Fatalf("esc must always reach free-form, step = %v", s.step)
	}
	s = typeModelsText(s, "acme/custom-model#x")
	if !strings.HasSuffix(s.input, "acme/custom-model#x") {
		t.Fatalf("free-form input = %q", s.input)
	}
}

func TestModelsPickerEntryWithoutVariantsComposesDefault(t *testing.T) {
	s := openPicker(t, modelsFixtureCatalog(modelmgr.CatalogSourceOpencode2))
	var captured modelmgr.Desired
	stubModelsSet(t, func(desired modelmgr.Desired, _ install.ModelOptions) (*install.ModelReceipt, error) {
		captured = desired
		return &install.ModelReceipt{Agent: desired.Agent, Action: "set", Value: desired.Compact()}, nil
	})

	s = typeModelsText(s, "gpt")
	s, _, cmd := s.update(key("enter"))
	if s.phase != modelsPhasePreview || cmd == nil {
		t.Fatalf("a variant-less entry must go straight to preview: phase = %v cmd = %v", s.phase, cmd)
	}
	s.onMutated(cmd().(modelsMutatedMsg))
	if captured.Variant != "" || captured.Compact() != "openai/gpt-5.2" {
		t.Fatalf("composed default reference = %q", captured.Compact())
	}
}
