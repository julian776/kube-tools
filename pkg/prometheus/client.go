package prometheus

import (
	"context"
	"fmt"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/julian776/kube-tools/pkg/kube"
)

// Client wraps the Prometheus HTTP API for querying container metrics.
type Client struct {
	api promv1.API
}

// NewClient creates a Prometheus client pointing at the given address (e.g. "http://localhost:9090").
func NewClient(address string) (*Client, error) {
	c, err := promapi.NewClient(promapi.Config{Address: address})
	if err != nil {
		return nil, fmt.Errorf("creating prometheus client: %w", err)
	}
	return &Client{api: promv1.NewAPI(c)}, nil
}

// QueryPodMetrics queries Prometheus for CPU and memory usage of a pod's containers
// over the given duration, returning results as ResourceMetrics.
func (c *Client) QueryPodMetrics(ctx context.Context, namespace, pod string, duration time.Duration, step time.Duration) ([]kube.ResourceMetrics, error) {
	end := time.Now()
	start := end.Add(-duration)

	cpuQuery := fmt.Sprintf(`rate(container_cpu_usage_seconds_total{namespace=%q, pod=%q, container!=""}[5m]) * 1000`, namespace, pod)
	memQuery := fmt.Sprintf(`container_memory_working_set_bytes{namespace=%q, pod=%q, container!=""} / 1048576`, namespace, pod)

	r := promv1.Range{Start: start, End: end, Step: step}

	cpuResult, _, err := c.api.QueryRange(ctx, cpuQuery, r)
	if err != nil {
		return nil, fmt.Errorf("querying cpu: %w", err)
	}

	memResult, _, err := c.api.QueryRange(ctx, memQuery, r)
	if err != nil {
		return nil, fmt.Errorf("querying memory: %w", err)
	}

	cpuMatrix, ok := cpuResult.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("unexpected cpu result type: %T", cpuResult)
	}

	memMatrix, ok := memResult.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("unexpected memory result type: %T", memResult)
	}

	// Build a map of container -> latest CPU value
	cpuByContainer := make(map[string]int64)
	for _, series := range cpuMatrix {
		container := string(series.Metric["container"])
		if len(series.Values) > 0 {
			latest := series.Values[len(series.Values)-1]
			cpuByContainer[container] = int64(latest.Value)
		}
	}

	// Build a map of container -> latest Memory value
	memByContainer := make(map[string]int64)
	for _, series := range memMatrix {
		container := string(series.Metric["container"])
		if len(series.Values) > 0 {
			latest := series.Values[len(series.Values)-1]
			memByContainer[container] = int64(latest.Value)
		}
	}

	// Merge into ResourceMetrics
	containers := make(map[string]bool)
	for c := range cpuByContainer {
		containers[c] = true
	}
	for c := range memByContainer {
		containers[c] = true
	}

	if len(containers) == 0 {
		return nil, nil
	}

	rm := kube.ResourceMetrics{PodName: pod}
	for name := range containers {
		cpu := cpuByContainer[name]
		mem := memByContainer[name]
		rm.Containers = append(rm.Containers, kube.ContainerMetrics{
			Name:     name,
			CPUMilli: cpu,
			MemoryMB: mem,
		})
		rm.TotalCPU += cpu
		rm.TotalMem += mem
	}

	return []kube.ResourceMetrics{rm}, nil
}

// ParseDuration converts a TimeRange duration string to time.Duration.
// Supports "1h", "4h", "1d", and "today".
func ParseDuration(d string) time.Duration {
	switch d {
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "today":
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return now.Sub(midnight)
	default:
		return time.Hour
	}
}
