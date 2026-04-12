package screens

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

func TestRenderPersona_ShowsAllPersonas(t *testing.T) {
	data := PersonaData{
		Personas: []model.PersonaID{
			model.PersonaProfessional,
			model.PersonaMentor,
			model.PersonaMinimal,
		},
		Cursor: 0,
	}
	output := RenderPersona(data)
	for _, p := range data.Personas {
		if !strings.Contains(output, string(p)) {
			t.Errorf("expected persona %q in output", p)
		}
	}
}

func TestRenderPersona_ShowsDescriptions(t *testing.T) {
	data := PersonaData{
		Personas: []model.PersonaID{
			model.PersonaProfessional,
			model.PersonaMentor,
			model.PersonaMinimal,
		},
		Cursor: 0,
	}
	output := RenderPersona(data)
	descriptions := []string{
		"Direct, concise, technical terminology",
		"Teaching-oriented",
		"Code only",
	}
	for _, desc := range descriptions {
		if !strings.Contains(output, desc) {
			t.Errorf("expected description containing %q in output", desc)
		}
	}
}
