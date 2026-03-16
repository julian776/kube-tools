package kube

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func newFakeClient(pods []corev1.Pod, deployments []appsv1.Deployment, podMetrics []metricsv1beta1.PodMetrics) *Client {
	kubeObjects := []runtime.Object{}
	for i := range pods {
		kubeObjects = append(kubeObjects, &pods[i])
	}
	for i := range deployments {
		kubeObjects = append(kubeObjects, &deployments[i])
	}
	kubeClient := fake.NewSimpleClientset(kubeObjects...)

	metricsObjects := []runtime.Object{}
	for i := range podMetrics {
		metricsObjects = append(metricsObjects, &podMetrics[i])
	}
	metricsClient := metricsfake.NewSimpleClientset(metricsObjects...)

	// The fake metrics clientset registers objects but PodMetrics Get uses
	// the standard reactor. We add a reactor that returns the right PodMetrics
	// for Get requests.
	metricsClient.PrependReactor("get", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getAction := action.(clienttesting.GetAction)
		for i := range podMetrics {
			if podMetrics[i].Name == getAction.GetName() && podMetrics[i].Namespace == getAction.GetNamespace() {
				return true, &podMetrics[i], nil
			}
		}
		return false, nil, nil
	})

	// Also handle List for metrics if needed.
	metricsClient.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		listAction := action.(clienttesting.ListAction)
		result := &metricsv1beta1.PodMetricsList{}
		for i := range podMetrics {
			if podMetrics[i].Namespace == listAction.GetNamespace() {
				result.Items = append(result.Items, podMetrics[i])
			}
		}
		return true, result, nil
	})

	return NewClientFromInterfaces(kubeClient, metricsClient)
}

func TestListPodNames(t *testing.T) {
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-ns-pod", Namespace: "kube-system"}},
	}

	client := newFakeClient(pods, nil, nil)

	names, err := client.ListPodNames("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(names))
	}

	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["pod-a"] || !nameSet["pod-b"] {
		t.Errorf("expected pod-a and pod-b, got %v", names)
	}
}

func TestListPodNames_EmptyNamespace(t *testing.T) {
	client := newFakeClient(nil, nil, nil)

	names, err := client.ListPodNames("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 pods, got %d", len(names))
	}
}

func TestListDeploymentNames(t *testing.T) {
	deps := []appsv1.Deployment{
		{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}},
	}

	client := newFakeClient(nil, deps, nil)

	names, err := client.ListDeploymentNames("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(names))
	}
}

func TestGetPodMetrics(t *testing.T) {
	podMetrics := []metricsv1beta1.PodMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
			Containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				{
					Name: "sidecar",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			},
		},
	}

	client := newFakeClient(nil, nil, podMetrics)

	results, err := client.GetPodMetrics("default", "my-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	rm := results[0]
	if rm.PodName != "my-pod" {
		t.Errorf("expected PodName 'my-pod', got %q", rm.PodName)
	}
	if len(rm.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(rm.Containers))
	}
	if rm.TotalCPU != 300 {
		t.Errorf("expected TotalCPU 300, got %d", rm.TotalCPU)
	}
	if rm.TotalMem != 160 {
		t.Errorf("expected TotalMem 160, got %d", rm.TotalMem)
	}

	if rm.Containers[0].Name != "app" {
		t.Errorf("expected first container 'app', got %q", rm.Containers[0].Name)
	}
	if rm.Containers[0].CPUMilli != 250 {
		t.Errorf("expected app CPU 250m, got %d", rm.Containers[0].CPUMilli)
	}
	if rm.Containers[0].MemoryMB != 128 {
		t.Errorf("expected app memory 128Mi, got %d", rm.Containers[0].MemoryMB)
	}
}

func TestGetPodMetrics_NotFound(t *testing.T) {
	client := newFakeClient(nil, nil, nil)

	_, err := client.GetPodMetrics("default", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pod metrics")
	}
}

func TestGetDeploymentMetrics(t *testing.T) {
	labels := map[string]string{"app": "web"}

	deps := []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
			},
		},
	}

	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "default", Labels: labels}},
		{ObjectMeta: metav1.ObjectMeta{Name: "web-def", Namespace: "default", Labels: labels}},
	}

	podMetrics := []metricsv1beta1.PodMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "default"},
			Containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-def", Namespace: "default"},
			Containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}

	client := newFakeClient(pods, deps, podMetrics)

	results, err := client.GetDeploymentMetrics("default", "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 pod metrics, got %d", len(results))
	}

	var totalCPU, totalMem int64
	for _, rm := range results {
		totalCPU += rm.TotalCPU
		totalMem += rm.TotalMem
	}

	if totalCPU != 300 {
		t.Errorf("expected total CPU 300, got %d", totalCPU)
	}
	if totalMem != 192 {
		t.Errorf("expected total memory 192, got %d", totalMem)
	}
}

func TestGetDeploymentMetrics_NotFound(t *testing.T) {
	client := newFakeClient(nil, nil, nil)

	_, err := client.GetDeploymentMetrics("default", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent deployment")
	}
}

func TestGetDeploymentPodNames(t *testing.T) {
	labels := map[string]string{"app": "web"}

	deps := []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
			},
		},
	}

	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "default", Labels: labels}},
		{ObjectMeta: metav1.ObjectMeta{Name: "web-def", Namespace: "default", Labels: labels}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-pod", Namespace: "default"}},
	}

	client := newFakeClient(pods, deps, nil)

	names, err := client.GetDeploymentPodNames("default", "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 pod names, got %d", len(names))
	}

	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["web-abc"] || !nameSet["web-def"] {
		t.Errorf("expected web-abc and web-def, got %v", names)
	}
}

func TestGetDeploymentPodNames_NotFound(t *testing.T) {
	client := newFakeClient(nil, nil, nil)

	_, err := client.GetDeploymentPodNames("default", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent deployment")
	}
}
