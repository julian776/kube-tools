package graph

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/julian776/kube-tools/pkg/kube"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TimeRange represents a selectable time window.
type TimeRange struct {
	Label    string
	Duration string // for future Prometheus integration
}

var timeRanges = []TimeRange{
	{Label: "1 Hour", Duration: "1h"},
	{Label: "4 Hours", Duration: "4h"},
	{Label: "1 Day", Duration: "1d"},
	{Label: "Today", Duration: "today"},
}

// MetricsFetcher is a function that fetches metrics for a given time range.
type MetricsFetcher func(tr TimeRange) ([]kube.ResourceMetrics, error)

// Model is the bubbletea model for the interactive graph view.
type Model struct {
	kind       string
	name       string
	activeTab  int
	fetcher    MetricsFetcher
	metrics    []kube.ResourceMetrics
	err        error
}

// NewModel creates a new TUI model.
func NewModel(kind, name string, fetcher MetricsFetcher) Model {
	return Model{
		kind:      kind,
		name:      name,
		activeTab: 0,
		fetcher:   fetcher,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchMetrics
}

func (m Model) fetchMetrics() tea.Msg {
	metrics, err := m.fetcher(timeRanges[m.activeTab])
	if err != nil {
		return errMsg{err}
	}
	return metricsMsg{metrics}
}

type metricsMsg struct{ metrics []kube.ResourceMetrics }
type errMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(timeRanges)
			return m, m.fetchMetrics
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(timeRanges)) % len(timeRanges)
			return m, m.fetchMetrics
		}
	case metricsMsg:
		m.metrics = msg.metrics
		m.err = nil
	case errMsg:
		m.err = msg.err
		m.metrics = nil
	}
	return m, nil
}

var (
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("240"))

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("39"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

func (m Model) View() string {
	var b strings.Builder

	// Render tabs
	var tabs []string
	for i, tr := range timeRanges {
		if i == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(tr.Label))
		} else {
			tabs = append(tabs, tabStyle.Render(tr.Label))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	b.WriteString("\n\n")

	// Render content
	if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", m.err))
	} else if m.metrics != nil {
		var buf bytes.Buffer
		RenderResourceUsage(&buf, m.kind, m.name, m.metrics)
		b.WriteString(buf.String())
	} else {
		b.WriteString("  Loading...\n")
	}

	b.WriteString(helpStyle.Render("  ←/→ switch tab • q quit"))
	b.WriteString("\n")

	return b.String()
}

// RunInteractive starts the interactive TUI.
func RunInteractive(kind, name string, fetcher MetricsFetcher) error {
	p := tea.NewProgram(NewModel(kind, name, fetcher))
	_, err := p.Run()
	return err
}
