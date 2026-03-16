package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PrometheusRef stores a reference to a Prometheus service in a cluster.
// Either URL is set directly, or ServiceName+Namespace+Port for port-forwarding.
type PrometheusRef struct {
	URL         string `yaml:"url,omitempty"`
	ServiceName string `yaml:"service_name,omitempty"`
	Namespace   string `yaml:"namespace,omitempty"`
	Port        int    `yaml:"port,omitempty"`
}

// ContextConfig holds configuration for a specific kube context.
type ContextConfig struct {
	Prometheus PrometheusRef `yaml:"prometheus"`
}

// Config is the top-level configuration.
type Config struct {
	Contexts map[string]ContextConfig `yaml:"contexts"`
}

// Dir returns the config directory path (~/.config/kube-tools).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".config", "kube-tools"), nil
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config from disk. Returns an empty config if the file doesn't exist.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Contexts: make(map[string]ContextConfig)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]ContextConfig)
	}
	return &cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(p, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// GetPrometheus returns the Prometheus ref for the given context, if configured.
func (c *Config) GetPrometheus(kubeCtx string) (PrometheusRef, bool) {
	ctx, ok := c.Contexts[kubeCtx]
	if !ok {
		return PrometheusRef{}, false
	}
	if ctx.Prometheus.ServiceName == "" && ctx.Prometheus.URL == "" {
		return PrometheusRef{}, false
	}
	return ctx.Prometheus, true
}

// SetPrometheus stores the Prometheus ref for the given context.
func (c *Config) SetPrometheus(kubeCtx string, ref PrometheusRef) {
	c.Contexts[kubeCtx] = ContextConfig{Prometheus: ref}
}
