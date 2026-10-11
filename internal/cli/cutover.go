package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/approval"
	"eks-bluegreen-cli/internal/cutover"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newCutoverCmd() *cobra.Command {
	var jobID string
	var zoneID string
	var recordName string

	cmd := &cobra.Command{
		Use:   "cutover",
		Short: "Switch production traffic to the Green cluster",
		Long: `Executes the traffic cutover to the Green cluster after strictly verifying
valid cryptographic approval and validation state. Retains the Blue cluster intact.`,
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

			if zoneID == "" {
				zoneID = appCtx.Config.Route53ZoneID
			}
			if recordName == "" {
				recordName = appCtx.Config.RecordName
			}

			fmt.Printf("⚡ Executing traffic cutover for job %s...\n", jobID)

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseCutoverInProgress, models.StatusRunning, "")

			appMgr := approval.NewManager(appCtx.Store)
			op := cutover.NewOperator(appCtx.AWSClient, appMgr, appCtx.Store)

			if err := op.ExecuteCutover(ctx, jobID, zoneID, recordName, job.PlanHash, ""); err != nil {
				_, _ = wf.Transition(ctx, job.ID, models.PhaseCutoverInProgress, models.StatusFailed, err.Error())
				return fmt.Errorf("cutover execution failed: %w", err)
			}

			_, _ = wf.Transition(ctx, job.ID, models.PhaseCutoverCompleted, models.StatusCompleted, "")
			fmt.Printf("\n🎉 Traffic cutover completed successfully for %s!\n", job.TargetCluster)
			fmt.Println("🛡️  SAFETY NOTE: The Blue cluster remains active and untouched.")
			fmt.Println("If you detect unexpected production errors, run 'eksbg recover' immediately to switch traffic back.")

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to execute cutover for")
	cmd.Flags().StringVar(&zoneID, "zone-id", "", "Route53 Hosted Zone ID (overrides config)")
	cmd.Flags().StringVar(&recordName, "record-name", "", "DNS Record name (e.g. app.example.com)")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
