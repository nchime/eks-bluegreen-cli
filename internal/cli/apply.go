package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"eks-bluegreen-cli/internal/provisioner"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newApplyCmd() *cobra.Command {
	var planPath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Provision Green EKS cluster infrastructure based on an upgrade plan",
		Long: `Validates plan file, verifies safety guardrails, and creates the Green EKS control
plane, Managed Node Groups, and Add-ons. Supports dry-run and failure resumption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			appCtx, err := InitAppContext(ctx, cfgFile, mockFlag)
			if err != nil {
				return err
			}
			defer appCtx.Store.Close()

			if planPath == "" {
				planPath = "./plans/upgrade.yaml"
			}

			fmt.Printf("📦 Loading upgrade plan from %s (Dry-run: %v)...\n", planPath, dryRun)

			data, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("failed to read plan file %q: %w", planPath, err)
			}

			var plan models.PlanReport
			if err := yaml.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("failed to parse plan YAML: %w", err)
			}

			if !plan.Compatible && len(plan.BlockingReasons) > 0 {
				return fmt.Errorf("cannot apply plan: plan has blocking issues: %v", plan.BlockingReasons)
			}

			// Retrieve or create job
			job, err := appCtx.Store.GetJob(ctx, plan.JobID)
			if err != nil {
				job = &models.UpgradeJob{
					ID:            plan.JobID,
					SourceCluster: plan.SourceCluster,
					SourceRegion:  plan.Region,
					TargetCluster: plan.TargetCluster,
					TargetRegion:  plan.Region,
					SourceVersion: plan.SourceVersion,
					TargetVersion: plan.TargetVersion,
					CurrentPhase:  models.PhasePlanned,
					Status:        models.StatusPending,
					PlanPath:      planPath,
					PlanHash:      plan.PlanHash,
				}
				_ = appCtx.Store.CreateJob(ctx, job)
			}

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseProvisioning, models.StatusRunning, "")

			// Run provisioner
			prov := provisioner.NewProvisioner(appCtx.AWSClient, appCtx.Store)
			if err := prov.ProvisionGreenInfrastructure(ctx, job, &plan, dryRun); err != nil {
				_, _ = wf.Transition(ctx, job.ID, models.PhaseProvisioning, models.StatusFailed, err.Error())
				return fmt.Errorf("infrastructure provisioning failed: %w", err)
			}

			if dryRun {
				fmt.Println("🔎 [DRY-RUN] Green infrastructure simulation completed successfully. No resources were created.")
				return nil
			}

			_, _ = wf.Transition(ctx, job.ID, models.PhaseProvisioned, models.StatusCompleted, "")
			fmt.Printf("\n✨ Green cluster %q provisioned successfully!\n", plan.TargetCluster)
			fmt.Printf("Next step: Run 'eksbg sync --job %s' to migrate workloads to Green cluster.\n", job.ID)

			return nil
		},
	}

	cmd.Flags().StringVarP(&planPath, "plan", "p", "./plans/upgrade.yaml", "Path to the plan YAML file")

	return cmd
}
