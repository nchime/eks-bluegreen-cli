package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/reporter"
	"eks-bluegreen-cli/internal/validator"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newValidateCmd() *cobra.Command {
	var jobID string
	var healthURLs []string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Green cluster, workloads, endpoints, and healthchecks",
		Long: `Performs comprehensive validation of the Green environment:
EKS Control Plane, worker nodes, add-ons, pod readiness probes, service endpoints,
ingress, and application HTTP health endpoints.`,
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

			fmt.Printf("🔍 Running automated validation for job %s (Target: %s)...\n", jobID, job.TargetCluster)

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseValidating, models.StatusRunning, "")

			allURLs := append(appCtx.Config.HealthCheckURLs, healthURLs...)
			v := validator.NewValidator(appCtx.AWSClient, appCtx.K8sClient, appCtx.Store)
			results, valHash, err := v.Validate(ctx, job, allURLs, appCtx.Config.HealthCheckSec)
			if err != nil {
				_, _ = wf.Transition(ctx, job.ID, models.PhaseValidating, models.StatusFailed, err.Error())
				return fmt.Errorf("validation execution error: %w", err)
			}

			// Render report
			fmt.Println("\n" + reporter.GenerateValidationMarkdown(job.ID, results))
			fmt.Printf("Validation Hash: %s\n\n", valHash)

			hasCrit := validator.HasCriticalFailures(results)
			if hasCrit {
				_, _ = wf.Transition(ctx, job.ID, models.PhaseValidating, models.StatusFailed, "One or more critical validation checks failed")
				fmt.Println("❌ Critical validation checks failed! Traffic cutover is blocked until resolved.")
				return fmt.Errorf("validation failed: critical checks did not pass")
			}

			_, _ = wf.Transition(ctx, job.ID, models.PhaseValidated, models.StatusCompleted, "")
			fmt.Println("✅ All essential validations passed successfully!")
			fmt.Printf("Next step: Run 'eksbg approve-cutover --job %s' to review and confirm cutover.\n", job.ID)

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to validate")
	cmd.Flags().StringSliceVar(&healthURLs, "url", []string{}, "Optional external HTTP health check URL(s) to test")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
