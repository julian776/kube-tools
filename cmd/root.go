package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var kubeContext string

var rootCmd = &cobra.Command{
	Use:   "kube-tools",
	Short: "A CLI toolkit for Kubernetes operations",
	Long:  "kube-tools provides utilities for visualizing and managing Kubernetes resources.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "Kubernetes context to use")

	// Register built-in completion command (supports bash, zsh, fish, powershell)
	rootCmd.CompletionOptions.HiddenDefaultCmd = false
}
