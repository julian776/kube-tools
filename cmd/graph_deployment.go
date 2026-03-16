package cmd

import (
	"fmt"

	"github.com/julianalvarez/kube-tools/pkg/graph"
	"github.com/julianalvarez/kube-tools/pkg/kube"
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

		client, err := kube.NewClient(kubeContext)
		if err != nil {
			return fmt.Errorf("failed to create kube client: %w", err)
		}

		fetcher := func(tr graph.TimeRange) ([]kube.ResourceMetrics, error) {
			return client.GetDeploymentMetrics(namespace, name)
		}

		return graph.RunInteractive("Deployment", name, fetcher)
	},
}

func init() {
	graphCmd.AddCommand(graphDeploymentCmd)
}
