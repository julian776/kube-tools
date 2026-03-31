package kube

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultPromNamespace = "monitoring"
	defaultPromRelease   = "kube-prometheus"
	defaultPromChart     = "prometheus-community/kube-prometheus-stack"
)

// InstallPrometheus installs kube-prometheus-stack via Helm into the cluster.
// It returns the Prometheus service candidate once ready.
func (c *Client) InstallPrometheus(kubeCtx string, onStatus func(string)) (PrometheusCandidate, error) {
	// Check helm is available
	if _, err := exec.LookPath("helm"); err != nil {
		return PrometheusCandidate{}, fmt.Errorf("helm not found in PATH — install it from https://helm.sh/docs/intro/install/")
	}

	ctx := context.Background()

	// Create namespace if it doesn't exist
	onStatus("Creating namespace " + defaultPromNamespace + "...")
	_, err := c.kube.CoreV1().Namespaces().Get(ctx, defaultPromNamespace, metav1.GetOptions{})
	if err != nil {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: defaultPromNamespace}}
		if _, err := c.kube.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
			return PrometheusCandidate{}, fmt.Errorf("creating namespace: %w", err)
		}
	}

	// Add the helm repo
	onStatus("Adding prometheus-community Helm repo...")
	if err := runHelm(kubeCtx, "repo", "add", "prometheus-community", "https://prometheus-community.github.io/helm-charts"); err != nil {
		// Ignore "already exists" errors
		_ = err
	}
	if err := runHelm(kubeCtx, "repo", "update"); err != nil {
		return PrometheusCandidate{}, fmt.Errorf("updating helm repos: %w", err)
	}

	// Install kube-prometheus-stack
	onStatus("Installing kube-prometheus-stack (this may take a few minutes)...")
	err = runHelm(kubeCtx,
		"upgrade", "--install", defaultPromRelease, defaultPromChart,
		"--namespace", defaultPromNamespace,
		"--set", "prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false",
		"--wait", "--timeout", "5m",
	)
	if err != nil {
		return PrometheusCandidate{}, fmt.Errorf("installing kube-prometheus-stack: %w", err)
	}

	// Wait for the Prometheus service to appear
	onStatus("Waiting for Prometheus to be ready...")
	var candidate PrometheusCandidate
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		candidates, err := c.DiscoverPrometheus()
		if err == nil && len(candidates) > 0 {
			// Prefer the one in our namespace
			for _, cand := range candidates {
				if cand.Namespace == defaultPromNamespace {
					candidate = cand
					break
				}
			}
			if candidate.ServiceName == "" {
				candidate = candidates[0]
			}
			break
		}
		time.Sleep(3 * time.Second)
	}

	if candidate.ServiceName == "" {
		return PrometheusCandidate{}, fmt.Errorf("prometheus service not found after installation")
	}

	onStatus(fmt.Sprintf("Prometheus installed: %s", candidate.Display()))
	return candidate, nil
}

// runHelm executes a helm command with an optional kube context.
func runHelm(kubeCtx string, args ...string) error {
	if kubeCtx != "" {
		args = append([]string{"--kube-context", kubeCtx}, args...)
	}
	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}
