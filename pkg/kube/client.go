package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// ContainerMetrics holds resource usage for a single container.
type ContainerMetrics struct {
	Name     string
	CPUMilli int64
	MemoryMB int64
}

// ResourceMetrics holds aggregated resource usage.
type ResourceMetrics struct {
	PodName    string
	Containers []ContainerMetrics
	TotalCPU   int64 // millicores
	TotalMem   int64 // megabytes
}

// Client wraps Kubernetes and Metrics API clients.
type Client struct {
	kube    kubernetes.Interface
	metrics metricsv.Interface
}

// NewClient creates a Client using the given kube context (empty = default).
func NewClient(kubeCtx string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if kubeCtx != "" {
		overrides.CurrentContext = kubeCtx
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kube client: %w", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating metrics client: %w", err)
	}

	return &Client{kube: kubeClient, metrics: metricsClient}, nil
}

// NewClientFromInterfaces creates a Client from pre-built interfaces (useful for testing).
func NewClientFromInterfaces(kubeClient kubernetes.Interface, metricsClient metricsv.Interface) *Client {
	return &Client{kube: kubeClient, metrics: metricsClient}
}

// ListPodNames returns pod names in the given namespace.
func (c *Client) ListPodNames(ns string) ([]string, error) {
	pods, err := c.kube.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}
	return names, nil
}

// ListDeploymentNames returns deployment names in the given namespace.
func (c *Client) ListDeploymentNames(ns string) ([]string, error) {
	deps, err := c.kube.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(deps.Items))
	for _, d := range deps.Items {
		names = append(names, d.Name)
	}
	return names, nil
}

// GetPodMetrics fetches resource usage for a single pod.
func (c *Client) GetPodMetrics(ns, name string) ([]ResourceMetrics, error) {
	pm, err := c.metrics.MetricsV1beta1().PodMetricses(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	rm := ResourceMetrics{PodName: name}
	for _, container := range pm.Containers {
		cpu := container.Usage.Cpu().MilliValue()
		mem := container.Usage.Memory().Value() / (1024 * 1024)
		rm.Containers = append(rm.Containers, ContainerMetrics{
			Name:     container.Name,
			CPUMilli: cpu,
			MemoryMB: mem,
		})
		rm.TotalCPU += cpu
		rm.TotalMem += mem
	}

	return []ResourceMetrics{rm}, nil
}

// GetDeploymentMetrics fetches resource usage for all pods in a deployment.
func (c *Client) GetDeploymentMetrics(ns, name string) ([]ResourceMetrics, error) {
	dep, err := c.kube.AppsV1().Deployments(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment: %w", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("parsing selector: %w", err)
	}

	pods, err := c.kube.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var results []ResourceMetrics
	for _, pod := range pods.Items {
		pm, err := c.metrics.MetricsV1beta1().PodMetricses(ns).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			continue // pod may not have metrics yet
		}

		rm := ResourceMetrics{PodName: pod.Name}
		for _, container := range pm.Containers {
			cpu := container.Usage.Cpu().MilliValue()
			mem := container.Usage.Memory().Value() / (1024 * 1024)
			rm.Containers = append(rm.Containers, ContainerMetrics{
				Name:     container.Name,
				CPUMilli: cpu,
				MemoryMB: mem,
			})
			rm.TotalCPU += cpu
			rm.TotalMem += mem
		}
		results = append(results, rm)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no metrics found for deployment %s", name)
	}
	return results, nil
}
