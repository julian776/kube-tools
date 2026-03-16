package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// PrometheusCandidate represents a potential Prometheus service found in the cluster.
type PrometheusCandidate struct {
	ServiceName string
	Namespace   string
	Port        int
}

// Display returns a human-readable label for the candidate.
func (c PrometheusCandidate) Display() string {
	return fmt.Sprintf("%s/%s (port %d)", c.Namespace, c.ServiceName, c.Port)
}

// DiscoverPrometheus searches for Prometheus services across all namespaces.
// It looks for services matching known names and labels.
func (c *Client) DiscoverPrometheus() ([]PrometheusCandidate, error) {
	ctx := context.Background()

	// Search all namespaces
	services, err := c.kube.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	var candidates []PrometheusCandidate
	seen := make(map[string]bool)

	for _, svc := range services.Items {
		if isPrometheusService(svc) {
			port := getPrometheusPort(svc)
			if port == 0 {
				continue
			}
			key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, PrometheusCandidate{
				ServiceName: svc.Name,
				Namespace:   svc.Namespace,
				Port:        port,
			})
		}
	}

	return candidates, nil
}

// isPrometheusService checks if a service is likely a Prometheus server.
func isPrometheusService(svc corev1.Service) bool {
	// Check common label patterns
	labels := svc.Labels
	if labels["app.kubernetes.io/name"] == "prometheus" {
		return true
	}
	if labels["app"] == "prometheus" {
		return true
	}
	if labels["app.kubernetes.io/component"] == "prometheus" {
		return true
	}

	// Check common name patterns
	name := svc.Name
	knownNames := []string{
		"prometheus",
		"prometheus-server",
		"prometheus-kube-prometheus-prometheus",
		"prometheus-operated",
		"kube-prometheus-stack-prometheus",
		"prometheus-k8s",
	}
	for _, known := range knownNames {
		if name == known {
			return true
		}
	}

	return false
}

// getPrometheusPort returns the most likely Prometheus HTTP port from a service.
func getPrometheusPort(svc corev1.Service) int {
	// Prefer well-known port names
	for _, port := range svc.Spec.Ports {
		if port.Name == "http-web" || port.Name == "web" || port.Name == "http" {
			return int(port.Port)
		}
	}
	// Fallback to port 9090
	for _, port := range svc.Spec.Ports {
		if port.Port == 9090 {
			return 9090
		}
	}
	// Fallback to first port
	if len(svc.Spec.Ports) > 0 {
		return int(svc.Spec.Ports[0].Port)
	}
	return 0
}

// CurrentContext returns the active kube context name.
func (c *Client) CurrentContext(kubeCtx string) (string, error) {
	if kubeCtx != "" {
		return kubeCtx, nil
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return "", fmt.Errorf("loading kubeconfig: %w", err)
	}
	return config.CurrentContext, nil
}
