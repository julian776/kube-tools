package cmd

import (
	"fmt"
	"strings"

	"github.com/julian776/kube-tools/pkg/config"
	"github.com/julian776/kube-tools/pkg/kube"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure kube-tools for the current cluster",
	Long:  "Interactively discover and configure Prometheus for the current kube context.",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	client, err := kube.NewClient(kubeContext)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	ctxName, err := client.CurrentContext(kubeContext)
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	fmt.Printf("Setting up kube-tools for context: %s\n\n", ctxName)
	fmt.Println("Discovering Prometheus services in the cluster...")

	candidates, err := client.DiscoverPrometheus()
	if err != nil {
		return fmt.Errorf("failed to discover prometheus: %w", err)
	}

	// Build choices: discovered services + manual URL option
	var choices []setupChoice
	for _, c := range candidates {
		choices = append(choices, setupChoice{
			label:     fmt.Sprintf("  %s", c.Display()),
			candidate: &c,
		})
	}
	choices = append(choices, setupChoice{
		label:  "  Enter a Prometheus URL manually",
		manual: true,
	})

	if len(candidates) == 0 {
		fmt.Println("No Prometheus services found in the cluster.")
	} else {
		fmt.Printf("Found %d Prometheus service(s):\n\n", len(candidates))
	}

	// Run interactive picker
	m := newSetupModel(choices, ctxName)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("interactive setup failed: %w", err)
	}

	final := result.(setupModel)
	if final.cancelled {
		fmt.Println("Setup cancelled.")
		return nil
	}

	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if final.manualURL != "" {
		cfg.SetPrometheus(ctxName, config.PrometheusRef{
			URL: final.manualURL,
		})
	} else if final.selected != nil {
		cfg.SetPrometheus(ctxName, config.PrometheusRef{
			ServiceName: final.selected.ServiceName,
			Namespace:   final.selected.Namespace,
			Port:        final.selected.Port,
		})
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	cfgPath, _ := config.Path()
	fmt.Printf("\nConfiguration saved to %s\n", cfgPath)
	fmt.Println("kube-tools will now use this Prometheus automatically for graph commands.")
	return nil
}

// --- Bubbletea model for interactive setup ---

type setupChoice struct {
	label     string
	candidate *kube.PrometheusCandidate
	manual    bool
}

type setupModel struct {
	choices    []setupChoice
	cursor     int
	ctxName    string
	selected   *kube.PrometheusCandidate
	manualURL  string
	inputMode  bool
	inputValue string
	cancelled  bool
	done       bool
}

func newSetupModel(choices []setupChoice, ctxName string) setupModel {
	return setupModel{
		choices: choices,
		ctxName: ctxName,
	}
}

func (m setupModel) Init() tea.Cmd {
	return nil
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inputMode {
			return m.updateInput(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m setupModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.choices)-1 {
			m.cursor++
		}
	case "enter":
		choice := m.choices[m.cursor]
		if choice.manual {
			m.inputMode = true
			return m, nil
		}
		m.selected = choice.candidate
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m setupModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.inputMode = false
		m.inputValue = ""
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.inputValue)
		if url != "" {
			m.manualURL = url
			m.done = true
			return m, tea.Quit
		}
	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.inputValue += msg.String()
		}
	}
	return m, nil
}

var (
	setupTitleStyle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	setupSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	setupDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func (m setupModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	if m.inputMode {
		b.WriteString(setupTitleStyle.Render("Enter Prometheus URL:"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  > %s█\n", m.inputValue))
		b.WriteString("\n")
		b.WriteString(setupDimStyle.Render("  enter confirm • esc back"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(setupTitleStyle.Render("Select Prometheus for context: " + m.ctxName))
	b.WriteString("\n\n")

	for i, choice := range m.choices {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
			b.WriteString(setupSelectedStyle.Render(cursor + choice.label))
		} else {
			b.WriteString(cursor + choice.label)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(setupDimStyle.Render("  ↑/↓ navigate • enter select • q quit"))
	b.WriteString("\n")

	return b.String()
}
