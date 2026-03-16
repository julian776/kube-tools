package cmd

import (
	"fmt"
	"os"

	"github.com/julian776/kube-tools/pkg/config"
	"github.com/julian776/kube-tools/pkg/kube"
	"github.com/spf13/cobra"
)

var (
	namespace     string
	prometheusURL string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize resource usage for Kubernetes objects",
	Long:  "Display CPU and memory usage graphs for pods, deployments, and other Kubernetes resources.",
}

func init() {
	graphCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	graphCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "", "Prometheus server URL (overrides config)")
	rootCmd.AddCommand(graphCmd)
}

// resolvePrometheusURL determines the Prometheus URL to use.
// Precedence: --prometheus-url flag > KUBE_TOOLS_PROMETHEUS_URL env > config file.
// If the config references a service, a port-forward is started and the stop func must be called.
func resolvePrometheusURL() (url string, stop func(), err error) {
	// 1. Flag
	if prometheusURL != "" {
		return prometheusURL, nil, nil
	}

	// 2. Environment variable
	if envURL := os.Getenv("KUBE_TOOLS_PROMETHEUS_URL"); envURL != "" {
		return envURL, nil, nil
	}

	// 3. Config file
	cfg, err := config.Load()
	if err != nil {
		return "", nil, nil // silently fall back
	}

	client, err := kube.NewClient(kubeContext)
	if err != nil {
		return "", nil, nil
	}

	ctxName, err := client.CurrentContext(kubeContext)
	if err != nil {
		return "", nil, nil
	}

	prom, ok := cfg.GetPrometheus(ctxName)
	if !ok {
		return "", nil, nil
	}

	// Direct URL configured
	if prom.URL != "" {
		return prom.URL, nil, nil
	}

	// Service reference — port-forward
	if prom.ServiceName != "" {
		localURL, stopFn, err := kube.PortForward(kubeContext, prom.Namespace, prom.ServiceName, prom.Port)
		if err != nil {
			return "", nil, fmt.Errorf("port-forwarding to %s/%s: %w", prom.Namespace, prom.ServiceName, err)
		}
		return localURL, stopFn, nil
	}

	return "", nil, nil
}
