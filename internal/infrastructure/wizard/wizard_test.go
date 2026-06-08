package wizard

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

func TestInitWizardModelAdjustsDefaults(t *testing.T) {
	model := newInitWizardModel(minimalConfig())

	model.adjustSelection(5) // adjust default min
	if model.defaultMin != 85 {
		t.Fatalf("expected default min 85, got %.0f", model.defaultMin)
	}
	if model.domains[0].min != 85 {
		t.Fatalf("expected domain min to match default, got %.0f", model.domains[0].min)
	}

	model.cursor = 1
	model.adjustSelection(5) // adjust domain min
	if !model.domains[0].override {
		t.Fatalf("expected override flag set")
	}
	if model.domains[0].min != 90 {
		t.Fatalf("expected domain min 90, got %.0f", model.domains[0].min)
	}
}

func TestInitWizardModelConfigOutput(t *testing.T) {
	model := newInitWizardModel(minimalConfig())
	model.cursor = 1
	model.adjustSelection(5) // ensure override

	cfg := model.toConfig()
	if cfg.Policy.DefaultMin != model.defaultMin {
		t.Fatalf("default min mismatch: %.0f vs %.0f", cfg.Policy.DefaultMin, model.defaultMin)
	}
	if len(cfg.Policy.Domains) != len(model.domains) {
		t.Fatalf("domain count mismatch")
	}
	if cfg.Policy.Domains[0].Min == nil {
		t.Fatalf("expected overridden min")
	}
	if *cfg.Policy.Domains[0].Min != model.domains[0].min {
		t.Fatalf("expected min %.0f, got %.0f", model.domains[0].min, *cfg.Policy.Domains[0].Min)
	}
}

func TestRunInitWizardCompletes(t *testing.T) {
	var out bytes.Buffer
	stdin := strings.NewReader("\r\r\r")
	cfg, confirmed, err := runInitWizard(minimalConfig(), &out, stdin)
	if err != nil {
		t.Fatalf("wizard error: %v", err)
	}
	if !confirmed {
		t.Fatalf("expected wizard to confirm")
	}
	if cfg.Policy.DefaultMin != minimalConfig().Policy.DefaultMin {
		t.Fatalf("unexpected default min")
	}
	if len(cfg.Policy.Domains) == 0 {
		t.Fatalf("expected domains preserved")
	}
}

func TestInitWizardMoveCursor(t *testing.T) {
	model := newInitWizardModel(minimalConfig())
	model.moveCursor(1)
	if model.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", model.cursor)
	}
	model.moveCursor(-5)
	if model.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", model.cursor)
	}
	model.moveCursor(len(model.domains) + 5)
	if model.cursor != len(model.domains) {
		t.Fatalf("expected cursor at max %d, got %d", len(model.domains), model.cursor)
	}
}

func TestInitWizardClamp(t *testing.T) {
	if clamp(-5, 10) != 0 {
		t.Fatalf("expected clamp to min")
	}
	if clamp(20, 10) != 10 {
		t.Fatalf("expected clamp to max")
	}
	if clamp(5, 10) != 5 {
		t.Fatalf("expected clamp to keep value")
	}
}

func TestInitWizardUpdateTransitions(t *testing.T) {
	model := newInitWizardModel(minimalConfig())
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.state != stateEdit {
		t.Fatalf("expected edit state, got %d", model.state)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.state != stateConfirm {
		t.Fatalf("expected confirm state, got %d", model.state)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.state != stateEdit {
		t.Fatalf("expected edit state on esc, got %d", model.state)
	}
}

func TestInitWizardViewConfirmShowsExcludes(t *testing.T) {
	model := newInitWizardModel(minimalConfig())
	model.state = stateConfirm
	model.exclude = []string{"internal/generated/*"}
	view := model.View()
	if !strings.Contains(view, "Configured exclusions") {
		t.Fatalf("expected exclusion text in view")
	}
}

func minimalConfig() application.Config {
	return application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: 80,
			Domains:    []domain.Domain{{Name: "module", Match: []string{"./..."}}},
		},
	}
}
