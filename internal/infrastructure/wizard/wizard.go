package wizard

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.klarlabs.de/coverctl/internal/application"
	"go.klarlabs.de/coverctl/internal/domain"
)

type (
	wizardState int

	initWizardModel struct {
		state      wizardState
		defaultMin float64
		domains    []wizardDomain
		cursor     int
		confirmed  bool
		aborted    bool
		exclude    []string
	}

	wizardDomain struct {
		domain   domain.Domain
		min      float64
		override bool
	}
)

const (
	stateIntro wizardState = iota
	stateEdit
	stateConfirm
)

var (
	colorOrange = lipgloss.Color("#F97316")
	colorSky    = lipgloss.Color("#0EA5E9")
	colorLime   = lipgloss.Color("#84CC16")
	colorSlate  = lipgloss.Color("#64748B")
	colorInk    = lipgloss.Color("#0F172A")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
	badgeStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorInk).Background(colorSky).Padding(0, 1)
	subtleStyle  = lipgloss.NewStyle().Foreground(colorSlate)
	keyStyle     = lipgloss.NewStyle().Foreground(colorInk).Background(lipgloss.Color("#E2E8F0")).Padding(0, 1)
	activeStep   = lipgloss.NewStyle().Bold(true).Foreground(colorInk).Background(colorLime).Padding(0, 1)
	inactiveStep = lipgloss.NewStyle().Foreground(colorSlate).Padding(0, 1).Border(lipgloss.RoundedBorder(), false, false, true, false).BorderForeground(colorSlate)
	selectedRow  = lipgloss.NewStyle().Bold(true).Foreground(colorInk).Background(lipgloss.Color("#E0F2FE")).Padding(0, 1)
	domainStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorSky)
	valueStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
)

func Run(cfg application.Config, stdout io.Writer, stdin io.Reader) (application.Config, bool, error) {
	return runInitWizard(cfg, stdout, stdin)
}

func runInitWizard(cfg application.Config, stdout io.Writer, stdin io.Reader) (application.Config, bool, error) {
	model := newInitWizardModel(cfg)
	program := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(stdout))
	res, err := program.Run()
	if err != nil {
		return cfg, false, err
	}
	finalModel, ok := res.(*initWizardModel)
	if !ok {
		return cfg, false, fmt.Errorf("unexpected wizard state")
	}
	if finalModel.aborted || !finalModel.confirmed {
		return cfg, false, nil
	}
	return finalModel.toConfig(), true, nil
}

func newInitWizardModel(cfg application.Config) *initWizardModel {
	defaultMin := cfg.Policy.DefaultMin
	if defaultMin <= 0 {
		defaultMin = 80
	}
	domains := make([]wizardDomain, len(cfg.Policy.Domains))
	for i, d := range cfg.Policy.Domains {
		minVal := defaultMin
		override := false
		if d.Min != nil {
			minVal = *d.Min
			override = true
		}
		domains[i] = wizardDomain{
			domain:   d,
			min:      minVal,
			override: override,
		}
	}
	if len(domains) == 0 {
		domains = append(domains, wizardDomain{domain: domain.Domain{Name: "module", Match: []string{"./..."}, Min: nil}, min: defaultMin})
	}
	return &initWizardModel{
		state:      stateIntro,
		defaultMin: defaultMin,
		domains:    domains,
		cursor:     0,
		exclude:    append([]string(nil), cfg.Exclude...),
	}
}

func (m *initWizardModel) Init() tea.Cmd {
	return nil
}

func (m *initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "q":
		m.aborted = true
		return m, tea.Quit
	case "enter":
		switch m.state {
		case stateIntro:
			m.state = stateEdit
		case stateEdit:
			m.state = stateConfirm
		case stateConfirm:
			m.confirmed = true
			return m, tea.Quit
		}
	case "esc":
		if m.state == stateConfirm {
			m.state = stateEdit
		}
	case "up":
		if m.state == stateEdit {
			m.moveCursor(-1)
		}
	case "down":
		if m.state == stateEdit {
			m.moveCursor(1)
		}
	case "left", "-":
		if m.state == stateEdit {
			m.adjustSelection(-5)
		}
	case "right", "+":
		if m.state == stateEdit {
			m.adjustSelection(5)
		}
	}
	return m, nil
}

func (m *initWizardModel) View() string {
	switch m.state {
	case stateIntro:
		return m.viewIntro()
	case stateEdit:
		return m.viewEdit()
	case stateConfirm:
		return m.viewConfirm()
	default:
		return ""
	}
}

func (m *initWizardModel) moveCursor(delta int) {
	max := len(m.domains)
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > max {
		m.cursor = max
	}
}

func (m *initWizardModel) adjustSelection(delta float64) {
	if m.cursor == 0 {
		m.adjustDefault(delta)
		return
	}
	m.adjustDomain(m.cursor-1, delta)
}

func (m *initWizardModel) adjustDefault(delta float64) {
	m.defaultMin = clamp(m.defaultMin+delta, 100)
	for i := range m.domains {
		if !m.domains[i].override {
			m.domains[i].min = m.defaultMin
		}
	}
}

func (m *initWizardModel) adjustDomain(index int, delta float64) {
	if index < 0 || index >= len(m.domains) {
		return
	}
	value := clamp(m.domains[index].min+delta, 100)
	m.domains[index].min = value
	if !m.domains[index].override {
		m.domains[index].override = true
	}
}

func (m *initWizardModel) viewIntro() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s %s\n", titleStyle.Render("coverctl init"), badgeStyle.Render("wizard"))
	fmt.Fprintf(&b, "%s\n\n", subtleStyle.Render("Domain-aware coverage policy setup"))
	fmt.Fprintf(&b, "%s\n\n", m.stepper())
	fmt.Fprintf(&b, "coverctl detected %d domains. The wizard helps you review coverage thresholds.\n\n", len(m.domains))
	fmt.Fprintf(&b, "Default coverage is %s. Press %s to continue or %s to cancel.\n",
		valueStyle.Render(fmt.Sprintf("%.0f%%", m.defaultMin)),
		keyStyle.Render("Enter"),
		keyStyle.Render("Ctrl+C"),
	)
	return b.String()
}

func (m *initWizardModel) viewEdit() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s %s\n", titleStyle.Render("Review thresholds"), badgeStyle.Render("step 2/3"))
	fmt.Fprintf(&b, "%s\n", m.stepper())
	fmt.Fprintf(&b, "%s\n\n", subtleStyle.Render("Use ↑/↓ to move, ←/→ or +/- to adjust."))
	fmt.Fprintf(&b, "%s\n", subtleStyle.Render("Default min (applies to non-customized domains):"))
	defaultLine := fmt.Sprintf("Default min: %s", valueStyle.Render(fmt.Sprintf("%.0f%%", m.defaultMin)))
	if m.cursor == 0 {
		fmt.Fprintf(&b, "%s\n\n", selectedRow.Render(defaultLine))
	} else {
		fmt.Fprintf(&b, "  %s\n\n", defaultLine)
	}
	fmt.Fprintf(&b, "%s\n", subtleStyle.Render("Domains:"))
	for idx, dom := range m.domains {
		custom := ""
		if dom.override {
			custom = " (custom)"
		}
		row := fmt.Sprintf("%s %s%s", domainStyle.Render(dom.domain.Name), valueStyle.Render(fmt.Sprintf("%.0f%%", dom.min)), custom)
		if m.cursor == idx+1 {
			fmt.Fprintf(&b, "%s\n", selectedRow.Render(row))
		} else {
			fmt.Fprintf(&b, "  %s\n", row)
		}
	}
	fmt.Fprintf(&b, "\nPress %s to continue, %s to cancel.\n", keyStyle.Render("Enter"), keyStyle.Render("q"))
	return b.String()
}

func (m *initWizardModel) viewConfirm() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s %s\n", titleStyle.Render("Confirm policy"), badgeStyle.Render("step 3/3"))
	fmt.Fprintf(&b, "%s\n\n", m.stepper())
	fmt.Fprintf(&b, "Default min coverage: %s\n", valueStyle.Render(fmt.Sprintf("%.0f%%", m.defaultMin)))
	fmt.Fprintf(&b, "%s\n", subtleStyle.Render("Domains summary:"))
	for _, dom := range m.domains {
		fmt.Fprintf(&b, "  %s %s\n", domainStyle.Render(dom.domain.Name), valueStyle.Render(fmt.Sprintf("%.0f%%", dom.min)))
	}
	if len(m.exclude) > 0 {
		fmt.Fprintf(&b, "\n%s\n", subtleStyle.Render("Configured exclusions:"))
		for _, pattern := range m.exclude {
			fmt.Fprintf(&b, "  - %s\n", pattern)
		}
	} else {
		fmt.Fprintf(&b, "\n%s\n", subtleStyle.Render("No exclusions configured."))
	}
	fmt.Fprintf(&b, "\nPress %s to save, %s to go back, %s to cancel.\n",
		keyStyle.Render("Enter"),
		keyStyle.Render("Esc"),
		keyStyle.Render("q"),
	)
	return b.String()
}

func (m *initWizardModel) stepper() string {
	return strings.Join([]string{
		m.stepLabel("Intro", m.state == stateIntro),
		m.stepLabel("Review", m.state == stateEdit),
		m.stepLabel("Confirm", m.state == stateConfirm),
	}, " ")
}

func (m *initWizardModel) stepLabel(label string, active bool) string {
	if active {
		return activeStep.Render(label)
	}
	return inactiveStep.Render(label)
}

func (m *initWizardModel) toConfig() application.Config {
	cfg := application.Config{
		Version: 1,
		Policy: domain.Policy{
			DefaultMin: m.defaultMin,
			Domains:    make([]domain.Domain, len(m.domains)),
		},
		Exclude: append([]string(nil), m.exclude...),
	}
	for i, dom := range m.domains {
		d := dom.domain
		d.Min = nil
		if dom.override {
			min := dom.min
			d.Min = &min
		}
		cfg.Policy.Domains[i] = d
	}
	return cfg
}

// clamp constrains value to the range [0, max].
func clamp(value, max float64) float64 {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}
