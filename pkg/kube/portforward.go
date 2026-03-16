package kube

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward establishes a port-forward to a Kubernetes service and returns
// the local URL and a stop function.
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

	// Find a pod backing the service to port-forward to
	// We port-forward to the service endpoint via the API server
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%d/proxy", namespace, serviceName, remotePort)
	_ = path

	// Use pod-based port-forward for reliability
	// First, find a random free local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("finding free port: %w", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Build the port-forward URL to the service
	serverURL, err := url.Parse(config.Host)
	if err != nil {
		return "", nil, fmt.Errorf("parsing server URL: %w", err)
	}
	serverURL.Path = fmt.Sprintf("/api/v1/namespaces/%s/services/%s/portforward", namespace, serviceName)

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return "", nil, fmt.Errorf("creating round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	fw, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
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
