package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/recovery"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newRecoverCmd() *cobra.Command {
	var jobID string
	var zoneID string
	var recordName string
	var autoConfirm bool

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Emergency rollback of traffic from Green back to Blue cluster",
		Long: `Reverts traffic routing from Green back to the Blue cluster.
Displays warnings regarding database state and in-flight traffic before switching back.`,
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

			fmt.Println("⚠️  EMERGENCY TRAFFIC ROLLBACK INITIATION")
			fmt.Println("===================================================================")
			fmt.Printf("Job:                  %s\n", job.ID)
			fmt.Printf("Reverting from Green: %s\n", job.TargetCluster)
			fmt.Printf("Reverting back to Blue: %s\n", job.SourceCluster)
			fmt.Println("-------------------------------------------------------------------")
			fmt.Println("⚠️  DATA CONSISTENCY WARNING:")
			fmt.Println("  Traffic will return to the Blue cluster immediately.")
			fmt.Println("  Any data or database writes accepted by the Green cluster since cutover")
			fmt.Println("  must be reconciled manually. Automated database rollback is not provided.")
			fmt.Println("===================================================================")

			if !autoConfirm {
				fmt.Printf("Are you sure you want to rollback traffic to Blue cluster %s? (yes/no): ", job.SourceCluster)
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "yes" && input != "y" {
					fmt.Println("❌ Rollback cancelled.")
					return nil
				}
			}

			rec := recovery.NewRecoverer(appCtx.AWSClient, appCtx.Store)
			if err := rec.RecoverTraffic(ctx, jobID, zoneID, recordName); err != nil {
				return fmt.Errorf("traffic recovery failed: %w", err)
			}

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseRecovered, models.StatusCompleted, "")

			fmt.Printf("\n✅ Emergency rollback complete. Traffic successfully reverted to Blue cluster %s!\n", job.SourceCluster)

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to rollback")
	cmd.Flags().StringVar(&zoneID, "zone-id", "", "Route53 Hosted Zone ID")
	cmd.Flags().StringVar(&recordName, "record-name", "", "DNS Record name")
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "Confirm rollback without interactive prompt")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
