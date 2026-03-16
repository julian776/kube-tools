package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/julian776/kube-tools/pkg/kube"
)

const barMaxWidth = 40

// RenderResourceUsage prints a terminal bar chart of resource usage.
func RenderResourceUsage(w io.Writer, kind, name string, metrics []kube.ResourceMetrics) {
	fmt.Fprintf(w, "\n  %s: %s\n", kind, name)
	fmt.Fprintf(w, "  %s\n\n", strings.Repeat("─", 60))

	// Find max values for scaling bars
	var maxCPU, maxMem int64
	for _, rm := range metrics {
		for _, c := range rm.Containers {
			if c.CPUMilli > maxCPU {
				maxCPU = c.CPUMilli
			}
			if c.MemoryMB > maxMem {
				maxMem = c.MemoryMB
			}
		}
	}
	if maxCPU == 0 {
		maxCPU = 1
	}
	if maxMem == 0 {
		maxMem = 1
	}

	for _, rm := range metrics {
		if len(metrics) > 1 {
			fmt.Fprintf(w, "  Pod: %s\n", rm.PodName)
		}

		for _, c := range rm.Containers {
			fmt.Fprintf(w, "  Container: %s\n", c.Name)

			cpuBar := bar(c.CPUMilli, maxCPU)
			memBar := bar(c.MemoryMB, maxMem)

			fmt.Fprintf(w, "    CPU  %s %dm\n", cpuBar, c.CPUMilli)
			fmt.Fprintf(w, "    MEM  %s %dMi\n", memBar, c.MemoryMB)
			fmt.Fprintln(w)
		}

		fmt.Fprintf(w, "  Total: CPU %dm | Memory %dMi\n", rm.TotalCPU, rm.TotalMem)
		fmt.Fprintf(w, "  %s\n\n", strings.Repeat("─", 60))
	}

	if len(metrics) > 1 {
		var totalCPU, totalMem int64
		for _, rm := range metrics {
			totalCPU += rm.TotalCPU
			totalMem += rm.TotalMem
		}
		fmt.Fprintf(w, "  Grand Total (%d pods): CPU %dm | Memory %dMi\n\n", len(metrics), totalCPU, totalMem)
	}
}

func bar(value, max int64) string {
	if max == 0 {
		return ""
	}
	width := int(float64(value) / float64(max) * barMaxWidth)
	if width < 1 && value > 0 {
		width = 1
	}
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", width), strings.Repeat(" ", barMaxWidth-width))
}
