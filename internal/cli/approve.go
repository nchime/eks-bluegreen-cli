package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/approval"
	"eks-bluegreen-cli/internal/reporter"
	"eks-bluegreen-cli/internal/workflow"
	"eks-bluegreen-cli/pkg/models"
)

func newApproveCmd() *cobra.Command {
	var jobID string
	var approver string
	var comments string
	var autoConfirm bool

	cmd := &cobra.Command{
		Use:   "approve-cutover",
		Short: "Review validation findings and grant explicit approval for cutover",
		Long: `Displays validation results and Blue vs Green endpoints, prompts the user for
explicit authorization, and records an immutable cryptographic audit record.`,
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
			if approver == "" {
				approver = os.Getenv("USER")
				if approver == "" {
					approver = "operator"
				}
			}

			job, err := appCtx.Store.GetJob(ctx, jobID)
			if err != nil {
				return fmt.Errorf("job %s not found: %w", jobID, err)
			}

			// Show validation summary
			valResults, err := appCtx.Store.GetValidationResults(ctx, jobID)
			if err != nil || len(valResults) == 0 {
				return fmt.Errorf("no validation results found. Run 'eksbg validate --job %s' first", jobID)
			}

			fmt.Println("\n" + reporter.GenerateValidationMarkdown(jobID, valResults))

			// Show Blue vs Green Endpoints
			blueInfo, _ := appCtx.AWSClient.DescribeCluster(ctx, job.SourceCluster)
			greenInfo, _ := appCtx.AWSClient.DescribeCluster(ctx, job.TargetCluster)

			fmt.Println("-------------------------------------------------------------------")
			fmt.Printf("🔵 Source (Blue)  Endpoint: %s\n", blueInfo.Endpoint)
			fmt.Printf("🟢 Target (Green) Endpoint: %s\n", greenInfo.Endpoint)
			fmt.Println("-------------------------------------------------------------------")
			fmt.Println("⚠️  SAFETY NOTICE:")
			fmt.Println("  1. Blue cluster will NOT be deleted or modified.")
			fmt.Println("  2. Cutover will shift DNS traffic to Green cluster.")
			fmt.Println("  3. Rollback is available via 'eksbg recover' if issues occur.")
			fmt.Println("-------------------------------------------------------------------")

			if !autoConfirm {
				fmt.Printf("Do you explicitly approve traffic cutover for job %s? (yes/no): ", jobID)
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "yes" && input != "y" {
					fmt.Println("❌ Cutover approval cancelled by user.")
					return nil
				}
			}

			mgr := approval.NewManager(appCtx.Store)
			rec, err := mgr.Approve(ctx, jobID, approver, comments, job.PlanHash)
			if err != nil {
				return fmt.Errorf("approval rejected: %w", err)
			}

			wf := workflow.NewEngine(appCtx.Store)
			_, _ = wf.Transition(ctx, job.ID, models.PhaseApproved, models.StatusCompleted, "")

			fmt.Printf("\n✅ Cutover APPROVED by %s at %s\n", rec.Approver, rec.ApprovedAt.Format("2006-01-02 15:04:05 UTC"))
			fmt.Printf("Plan Hash:       %s\n", rec.PlanHash)
			fmt.Printf("Validation Hash: %s\n", rec.ValidationHash)
			fmt.Printf("\nNext step: Run 'eksbg cutover --job %s' to execute the traffic switch.\n", job.ID)

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to approve")
	cmd.Flags().StringVar(&approver, "approver", "", "Name or email of the approver (defaults to current user)")
	cmd.Flags().StringVar(&comments, "comments", "Approved after validation review", "Optional approval notes")
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "Auto-confirm approval without interactive prompt")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
