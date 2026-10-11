package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"eks-bluegreen-cli/internal/deployer"
	"eks-bluegreen-cli/internal/inspector"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newSyncCmd() *cobra.Command {
	var jobID string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize and deploy workloads to the Green cluster",
		Long: `Deploys namespaces, services, and workloads to the provisioned Green cluster
using safe secret handling and idempotent manifests.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			appCtx, err := InitAppContext(ctx, cfgFile, mockFlag)
			if err != nil {
				return err
			}
			defer appCtx.Store.Close()

			if jobID == "" {
				return fmt.Errorf("--job flag is required")
			}

			job, err := appCtx.Store.GetJob(ctx, jobID)
			if err != nil {
				return fmt.Errorf("job %s not found: %w", jobID, err)
			}

			fmt.Printf("🔄 Synchronizing workloads for job %s (Target: %s)...\n", jobID, job.TargetCluster)

			// Load plan
			if job.PlanPath == "" {
				job.PlanPath = "./plans/upgrade.yaml"
			}
			planBytes, err := os.ReadFile(job.PlanPath)
			if err != nil {
				return fmt.Errorf("failed to read plan at %s: %w", job.PlanPath, err)
			}
			var plan models.PlanReport
			if err := yaml.Unmarshal(planBytes, &plan); err != nil {
				return fmt.Errorf("failed to parse plan: %w", err)
			}

			// Inspect source to get current workload state
			ins := inspector.NewInspector(appCtx.AWSClient, appCtx.K8sClient)
			inspection, err := ins.Inspect(ctx, job.SourceCluster)
			if err != nil {
				return fmt.Errorf("failed to inspect source workloads: %w", err)
			}

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseSyncing, models.StatusRunning, "")

			dep := deployer.NewDeployer(appCtx.K8sClient, appCtx.Store)
			if err := dep.SyncWorkloads(ctx, job.ID, inspection, &plan); err != nil {
				_, _ = wf.Transition(ctx, job.ID, models.PhaseSyncing, models.StatusFailed, err.Error())
				return fmt.Errorf("workload sync failed: %w", err)
			}

			_, _ = wf.Transition(ctx, job.ID, models.PhaseSynced, models.StatusCompleted, "")
			fmt.Printf("\n✨ Workloads synchronized successfully to %s!\n", job.TargetCluster)
			fmt.Printf("Next step: Run 'eksbg validate --job %s' to execute automated health and readiness checks.\n", job.ID)

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to synchronize")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
