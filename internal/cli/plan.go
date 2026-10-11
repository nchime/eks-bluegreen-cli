package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"eks-bluegreen-cli/internal/inspector"
	"eks-bluegreen-cli/internal/planner"
	"eks-bluegreen-cli/internal/reporter"
	"eks-bluegreen-cli/pkg/models"
)

func newPlanCmd() *cobra.Command {
	var sourceCluster string
	var targetCluster string
	var targetVersion string
	var planOutput string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate a Blue-Green upgrade and migration plan",
		Long: `Compares source cluster configuration against target EKS version, verifies
compatibility, checks deprecated APIs, classifies replicated resources, and outputs a plan file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			appCtx, err := InitAppContext(ctx, cfgFile, mockFlag)
			if err != nil {
				return err
			}
			defer appCtx.Store.Close()

			if sourceCluster == "" {
				sourceCluster = appCtx.Config.BlueCluster
			}
			if targetCluster == "" {
				targetCluster = appCtx.Config.GreenCluster
				if targetCluster == "" {
					targetCluster = sourceCluster + "-green"
				}
			}
			if targetVersion == "" {
				targetVersion = appCtx.Config.TargetVersion
			}
			if planOutput == "" {
				planOutput = "./plans/upgrade.yaml"
			}

			if sourceCluster == "" || targetVersion == "" {
				return fmt.Errorf("source cluster and target version are required (via flags or config)")
			}

			reg := appCtx.Config.AWSRegion
			if region != "" {
				reg = region
			}

			fmt.Printf("📋 Planning upgrade from %q to target EKS v%s (%q) in %s...\n",
				sourceCluster, targetVersion, targetCluster, reg)

			// 1. Inspect source cluster
			ins := inspector.NewInspector(appCtx.AWSClient, appCtx.K8sClient)
			inspection, err := ins.Inspect(ctx, sourceCluster)
			if err != nil {
				return fmt.Errorf("failed to inspect source cluster: %w", err)
			}

			// 2. Generate Plan
			jobID := "job-" + uuid.New().String()[:8]
			p := planner.NewPlanner(appCtx.AWSClient)
			planReport, err := p.GeneratePlan(ctx, jobID, inspection, targetCluster, targetVersion, reg)
			if err != nil {
				return fmt.Errorf("planning failed: %w", err)
			}

			// 3. Save Plan file (YAML)
			dir := filepath.Dir(planOutput)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create plan directory: %w", err)
			}

			planBytes, err := yaml.Marshal(planReport)
			if err != nil {
				return fmt.Errorf("failed to serialize plan: %w", err)
			}
			if err := os.WriteFile(planOutput, planBytes, 0644); err != nil {
				return fmt.Errorf("failed to write plan to %s: %w", planOutput, err)
			}

			// 4. Save Job in state store
			job := &models.UpgradeJob{
				ID:            jobID,
				SourceCluster: sourceCluster,
				SourceRegion:  reg,
				TargetCluster: targetCluster,
				TargetRegion:  reg,
				SourceVersion: inspection.Cluster.Version,
				TargetVersion: targetVersion,
				CurrentPhase:  models.PhasePlanned,
				Status:        models.StatusCompleted,
				PlanPath:      planOutput,
				PlanHash:      planReport.PlanHash,
			}
			if err := appCtx.Store.CreateJob(ctx, job); err != nil {
				return fmt.Errorf("failed to record job in store: %w", err)
			}
			if err := appCtx.Store.SaveResourcePlans(ctx, jobID, planReport.ResourcePlans); err != nil {
				return fmt.Errorf("failed to save resource plans: %w", err)
			}

			// 5. Output Summary Markdown
			fmt.Println("\n" + reporter.GeneratePlanMarkdown(planReport))
			fmt.Printf("✅ Upgrade plan saved to %s\n", planOutput)
			fmt.Printf("🔑 Job ID: %s | Plan Hash: %s\n", jobID, planReport.PlanHash)
			fmt.Printf("Next step: Review the plan and run 'eksbg apply --plan %s'\n", planOutput)

			return nil
		},
	}

	cmd.Flags().StringVarP(&sourceCluster, "source", "s", "", "Source EKS cluster name (Blue)")
	cmd.Flags().StringVarP(&targetCluster, "target-cluster", "t", "", "Target Green cluster name (default: <source>-green)")
	cmd.Flags().StringVar(&targetVersion, "target-version", "", "Target Kubernetes version (e.g. 1.30)")
	cmd.Flags().StringVarP(&planOutput, "output-plan", "o", "./plans/upgrade.yaml", "Destination path for generated plan YAML")

	return cmd
}
