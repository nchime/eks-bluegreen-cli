package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/inspector"
	"eks-bluegreen-cli/internal/reporter"
)

func newInspectCmd() *cobra.Command {
	var clusterName string
	var format string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect an existing EKS cluster and workloads",
		Long: `Analyzes source EKS cluster, node groups, add-ons, networking, and Kubernetes
workloads, identifying deprecated APIs and compatibility factors for upgrading.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			appCtx, err := InitAppContext(ctx, cfgFile, mockFlag)
			if err != nil {
				return err
			}
			defer appCtx.Store.Close()

			if clusterName == "" {
				clusterName = appCtx.Config.BlueCluster
			}
			if clusterName == "" {
				return fmt.Errorf("--cluster flag or blue_cluster in config is required")
			}

			fmt.Printf("🔍 Inspecting EKS cluster %q (Mock: %v)...\n", clusterName, appCtx.MockMode)

			ins := inspector.NewInspector(appCtx.AWSClient, appCtx.K8sClient)
			inspection, err := ins.Inspect(ctx, clusterName)
			if err != nil {
				return fmt.Errorf("inspection failed: %w", err)
			}

			var output string
			if format == "json" {
				output, err = reporter.ToJSON(inspection)
				if err != nil {
					return err
				}
			} else {
				output = reporter.GenerateInspectionMarkdown(inspection)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
					return fmt.Errorf("failed to write inspection report to %q: %w", outputFile, err)
				}
				fmt.Printf("✅ Inspection report written to %s\n", outputFile)
			} else {
				fmt.Println(output)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&clusterName, "cluster", "", "EKS cluster name to inspect")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Report output format (markdown or json)")
	cmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "Optional file path to save the inspection report")

	return cmd
}
