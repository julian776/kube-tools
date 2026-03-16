package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/julianalvarez/kube-tools/pkg/kube"
)

func TestBar(t *testing.T) {
	tests := []struct {
		name      string
		value     int64
		max       int64
		wantWidth int
	}{
		{"full bar", 100, 100, barMaxWidth},
		{"half bar", 50, 100, barMaxWidth / 2},
		{"zero value", 0, 100, 0},
		{"zero max", 0, 0, 0},
		{"small value gets min width", 1, 1000, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bar(tt.value, tt.max)
			if tt.max == 0 {
				if result != "" {
					t.Errorf("expected empty string for max=0, got %q", result)
				}
				return
			}

			filled := strings.Count(result, "█")
			if filled != tt.wantWidth {
				t.Errorf("bar(%d, %d): filled width = %d, want %d", tt.value, tt.max, filled, tt.wantWidth)
			}

			// Verify total bar width is always barMaxWidth (plus brackets)
			inner := result[1 : len(result)-1] // strip [ and ]
			// Count runes, not bytes, since █ is multi-byte
			runeCount := 0
			for range inner {
				runeCount++
			}
			if runeCount != barMaxWidth {
				t.Errorf("bar inner width = %d runes, want %d", runeCount, barMaxWidth)
			}
		})
	}
}

func TestRenderResourceUsage_SinglePod(t *testing.T) {
	var buf bytes.Buffer

	metrics := []kube.ResourceMetrics{
		{
			PodName: "my-pod",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 250, MemoryMB: 128},
				{Name: "sidecar", CPUMilli: 50, MemoryMB: 32},
			},
			TotalCPU: 300,
			TotalMem: 160,
		},
	}

	RenderResourceUsage(&buf, "Pod", "my-pod", metrics)
	output := buf.String()

	// Verify header
	if !strings.Contains(output, "Pod: my-pod") {
		t.Error("output should contain 'Pod: my-pod'")
	}

	// Verify containers are listed
	if !strings.Contains(output, "Container: app") {
		t.Error("output should contain 'Container: app'")
	}
	if !strings.Contains(output, "Container: sidecar") {
		t.Error("output should contain 'Container: sidecar'")
	}

	// Verify CPU and memory values
	if !strings.Contains(output, "250m") {
		t.Error("output should contain '250m' for CPU")
	}
	if !strings.Contains(output, "128Mi") {
		t.Error("output should contain '128Mi' for memory")
	}

	// Verify totals
	if !strings.Contains(output, "Total: CPU 300m | Memory 160Mi") {
		t.Error("output should contain total line")
	}

	// Single pod should NOT show "Pod:" prefix per pod or grand total
	if strings.Contains(output, "Grand Total") {
		t.Error("single pod should not show grand total")
	}
}

func TestRenderResourceUsage_MultiplePods(t *testing.T) {
	var buf bytes.Buffer

	metrics := []kube.ResourceMetrics{
		{
			PodName: "pod-1",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 100, MemoryMB: 64},
			},
			TotalCPU: 100,
			TotalMem: 64,
		},
		{
			PodName: "pod-2",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 200, MemoryMB: 128},
			},
			TotalCPU: 200,
			TotalMem: 128,
		},
	}

	RenderResourceUsage(&buf, "Deployment", "my-deploy", metrics)
	output := buf.String()

	// Verify deployment header
	if !strings.Contains(output, "Deployment: my-deploy") {
		t.Error("output should contain 'Deployment: my-deploy'")
	}

	// Multiple pods should show "Pod:" labels
	if !strings.Contains(output, "Pod: pod-1") {
		t.Error("output should contain 'Pod: pod-1'")
	}
	if !strings.Contains(output, "Pod: pod-2") {
		t.Error("output should contain 'Pod: pod-2'")
	}

	// Verify grand total
	if !strings.Contains(output, "Grand Total (2 pods): CPU 300m | Memory 192Mi") {
		t.Error("output should contain grand total")
	}
}

func TestRenderResourceUsage_ZeroMetrics(t *testing.T) {
	var buf bytes.Buffer

	metrics := []kube.ResourceMetrics{
		{
			PodName: "idle-pod",
			Containers: []kube.ContainerMetrics{
				{Name: "app", CPUMilli: 0, MemoryMB: 0},
			},
			TotalCPU: 0,
			TotalMem: 0,
		},
	}

	RenderResourceUsage(&buf, "Pod", "idle-pod", metrics)
	output := buf.String()

	if !strings.Contains(output, "0m") {
		t.Error("output should contain '0m' for zero CPU")
	}
	if !strings.Contains(output, "0Mi") {
		t.Error("output should contain '0Mi' for zero memory")
	}
}
