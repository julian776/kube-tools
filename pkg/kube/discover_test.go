package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestDiscoverPrometheus_ByName(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus-server", Namespace: "monitoring"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "http", Port: 9090}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "some-other-svc", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 8080}},
			},
		},
	)

	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())
	candidates, err := client.DiscoverPrometheus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	if candidates[0].ServiceName != "prometheus-server" {
		t.Errorf("expected 'prometheus-server', got %q", candidates[0].ServiceName)
	}
	if candidates[0].Namespace != "monitoring" {
		t.Errorf("expected 'monitoring', got %q", candidates[0].Namespace)
	}
	if candidates[0].Port != 9090 {
		t.Errorf("expected port 9090, got %d", candidates[0].Port)
	}
}

func TestDiscoverPrometheus_ByLabel(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-prom",
				Namespace: "observability",
				Labels:    map[string]string{"app.kubernetes.io/name": "prometheus"},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "web", Port: 9090}},
			},
		},
	)

	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())
	candidates, err := client.DiscoverPrometheus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].ServiceName != "my-prom" {
		t.Errorf("expected 'my-prom', got %q", candidates[0].ServiceName)
	}
}

func TestDiscoverPrometheus_MultipleCandidates(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus-server", Namespace: "monitoring"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus-k8s", Namespace: "monitoring"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9090}},
			},
		},
	)

	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())
	candidates, err := client.DiscoverPrometheus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
}

func TestDiscoverPrometheus_NoCandidates(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 6379}},
			},
		},
	)

	client := NewClientFromInterfaces(kubeClient, metricsfake.NewSimpleClientset())
	candidates, err := client.DiscoverPrometheus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestPrometheusCandidate_Display(t *testing.T) {
	c := PrometheusCandidate{
		ServiceName: "prometheus-server",
		Namespace:   "monitoring",
		Port:        9090,
	}

	display := c.Display()
	expected := "monitoring/prometheus-server (port 9090)"
	if display != expected {
		t.Errorf("expected %q, got %q", expected, display)
	}
}
