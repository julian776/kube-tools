package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/julian776/kube-tools/pkg/graph"
	"github.com/julian776/kube-tools/pkg/kube"
	promclient "github.com/julian776/kube-tools/pkg/prometheus"
	"github.com/spf13/cobra"
)

var graphPodCmd = &cobra.Command{
	Use:   "pod [name]",
	Short: "Graph resource usage for a pod",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, err := kube.NewClient(kubeContext)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		pods, err := client.ListPodNames(namespace)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return pods, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		var fetcher graph.MetricsFetcher

		if prometheusURL != "" {
			pc, err := promclient.NewClient(prometheusURL)
			if err != nil {
				return fmt.Errorf("failed to create prometheus client: %w", err)
			}
			fetcher = func(tr graph.TimeRange) ([]kube.ResourceMetrics, error) {
				dur := promclient.ParseDuration(tr.Duration)
				step := dur / 60
				if step < 15*time.Second {
					step = 15 * time.Second
				}
				return pc.QueryPodMetrics(context.Background(), namespace, name, dur, step)
			}
		} else {
			client, err := kube.NewClient(kubeContext)
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}
			fetcher = func(tr graph.TimeRange) ([]kube.ResourceMetrics, error) {
				return client.GetPodMetrics(namespace, name)
			}
		}

		return graph.RunInteractive("Pod", name, fetcher)
	},
}

func init() {
	graphCmd.AddCommand(graphPodCmd)
}
