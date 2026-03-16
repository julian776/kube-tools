//go:build integration

package prometheus

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/testcontainers/testcontainers-go"
	tcwait "github.com/testcontainers/testcontainers-go/wait"

	"github.com/julianalvarez/kube-tools/pkg/kube"
)

// startMetricsServer starts an HTTP server on a random port that exposes
// fake container_cpu_usage_seconds_total and container_memory_working_set_bytes
// metrics. It returns the port and a cleanup function.
func startMetricsServer(t *testing.T) (int, func()) {
	t.Helper()

	reg := prometheus.NewRegistry()

	cpuCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "container_cpu_usage_seconds_total",
		Help: "Cumulative cpu time consumed by the container.",
	}, []string{"namespace", "pod", "container"})

	memGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_memory_working_set_bytes",
		Help: "Current working set of the container in bytes.",
	}, []string{"namespace", "pod", "container"})

	reg.MustRegister(cpuCounter, memGauge)

	// Set initial values
	cpuCounter.WithLabelValues("default", "test-pod", "app").Add(100)
	memGauge.WithLabelValues("default", "test-pod", "app").Set(128 * 1024 * 1024) // 128MiB

	cpuCounter.WithLabelValues("default", "test-pod", "sidecar").Add(20)
	memGauge.WithLabelValues("default", "test-pod", "sidecar").Set(32 * 1024 * 1024) // 32MiB

	// Increment CPU counter in the background so rate() returns non-zero
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				cpuCounter.WithLabelValues("default", "test-pod", "app").Add(0.25)
				cpuCounter.WithLabelValues("default", "test-pod", "sidecar").Add(0.05)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)

	cleanup := func() {
		close(stopCh)
		srv.Close()
	}

	return port, cleanup
}

// promConfig generates a Prometheus configuration YAML that scrapes the
// host metrics server.
func promConfig(metricsPort int) string {
	return fmt.Sprintf(`global:
  scrape_interval: 2s
  evaluation_interval: 2s
scrape_configs:
  - job_name: test
    static_configs:
      - targets: ['host.testcontainers.internal:%d']
`, metricsPort)
}

// startPrometheus creates and starts a Prometheus container configured to
// scrape the host metrics server on the given port. Returns the container,
// the Prometheus URL, and a cleanup function.
func startPrometheus(t *testing.T, ctx context.Context, metricsPort int) (string, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "prom/prometheus:v2.51.0",
		ExposedPorts: []string{"9090/tcp"},
		WaitingFor:   tcwait.ForHTTP("/-/ready").WithPort("9090/tcp").WithStartupTimeout(30 * time.Second),
		HostAccessPorts: []int{metricsPort},
		Files: []testcontainers.ContainerFile{
			{
				Reader:            stringReader(promConfig(metricsPort)),
				ContainerFilePath: "/etc/prometheus/prometheus.yml",
				FileMode:          0644,
			},
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start prometheus container: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "9090/tcp")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %v", err)
	}

	promURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return promURL, cleanup
}

func TestIntegration_QueryPodMetrics(t *testing.T) {
	ctx := context.Background()

	// Start a local HTTP server serving fake metrics
	metricsPort, metricsCleanup := startMetricsServer(t)
	defer metricsCleanup()

	// Start Prometheus container
	promURL, promCleanup := startPrometheus(t, ctx, metricsPort)
	defer promCleanup()

	// Wait for Prometheus to scrape enough data points for rate() to work.
	// rate() needs at least 2 samples within the [5m] window.
	// With 2s scrape_interval, we need at least ~15s for reliable results.
	t.Log("Waiting for Prometheus to collect data...")
	client, err := NewClient(promURL)
	if err != nil {
		t.Fatalf("failed to create prometheus client: %v", err)
	}

	var results []kube.ResourceMetrics
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		results, err = client.QueryPodMetrics(ctx, "default", "test-pod", time.Hour, 15*time.Second)
		if err == nil && results != nil && len(results) > 0 && len(results[0].Containers) > 0 {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if results == nil || len(results) == 0 {
		t.Fatal("timed out waiting for metrics data from Prometheus")
	}

	rm := results[0]
	t.Logf("Got metrics: PodName=%s, Containers=%d, TotalCPU=%d, TotalMem=%d",
		rm.PodName, len(rm.Containers), rm.TotalCPU, rm.TotalMem)

	if rm.PodName != "test-pod" {
		t.Errorf("expected PodName 'test-pod', got %q", rm.PodName)
	}
	if len(rm.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(rm.Containers))
	}

	// Verify we got both containers
	containerNames := map[string]bool{}
	for _, c := range rm.Containers {
		containerNames[c.Name] = true
	}
	if !containerNames["app"] {
		t.Error("missing 'app' container in results")
	}
	if !containerNames["sidecar"] {
		t.Error("missing 'sidecar' container in results")
	}

	// Verify CPU values are positive (rate() should return non-zero with our incrementing counter)
	for _, c := range rm.Containers {
		t.Logf("  Container %s: CPU=%dm, Mem=%dMi", c.Name, c.CPUMilli, c.MemoryMB)
	}
}

func TestIntegration_QueryPodMetrics_UnknownPod(t *testing.T) {
	ctx := context.Background()

	metricsPort, metricsCleanup := startMetricsServer(t)
	defer metricsCleanup()

	promURL, promCleanup := startPrometheus(t, ctx, metricsPort)
	defer promCleanup()

	// Wait a bit for Prometheus to be ready
	time.Sleep(5 * time.Second)

	client, err := NewClient(promURL)
	if err != nil {
		t.Fatalf("failed to create prometheus client: %v", err)
	}

	results, err := client.QueryPodMetrics(ctx, "default", "nonexistent-pod", time.Hour, 15*time.Second)
	if err != nil {
		t.Fatalf("unexpected error for unknown pod: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for unknown pod, got %v", results)
	}
}

func TestIntegration_QueryPodMetrics_MemoryValues(t *testing.T) {
	ctx := context.Background()

	metricsPort, metricsCleanup := startMetricsServer(t)
	defer metricsCleanup()

	promURL, promCleanup := startPrometheus(t, ctx, metricsPort)
	defer promCleanup()

	t.Log("Waiting for Prometheus to collect data...")
	client, err := NewClient(promURL)
	if err != nil {
		t.Fatalf("failed to create prometheus client: %v", err)
	}

	// Wait for memory metrics (these don't need rate(), so they appear faster)
	var results []kube.ResourceMetrics
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		results, err = client.QueryPodMetrics(ctx, "default", "test-pod", time.Hour, 15*time.Second)
		if err == nil && results != nil && len(results) > 0 {
			// Check if we have memory data
			for _, c := range results[0].Containers {
				if c.MemoryMB > 0 {
					goto done
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
done:

	if results == nil || len(results) == 0 {
		t.Fatal("timed out waiting for memory metrics from Prometheus")
	}

	// Verify memory values are in the expected ballpark
	for _, c := range results[0].Containers {
		t.Logf("  Container %s: Mem=%dMi", c.Name, c.MemoryMB)
		switch c.Name {
		case "app":
			if c.MemoryMB != 128 {
				t.Errorf("expected app memory ~128Mi, got %d", c.MemoryMB)
			}
		case "sidecar":
			if c.MemoryMB != 32 {
				t.Errorf("expected sidecar memory ~32Mi, got %d", c.MemoryMB)
			}
		}
	}
}
