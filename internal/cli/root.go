package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	region    string
	mockFlag  bool
	dryRun    bool
	verbose   bool
)

// NewRootCmd creates the root Cobra command for eksbg.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "eksbg",
		Short: "EKS Blue-Green Upgrade CLI",
		Long: `eksbg is an open-source CLI for automated, zero-downtime Blue-Green
Kubernetes version upgrades on Amazon Web Services (EKS).

It provides inspection, planning, provisioning, workload sync, automated validation,
guarded approval workflows, traffic cutover, and emergency rollback.`,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Path to config file (default: ./configs/eksbg.yaml)")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS Region (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&mockFlag, "mock", false, "Run with mock AWS and Kubernetes clients (for offline demo/testing)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate operations without making any cloud infrastructure changes")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose debug output")

	// Register subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newInspectCmd())
	rootCmd.AddCommand(newPlanCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newApproveCmd())
	rootCmd.AddCommand(newCutoverCmd())
	rootCmd.AddCommand(newRecoverCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newReportCmd())

	return rootCmd
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
