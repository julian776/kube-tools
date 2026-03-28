package kube

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward establishes a port-forward to a pod backing a Kubernetes service.
// It resolves the service to a pod via the endpoint/selector, then port-forwards
// to that pod. Returns the local URL and a stop function.
func PortForward(kubeCtx, namespace, serviceName string, remotePort int) (localURL string, stop func(), err error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if kubeCtx != "" {
		overrides.CurrentContext = kubeCtx
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return "", nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", nil, fmt.Errorf("creating kube client: %w", err)
	}

	// Resolve the service to a backing pod
	podName, err := resolvePodForService(kubeClient, namespace, serviceName)
	if err != nil {
		return "", nil, fmt.Errorf("resolving pod for service %s/%s: %w", namespace, serviceName, err)
	}

	// Find a free local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("finding free port: %w", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Build the port-forward URL to the pod
	serverURL, err := url.Parse(config.Host)
	if err != nil {
		return "", nil, fmt.Errorf("parsing server URL: %w", err)
	}
	serverURL.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return "", nil, fmt.Errorf("creating round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	fw, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return "", nil, fmt.Errorf("creating port-forward: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- fw.ForwardPorts()
	}()

	select {
	case <-readyChan:
		// Port forward is ready
	case err := <-errChan:
		return "", nil, fmt.Errorf("port-forward failed: %w", err)
	}

	localURL = fmt.Sprintf("http://127.0.0.1:%d", localPort)
	stop = func() {
		close(stopChan)
	}

	return localURL, stop, nil
}

// resolvePodForService finds a running pod that backs the given service.
// It first checks Endpoints, then falls back to the service's selector.
func resolvePodForService(kubeClient kubernetes.Interface, namespace, serviceName string) (string, error) {
	ctx := context.Background()

	// Try Endpoints first — these are the actual pods the service routes to
	endpoints, err := kubeClient.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err == nil {
		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
					return addr.TargetRef.Name, nil
				}
			}
		}
	}

	// Fallback: get the service's selector and find a pod directly
	svc, err := kubeClient.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting service: %w", err)
	}

	if len(svc.Spec.Selector) == 0 {
		return "", fmt.Errorf("service %s has no selector", serviceName)
	}

	// Build label selector string
	var selectorParts []string
	for k, v := range svc.Spec.Selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}
	labelSelector := ""
	for i, p := range selectorParts {
		if i > 0 {
			labelSelector += ","
		}
		labelSelector += p
	}

	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         1,
	})
	if err != nil {
		return "", fmt.Errorf("listing pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for service %s", serviceName)
	}

	return pods.Items[0].Name, nil
}
