package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSave(t *testing.T) {
	// Use a temp dir for config
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create the config dir
	configDir := filepath.Join(tmpDir, ".config", "kube-tools")
	os.MkdirAll(configDir, 0755)

	// Save a config
	cfg := &Config{
		Contexts: map[string]ContextConfig{
			"my-cluster": {
				Prometheus: PrometheusRef{
					ServiceName: "prometheus-server",
					Namespace:   "monitoring",
					Port:        9090,
				},
			},
		},
	}

	err := Save(cfg)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load it back
	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	prom, ok := loaded.GetPrometheus("my-cluster")
	if !ok {
		t.Fatal("expected prometheus config for my-cluster")
	}
	if prom.ServiceName != "prometheus-server" {
		t.Errorf("expected service name 'prometheus-server', got %q", prom.ServiceName)
	}
	if prom.Namespace != "monitoring" {
		t.Errorf("expected namespace 'monitoring', got %q", prom.Namespace)
	}
	if prom.Port != 9090 {
		t.Errorf("expected port 9090, got %d", prom.Port)
	}
}

func TestLoadSave_URL(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := &Config{Contexts: map[string]ContextConfig{}}
	cfg.SetPrometheus("prod", PrometheusRef{
		URL: "http://prometheus.example.com:9090",
	})

	err := Save(cfg)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	prom, ok := loaded.GetPrometheus("prod")
	if !ok {
		t.Fatal("expected prometheus config for prod")
	}
	if prom.URL != "http://prometheus.example.com:9090" {
		t.Errorf("expected URL 'http://prometheus.example.com:9090', got %q", prom.URL)
	}
}

func TestLoad_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Contexts) != 0 {
		t.Errorf("expected empty contexts, got %d", len(cfg.Contexts))
	}
}

func TestGetPrometheus_NotConfigured(t *testing.T) {
	cfg := &Config{Contexts: map[string]ContextConfig{}}

	_, ok := cfg.GetPrometheus("unknown")
	if ok {
		t.Error("expected false for unconfigured context")
	}
}

func TestGetPrometheus_EmptyServiceName(t *testing.T) {
	cfg := &Config{
		Contexts: map[string]ContextConfig{
			"test": {Prometheus: PrometheusRef{}},
		},
	}

	_, ok := cfg.GetPrometheus("test")
	if ok {
		t.Error("expected false for empty prometheus ref")
	}
}

func TestSetPrometheus_OverwritesExisting(t *testing.T) {
	cfg := &Config{Contexts: map[string]ContextConfig{}}

	cfg.SetPrometheus("ctx", PrometheusRef{URL: "http://old:9090"})
	cfg.SetPrometheus("ctx", PrometheusRef{URL: "http://new:9090"})

	prom, ok := cfg.GetPrometheus("ctx")
	if !ok {
		t.Fatal("expected prometheus config")
	}
	if prom.URL != "http://new:9090" {
		t.Errorf("expected new URL, got %q", prom.URL)
	}
}
