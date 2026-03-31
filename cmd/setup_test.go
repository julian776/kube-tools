package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/julian776/kube-tools/pkg/kube"
)

func TestSetupCommand_Exists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "setup" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'setup' subcommand to be registered")
	}
}

func TestSetupModel_InitialState(t *testing.T) {
	choices := []setupChoice{
		{label: "  monitoring/prometheus (port 9090)", candidate: &kube.PrometheusCandidate{ServiceName: "prometheus", Namespace: "monitoring", Port: 9090}},
		{label: "  Install Prometheus", install: true},
		{label: "  Enter a Prometheus URL manually", manual: true},
	}

	m := newSetupModel(choices, "my-cluster")

	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
	if m.done {
		t.Error("expected done to be false")
	}
	if m.cancelled {
		t.Error("expected cancelled to be false")
	}
	if m.inputMode {
		t.Error("expected inputMode to be false")
	}
	if m.ctxName != "my-cluster" {
		t.Errorf("expected ctxName 'my-cluster', got %q", m.ctxName)
	}
}

func TestSetupModel_NavigateAndSelectService(t *testing.T) {
	candidate := &kube.PrometheusCandidate{ServiceName: "prometheus", Namespace: "monitoring", Port: 9090}
	choices := []setupChoice{
		{label: "  monitoring/prometheus (port 9090)", candidate: candidate},
		{label: "  Install Prometheus", install: true},
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Select first item (service)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	if !model.done {
		t.Error("expected done after selecting service")
	}
	if model.selected == nil {
		t.Fatal("expected selected to be non-nil")
	}
	if model.selected.ServiceName != "prometheus" {
		t.Errorf("expected selected service 'prometheus', got %q", model.selected.ServiceName)
	}
	if model.wantInstall {
		t.Error("should not want install when selecting a service")
	}
}

func TestSetupModel_NavigateAndSelectInstall(t *testing.T) {
	choices := []setupChoice{
		{label: "  Install Prometheus", install: true},
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Select install (first item)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	if !model.done {
		t.Error("expected done after selecting install")
	}
	if !model.wantInstall {
		t.Error("expected wantInstall to be true")
	}
	if model.selected != nil {
		t.Error("expected selected to be nil for install")
	}
}

func TestSetupModel_NavigateToManualInput(t *testing.T) {
	choices := []setupChoice{
		{label: "  Install Prometheus", install: true},
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Move down to manual option
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(setupModel)
	if model.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", model.cursor)
	}

	// Select manual
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(setupModel)

	if !model.inputMode {
		t.Error("expected inputMode after selecting manual")
	}
	if model.done {
		t.Error("should not be done yet — need to enter URL")
	}
}

func TestSetupModel_ManualURLEntry(t *testing.T) {
	choices := []setupChoice{
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Select manual
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	// Type a URL character by character
	for _, ch := range "http://localhost:9090" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(setupModel)
	}

	if model.inputValue != "http://localhost:9090" {
		t.Errorf("expected input 'http://localhost:9090', got %q", model.inputValue)
	}

	// Press enter to confirm
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(setupModel)

	if !model.done {
		t.Error("expected done after entering URL")
	}
	if model.manualURL != "http://localhost:9090" {
		t.Errorf("expected manualURL 'http://localhost:9090', got %q", model.manualURL)
	}
}

func TestSetupModel_ManualURLBackspace(t *testing.T) {
	choices := []setupChoice{
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Enter input mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	// Type "abc"
	for _, ch := range "abc" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(setupModel)
	}

	// Backspace
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = updated.(setupModel)

	if model.inputValue != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", model.inputValue)
	}
}

func TestSetupModel_ManualURLEscGoesBack(t *testing.T) {
	choices := []setupChoice{
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Enter input mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	// Type something
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(setupModel)

	// Esc to go back
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(setupModel)

	if model.inputMode {
		t.Error("expected inputMode to be false after esc")
	}
	if model.inputValue != "" {
		t.Errorf("expected empty input after esc, got %q", model.inputValue)
	}
	if model.done {
		t.Error("should not be done after esc")
	}
}

func TestSetupModel_EmptyURLIgnored(t *testing.T) {
	choices := []setupChoice{
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Enter input mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	// Press enter with no input
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(setupModel)

	if model.done {
		t.Error("should not be done with empty URL")
	}
}

func TestSetupModel_Quit(t *testing.T) {
	choices := []setupChoice{
		{label: "  something"},
	}

	m := newSetupModel(choices, "ctx")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(setupModel)

	if !model.cancelled {
		t.Error("expected cancelled after 'q'")
	}
}

func TestSetupModel_CtrlCQuits(t *testing.T) {
	choices := []setupChoice{
		{label: "  something"},
	}

	m := newSetupModel(choices, "ctx")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(setupModel)

	if !model.cancelled {
		t.Error("expected cancelled after ctrl+c")
	}
}

func TestSetupModel_CursorBounds(t *testing.T) {
	choices := []setupChoice{
		{label: "  first"},
		{label: "  second"},
	}

	m := newSetupModel(choices, "ctx")

	// Try to go above 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	model := updated.(setupModel)
	if model.cursor != 0 {
		t.Errorf("cursor should stay at 0 when pressing up at top, got %d", model.cursor)
	}

	// Go down
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(setupModel)
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", model.cursor)
	}

	// Try to go below max
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(setupModel)
	if model.cursor != 1 {
		t.Errorf("cursor should stay at 1 when pressing down at bottom, got %d", model.cursor)
	}
}

func TestSetupModel_ViewShowsChoices(t *testing.T) {
	choices := []setupChoice{
		{label: "  monitoring/prometheus (port 9090)"},
		{label: "  Install Prometheus"},
		{label: "  Enter URL manually"},
	}

	m := newSetupModel(choices, "test-ctx")
	view := m.View()

	if !containsStr(view, "test-ctx") {
		t.Error("view should contain context name")
	}
	if !containsStr(view, "prometheus") {
		t.Error("view should contain prometheus choice")
	}
	if !containsStr(view, "Install") {
		t.Error("view should contain install choice")
	}
	if !containsStr(view, "URL") {
		t.Error("view should contain URL choice")
	}
}

func TestSetupModel_ViewInputMode(t *testing.T) {
	choices := []setupChoice{
		{label: "  Enter URL manually", manual: true},
	}

	m := newSetupModel(choices, "ctx")

	// Enter input mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(setupModel)

	view := model.View()
	if !containsStr(view, "Enter Prometheus URL") {
		t.Error("input mode view should contain URL prompt")
	}
	if !containsStr(view, "esc back") {
		t.Error("input mode view should contain esc hint")
	}
}

func TestSetupModel_ViewDoneIsEmpty(t *testing.T) {
	m := newSetupModel(nil, "ctx")
	m.done = true

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
