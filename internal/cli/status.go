package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var jobID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display status and execution steps of an upgrade job",
		Long:  `Displays current phase, health status, and step-by-step history of an upgrade job, or lists all jobs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			appCtx, err := InitAppContext(ctx, cfgFile, mockFlag)
			if err != nil {
				return err
			}
			defer appCtx.Store.Close()

			if jobID == "" {
				// List all jobs
				jobs, err := appCtx.Store.ListJobs(ctx)
				if err != nil {
					return fmt.Errorf("failed to list jobs: %w", err)
				}
				if len(jobs) == 0 {
					fmt.Println("No upgrade jobs found in state store.")
					return nil
				}
				fmt.Println("📋 EKS Blue-Green Upgrade Jobs:")
				fmt.Println("---------------------------------------------------------------------------------------------")
				fmt.Printf("%-12s | %-16s | %-16s | %-16s | %-10s | %s\n", "JOB ID", "SOURCE", "TARGET", "CURRENT PHASE", "STATUS", "CREATED")
				fmt.Println("---------------------------------------------------------------------------------------------")
				for _, j := range jobs {
					fmt.Printf("%-12s | %-16s | %-16s | %-16s | %-10s | %s\n",
						j.ID, j.SourceCluster, j.TargetCluster, j.CurrentPhase, j.Status, j.CreatedAt.Format("2006-01-02 15:04"))
				}
				fmt.Println("---------------------------------------------------------------------------------------------")
				fmt.Println("To view detailed status for a job, run: eksbg status --job <job-id>")
				return nil
			}

			job, err := appCtx.Store.GetJob(ctx, jobID)
			if err != nil {
				return fmt.Errorf("job %s not found: %w", jobID, err)
			}

			fmt.Println("===================================================================")
			fmt.Printf("📊 Job Status: %s\n", job.ID)
			fmt.Println("===================================================================")
			fmt.Printf("Source Cluster:  %s (v%s, %s)\n", job.SourceCluster, job.SourceVersion, job.SourceRegion)
			fmt.Printf("Target Cluster:  %s (v%s, %s)\n", job.TargetCluster, job.TargetVersion, job.TargetRegion)
			fmt.Printf("Current Phase:   %s\n", job.CurrentPhase)
			fmt.Printf("Status:          %s\n", job.Status)
			if job.LastError != "" {
				fmt.Printf("Last Error:      %s\n", job.LastError)
			}
			fmt.Printf("Created At:      %s\n", job.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Updated At:      %s\n", job.UpdatedAt.Format(time.RFC3339))

			steps, err := appCtx.Store.GetExecutionSteps(ctx, jobID)
			if err == nil && len(steps) > 0 {
				fmt.Println("\nExecution Steps:")
				fmt.Println("-------------------------------------------------------------------")
				for i, s := range steps {
					dur := "-"
					if s.CompletedAt != nil {
						dur = s.CompletedAt.Sub(s.StartedAt).Round(time.Millisecond).String()
					}
					fmt.Printf(" [%d] %-30s | %-9s | %s\n", i+1, s.StepName, s.Status, dur)
					if s.ErrorMessage != "" {
						fmt.Printf("     ❌ Error: %s\n", s.ErrorMessage)
					}
					if s.OutputReference != "" {
						fmt.Printf("     ℹ️ Output: %s\n", s.OutputReference)
					}
				}
				fmt.Println("-------------------------------------------------------------------")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Optional Job ID to query detailed status")

	return cmd
}
