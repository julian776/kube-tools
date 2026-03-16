package cmd

import (
	"github.com/spf13/cobra"
)

var (
	namespace     string
	prometheusURL string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize resource usage for Kubernetes objects",
	Long:  "Display CPU and memory usage graphs for pods, deployments, and other Kubernetes resources.",
}

func init() {
	graphCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	graphCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "", "Prometheus server URL for historical metrics (e.g. http://localhost:9090)")
	rootCmd.AddCommand(graphCmd)
}
