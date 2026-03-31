package kube

import (
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// fakeHelmRecorder records all helm calls and returns a configurable error.
type fakeHelmRecorder struct {
	calls []string
	err   error
}

func (f *fakeHelmRecorder) run(kubeCtx string, args ...string) error {
	call := strings.Join(args, " ")
	if kubeCtx != "" {
		call = "--kube-context " + kubeCtx + " " + call
	}
	f.calls = append(f.calls, call)
	return f.err
}

// stubHelm swaps both helmChecker and helmRunner for testing, returning a
// cleanup function to restore originals.
func stubHelm(runner HelmRunner) func() {
	origRunner := helmRunner
	origChecker := helmChecker
	helmRunner = runner
	helmChecker = func() error { return nil }
	return func() {
		helmRunner = origRunner
		helmChecker = origChecker
	}
}

func TestInstallPrometheus_CreatesNamespace(t *testing.T) {
	recorder := &fakeHelmRecorder{}
	defer stubHelm(recorder.run)()

	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-server",
				Namespace: defaultPromNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "http", Port: 9090}},
			},
		},
	)
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	var statuses []string
	onStatus := func(s string) { statuses = append(statuses, s) }

	candidate, err := client.InstallPrometheus("", onStatus)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify namespace was created
	ns, err := kubeClient.CoreV1().Namespaces().Get(nil, defaultPromNamespace, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("namespace should exist: %v", err)
	}
	if ns.Name != defaultPromNamespace {
		t.Errorf("expected namespace %q, got %q", defaultPromNamespace, ns.Name)
	}

	// Verify candidate
	if candidate.ServiceName != "prometheus-server" {
		t.Errorf("expected service 'prometheus-server', got %q", candidate.ServiceName)
	}
	if candidate.Namespace != defaultPromNamespace {
		t.Errorf("expected namespace %q, got %q", defaultPromNamespace, candidate.Namespace)
	}
	if candidate.Port != 9090 {
		t.Errorf("expected port 9090, got %d", candidate.Port)
	}

	// Verify status messages were emitted
	if len(statuses) < 3 {
		t.Errorf("expected at least 3 status messages, got %d", len(statuses))
	}
}

func TestInstallPrometheus_SkipsNamespaceIfExists(t *testing.T) {
	recorder := &fakeHelmRecorder{}
	defer stubHelm(recorder.run)()

	kubeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: defaultPromNamespace}},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-server",
				Namespace: defaultPromNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
	)
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify there's still only one namespace (no duplicate create)
	nsList, _ := kubeClient.CoreV1().Namespaces().List(nil, metav1.ListOptions{})
	count := 0
	for _, ns := range nsList.Items {
		if ns.Name == defaultPromNamespace {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 monitoring namespace, got %d", count)
	}
}

func TestInstallPrometheus_HelmCallsCorrect(t *testing.T) {
	recorder := &fakeHelmRecorder{}
	defer stubHelm(recorder.run)()

	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-server",
				Namespace: defaultPromNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
	)
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recorder.calls) != 3 {
		t.Fatalf("expected 3 helm calls, got %d: %v", len(recorder.calls), recorder.calls)
	}

	if !strings.Contains(recorder.calls[0], "repo add prometheus-community") {
		t.Errorf("first call should be repo add, got: %s", recorder.calls[0])
	}
	if !strings.Contains(recorder.calls[1], "repo update") {
		t.Errorf("second call should be repo update, got: %s", recorder.calls[1])
	}
	if !strings.Contains(recorder.calls[2], "upgrade --install "+defaultPromRelease) {
		t.Errorf("third call should be upgrade --install, got: %s", recorder.calls[2])
	}
	if !strings.Contains(recorder.calls[2], "--namespace "+defaultPromNamespace) {
		t.Errorf("install should target namespace %s, got: %s", defaultPromNamespace, recorder.calls[2])
	}
}

func TestInstallPrometheus_WithKubeContext(t *testing.T) {
	recorder := &fakeHelmRecorder{}
	defer stubHelm(recorder.run)()

	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-server",
				Namespace: defaultPromNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
	)
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("my-cluster", func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, call := range recorder.calls {
		if !strings.Contains(call, "--kube-context my-cluster") {
			t.Errorf("expected --kube-context my-cluster in call: %s", call)
		}
	}
}

func TestInstallPrometheus_HelmRepoUpdateFails(t *testing.T) {
	callCount := 0
	defer stubHelm(func(kubeCtx string, args ...string) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("network error")
		}
		return nil
	})()

	kubeClient := fake.NewSimpleClientset()
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err == nil {
		t.Fatal("expected error when repo update fails")
	}
	if !strings.Contains(err.Error(), "updating helm repos") {
		t.Errorf("expected 'updating helm repos' error, got: %v", err)
	}
}

func TestInstallPrometheus_HelmInstallFails(t *testing.T) {
	callCount := 0
	defer stubHelm(func(kubeCtx string, args ...string) error {
		callCount++
		if callCount == 3 {
			return fmt.Errorf("chart not found")
		}
		return nil
	})()

	kubeClient := fake.NewSimpleClientset()
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err == nil {
		t.Fatal("expected error when install fails")
	}
	if !strings.Contains(err.Error(), "installing kube-prometheus-stack") {
		t.Errorf("expected 'installing kube-prometheus-stack' error, got: %v", err)
	}
}

func TestInstallPrometheus_NoServiceAfterInstall(t *testing.T) {
	defer stubHelm(func(kubeCtx string, args ...string) error { return nil })()
	origTimeout := discoveryTimeout
	discoveryTimeout = 1 * time.Second
	defer func() { discoveryTimeout = origTimeout }()

	kubeClient := fake.NewSimpleClientset()
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err == nil {
		t.Fatal("expected error when no service found after install")
	}
	if !strings.Contains(err.Error(), "prometheus service not found after installation") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallPrometheus_PrefersMonitoringNamespace(t *testing.T) {
	defer stubHelm(func(kubeCtx string, args ...string) error { return nil })()

	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-server",
				Namespace: "other-namespace",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-k8s",
				Namespace: defaultPromNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
	)
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	candidate, err := client.InstallPrometheus("", func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if candidate.Namespace != defaultPromNamespace {
		t.Errorf("expected candidate in %q, got %q", defaultPromNamespace, candidate.Namespace)
	}
}

func TestInstallPrometheus_HelmNotFound(t *testing.T) {
	origChecker := helmChecker
	helmChecker = func() error { return fmt.Errorf("not found") }
	defer func() { helmChecker = origChecker }()

	kubeClient := fake.NewSimpleClientset()
	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())

	_, err := client.InstallPrometheus("", func(string) {})
	if err == nil {
		t.Fatal("expected error when helm not found")
	}
	if !strings.Contains(err.Error(), "helm not found in PATH") {
		t.Errorf("expected 'helm not found' error, got: %v", err)
	}
}
