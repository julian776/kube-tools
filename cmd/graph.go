package cmd

import (
	"github.com/spf13/cobra"
)

var namespace string

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize resource usage for Kubernetes objects",
	Long:  "Display CPU and memory usage graphs for pods, deployments, and other Kubernetes resources.",
}

func init() {
	graphCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	rootCmd.AddCommand(graphCmd)
}
