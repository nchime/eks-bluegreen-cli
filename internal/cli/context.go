package cli

import (
	"context"
	"os"
	"path/filepath"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/config"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/store"
)

// AppContext carries shared runtime resources for commands.
type AppContext struct {
	Config    *config.Config
	Store     store.Store
	AWSClient aws.Client
	K8sClient k8s.K8sClient
	MockMode  bool
}

// InitAppContext initializes configuration, state store, and clients.
func InitAppContext(ctx context.Context, configPath string, forceMock bool) (*AppContext, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	st, err := store.NewSQLiteStore(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}

	appCtx := &AppContext{
		Config:   cfg,
		Store:    st,
		MockMode: forceMock,
	}

	// Determine if Mock should be used (either explicit flag, env var, or AWS creds missing)
	if forceMock || os.Getenv("EKSBG_MOCK") == "true" {
		mockAWS := aws.NewMockAWSClient()
		mockAWS.EnableFilePersistence(filepath.Join(cfg.WorkDir, "mock_aws_state.json"))
		appCtx.AWSClient = mockAWS
		appCtx.K8sClient = k8s.NewMockK8sClient()
		appCtx.MockMode = true
		return appCtx, nil
	}

	// Try initializing real AWS client
	realAWS, err := aws.NewRealAWSClient(ctx, cfg.AWSRegion, cfg.AWSProfile)
	if err != nil {
		// Fallback to mock with warning
		appCtx.AWSClient = aws.NewMockAWSClient()
		appCtx.K8sClient = k8s.NewMockK8sClient()
		appCtx.MockMode = true
		return appCtx, nil
	}

	// Verify STS credentials
	if _, _, err := realAWS.CheckCredentials(ctx); err != nil {
		// Fallback to mock if authentication fails
		appCtx.AWSClient = aws.NewMockAWSClient()
		appCtx.K8sClient = k8s.NewMockK8sClient()
		appCtx.MockMode = true
		return appCtx, nil
	}

	appCtx.AWSClient = realAWS
	appCtx.K8sClient = k8s.NewMockK8sClient() // Default mock k8s client unless configured
	return appCtx, nil
}
