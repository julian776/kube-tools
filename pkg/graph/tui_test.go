package graph

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/julianalvarez/kube-tools/pkg/kube"
)

func mockFetcher(tr TimeRange) ([]kube.ResourceMetrics, error) {
	return []kube.ResourceMetrics{
		{
			PodName: "test-pod",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 100, MemoryMB: 64},
			},
			TotalCPU: 100,
			TotalMem: 64,
		},
	}, nil
}

func TestNewModel(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	if m.kind != "Pod" {
		t.Errorf("expected kind 'Pod', got %q", m.kind)
	}
	if m.name != "test-pod" {
		t.Errorf("expected name 'test-pod', got %q", m.name)
	}
	if m.activeTab != 0 {
		t.Errorf("expected activeTab 0, got %d", m.activeTab)
	}
}

func TestModel_TabNavigation(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	// Move right
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := updated.(Model)
	if model.activeTab != 1 {
		t.Errorf("expected activeTab 1 after right, got %d", model.activeTab)
	}

	// Move right again
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if model.activeTab != 2 {
		t.Errorf("expected activeTab 2 after right, got %d", model.activeTab)
	}

	// Move left
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if model.activeTab != 1 {
		t.Errorf("expected activeTab 1 after left, got %d", model.activeTab)
	}

	// Wrap around right
	m.activeTab = len(timeRanges) - 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if model.activeTab != 0 {
		t.Errorf("expected activeTab 0 after wrapping right, got %d", model.activeTab)
	}

	// Wrap around left
	m.activeTab = 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if model.activeTab != len(timeRanges)-1 {
		t.Errorf("expected activeTab %d after wrapping left, got %d", len(timeRanges)-1, model.activeTab)
	}
}

func TestModel_MetricsMsg(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	metrics := []kube.ResourceMetrics{
		{PodName: "test-pod", TotalCPU: 100, TotalMem: 64},
	}

	updated, _ := m.Update(metricsMsg{metrics})
	model := updated.(Model)

	if model.metrics == nil {
		t.Error("expected metrics to be set")
	}
	if model.err != nil {
		t.Error("expected no error")
	}
}

func TestModel_ErrMsg(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	updated, _ := m.Update(errMsg{err: fmt.Errorf("test error")})
	model := updated.(Model)

	if model.err == nil {
		t.Error("expected error to be set")
	}
	if model.metrics != nil {
		t.Error("expected metrics to be nil on error")
	}
}

func TestModel_View_Loading(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	view := m.View()
	if !contains(view, "Loading") {
		t.Error("expected 'Loading' in initial view")
	}
}

func TestModel_View_WithMetrics(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)
	m.metrics = []kube.ResourceMetrics{
		{
			PodName: "test-pod",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 100, MemoryMB: 64},
			},
			TotalCPU: 100,
			TotalMem: 64,
		},
	}

	view := m.View()
	if !contains(view, "100m") {
		t.Error("expected '100m' in view with metrics")
	}
	if !contains(view, "64Mi") {
		t.Error("expected '64Mi' in view with metrics")
	}
}

func TestModel_View_WithError(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)
	m.err = fmt.Errorf("connection refused")

	view := m.View()
	if !contains(view, "connection refused") {
		t.Error("expected error message in view")
	}
}

func TestModel_View_TabLabels(t *testing.T) {
	m := NewModel("Pod", "test-pod", mockFetcher)

	view := m.View()
	for _, tr := range timeRanges {
		if !contains(view, tr.Label) {
			t.Errorf("expected tab label %q in view", tr.Label)
		}
	}
}

func TestTimeRanges(t *testing.T) {
	if len(timeRanges) != 4 {
		t.Fatalf("expected 4 time ranges, got %d", len(timeRanges))
	}

	expected := []struct {
		label    string
		duration string
	}{
		{"1 Hour", "1h"},
		{"4 Hours", "4h"},
		{"1 Day", "1d"},
		{"Today", "today"},
	}

	for i, e := range expected {
		if timeRanges[i].Label != e.label {
			t.Errorf("timeRanges[%d].Label = %q, want %q", i, timeRanges[i].Label, e.label)
		}
		if timeRanges[i].Duration != e.duration {
			t.Errorf("timeRanges[%d].Duration = %q, want %q", i, timeRanges[i].Duration, e.duration)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
