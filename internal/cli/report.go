package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/reporter"
)

func newReportCmd() *cobra.Command {
	var jobID string
	var format string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a full audit and status report for an upgrade job",
		Long:  `Outputs complete Markdown or JSON report covering job metadata, approval records, validation results, and step logs.`,
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

			steps, _ := appCtx.Store.GetExecutionSteps(ctx, jobID)
			valResults, _ := appCtx.Store.GetValidationResults(ctx, jobID)
			appRecord, _ := appCtx.Store.GetLatestApproval(ctx, jobID)

			var output string
			if format == "json" {
				type fullReportData struct {
					Job        interface{} `json:"job"`
					Approval   interface{} `json:"approval,omitempty"`
					Steps      interface{} `json:"steps"`
					Validation interface{} `json:"validation"`
				}
				data := fullReportData{
					Job:        job,
					Approval:   appRecord,
					Steps:      steps,
					Validation: valResults,
				}
				output, err = reporter.ToJSON(data)
				if err != nil {
					return err
				}
			} else {
				output = reporter.GenerateFullReportMarkdown(job, steps, valResults, appRecord)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
					return fmt.Errorf("failed to save report to %s: %w", outputFile, err)
				}
				fmt.Printf("✅ Full report written to %s\n", outputFile)
			} else {
				fmt.Println(output)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job", "", "Job ID to generate report for")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Report format (markdown or json)")
	cmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "Destination file path for report")
	_ = cmd.MarkFlagRequired("job")

	return cmd
}
