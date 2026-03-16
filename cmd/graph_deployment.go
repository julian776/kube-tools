package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/julianalvarez/kube-tools/pkg/graph"
	"github.com/julianalvarez/kube-tools/pkg/kube"
	promclient "github.com/julianalvarez/kube-tools/pkg/prometheus"
	"github.com/spf13/cobra"
)

var graphDeploymentCmd = &cobra.Command{
	Use:   "deployment [name]",
	Short: "Graph resource usage for a deployment",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, err := kube.NewClient(kubeContext)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		deployments, err := client.ListDeploymentNames(namespace)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return deployments, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		var fetcher graph.MetricsFetcher

		if prometheusURL != "" {
			pc, err := promclient.NewClient(prometheusURL)
			if err != nil {
				return fmt.Errorf("failed to create prometheus client: %w", err)
			}

			// For deployments with Prometheus, we need to find pod names first
			kubeClient, err := kube.NewClient(kubeContext)
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			fetcher = func(tr graph.TimeRange) ([]kube.ResourceMetrics, error) {
				dur := promclient.ParseDuration(tr.Duration)
				step := dur / 60
				if step < 15*time.Second {
					step = 15 * time.Second
				}

				// Get deployment's pods via kube API, then query each from Prometheus
				depMetrics, err := kubeClient.GetDeploymentPodNames(namespace, name)
				if err != nil {
					return nil, err
				}

				var allMetrics []kube.ResourceMetrics
				for _, podName := range depMetrics {
					podMetrics, err := pc.QueryPodMetrics(context.Background(), namespace, podName, dur, step)
					if err != nil {
						continue
					}
					allMetrics = append(allMetrics, podMetrics...)
				}

				if len(allMetrics) == 0 {
					return nil, fmt.Errorf("no prometheus metrics found for deployment %s", name)
				}
				return allMetrics, nil
			}
		} else {
			client, err := kube.NewClient(kubeContext)
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}
			fetcher = func(tr graph.TimeRange) ([]kube.ResourceMetrics, error) {
				return client.GetDeploymentMetrics(namespace, name)
			}
		}

		return graph.RunInteractive("Deployment", name, fetcher)
	},
}

func init() {
	graphCmd.AddCommand(graphDeploymentCmd)
}
