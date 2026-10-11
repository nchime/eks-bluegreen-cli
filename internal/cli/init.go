package cli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"eks-bluegreen-cli/internal/config"
)

func newInitCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize eksbg configuration and check required tools",
		Long:  `Creates initial configuration file and verifies required prerequisites (kubectl, helm, aws cli, AWS authentication).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			fmt.Println("🚀 Initializing EKS Blue-Green Upgrade CLI (eksbg)...")
			fmt.Println("==================================================")

			if outputPath == "" {
				outputPath = "./configs/eksbg.yaml"
			}

			// Generate default configuration
			cfg := config.DefaultConfig()
			cfg.BlueCluster = "blue-cluster"
			cfg.GreenCluster = "green-cluster"
			cfg.TargetVersion = "1.30"
			cfg.WorkDir = "./.eksbg"

			if err := config.SaveConfig(cfg, outputPath); err != nil {
				return fmt.Errorf("failed to save config to %q: %w", outputPath, err)
			}
			absPath, _ := filepath.Abs(outputPath)
			fmt.Printf("✅ Generated initial configuration file: %s\n\n", absPath)

			fmt.Println("🔍 Checking required tools and environment:")

			// Check kubectl
			if p, err := exec.LookPath("kubectl"); err == nil {
				fmt.Printf("  [PASS] kubectl found at %s\n", p)
			} else {
				fmt.Println("  [WARN] kubectl not found in PATH")
			}

			// Check helm
			if p, err := exec.LookPath("helm"); err == nil {
				fmt.Printf("  [PASS] helm found at %s\n", p)
			} else {
				fmt.Println("  [WARN] helm not found in PATH")
			}

			// Check aws CLI
			if p, err := exec.LookPath("aws"); err == nil {
				fmt.Printf("  [PASS] aws CLI found at %s\n", p)
			} else {
				fmt.Println("  [WARN] aws CLI not found in PATH")
			}

			// Test AWS STS or Mock
			appCtx, err := InitAppContext(ctx, outputPath, mockFlag)
			if err != nil {
				fmt.Printf("  [WARN] Could not initialize state store: %v\n", err)
			} else {
				defer appCtx.Store.Close()
				acc, arn, authErr := appCtx.AWSClient.CheckCredentials(ctx)
				if authErr == nil {
					fmt.Printf("  [PASS] AWS Auth OK: Account %s, ARN %s\n", acc, arn)
				} else {
					fmt.Printf("  [INFO] Running in mock/offline mode (AWS credentials not active)\n")
				}
			}

			fmt.Println("\n✨ Initialization completed successfully.")
			fmt.Printf("Next step: Edit %s and run 'eksbg inspect --cluster <source-cluster>'\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "./configs/eksbg.yaml", "Destination path for generated config file")
	return cmd
}
