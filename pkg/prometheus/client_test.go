package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryPodMetrics_ParsesMatrix(t *testing.T) {
	now := time.Now().Unix()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("query")
		var metric string
		if contains(query, "cpu") {
			metric = "container_cpu_usage_seconds_total"
		} else {
			metric = "container_memory_working_set_bytes"
		}

		resp := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "matrix",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{
							"__name__":  metric,
							"namespace": "default",
							"pod":       "my-pod",
							"container": "app",
						},
						"values": [][]interface{}{
							{float64(now - 60), "250"},
							{float64(now), "300"},
						},
					},
					{
						"metric": map[string]string{
							"__name__":  metric,
							"namespace": "default",
							"pod":       "my-pod",
							"container": "sidecar",
						},
						"values": [][]interface{}{
							{float64(now - 60), "50"},
							{float64(now), "80"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	results, err := client.QueryPodMetrics(context.Background(), "default", "my-pod", time.Hour, 30*time.Second)
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
}

func TestQueryPodMetrics_EmptyResult(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "matrix",
				"result":     []interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	results, err := client.QueryPodMetrics(context.Background(), "default", "unknown-pod", time.Hour, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results != nil {
		t.Errorf("expected nil results for unknown pod, got %v", results)
	}
}

func TestQueryPodMetrics_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"status":    "error",
			"errorType": "internal",
			"error":     "something went wrong",
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryPodMetrics(context.Background(), "default", "my-pod", time.Hour, 30*time.Second)
	if err == nil {
		t.Error("expected error on server failure")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"unknown", time.Hour}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDuration(tt.input)
			if got != tt.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseDuration_Today(t *testing.T) {
	got := ParseDuration("today")
	if got <= 0 {
		t.Errorf("ParseDuration('today') should be positive, got %v", got)
	}
	if got > 24*time.Hour {
		t.Errorf("ParseDuration('today') should be <= 24h, got %v", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
