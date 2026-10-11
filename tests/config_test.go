package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/config"
)

// Test 1: CLI and Configuration Parsing
func TestConfigLoadingAndDefaults(t *testing.T) {
	// 1. Default config
	def := config.DefaultConfig()
	assert.Equal(t, "ap-northeast-2", def.AWSRegion)
	assert.Equal(t, "./.eksbg", def.WorkDir)
	assert.Equal(t, "sqlite", def.StateStore)
	assert.False(t, def.DryRun)

	// 2. Custom file config
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "eksbg.yaml")

	customCfg := &config.Config{
		AWSProfile:      "prod-deployer",
		AWSRegion:       "us-west-2",
		BlueCluster:     "prod-blue-cluster",
		GreenCluster:    "prod-green-cluster",
		TargetVersion:   "1.30",
		WorkDir:         filepath.Join(tmpDir, ".eksbg"),
		StateStore:      "sqlite",
		SQLitePath:      filepath.Join(tmpDir, ".eksbg", "state.db"),
		TrafficMethod:   "route53",
		Route53ZoneID:   "Z123456789",
		RecordName:      "api.mycompany.com",
		HealthCheckURLs: []string{"https://api.mycompany.com/healthz"},
		HealthCheckSec:  15,
		DryRun:          true,
	}

	err := config.SaveConfig(customCfg, cfgPath)
	require.NoError(t, err)

	loaded, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "prod-deployer", loaded.AWSProfile)
	assert.Equal(t, "us-west-2", loaded.AWSRegion)
	assert.Equal(t, "prod-blue-cluster", loaded.BlueCluster)
	assert.Equal(t, "prod-green-cluster", loaded.GreenCluster)
	assert.Equal(t, "1.30", loaded.TargetVersion)
	assert.True(t, loaded.DryRun)
	assert.Equal(t, 15, loaded.HealthCheckSec)
	assert.Contains(t, loaded.HealthCheckURLs, "https://api.mycompany.com/healthz")
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	os.Setenv("EKSBG_AWS_REGION", "eu-central-1")
	os.Setenv("EKSBG_BLUE_CLUSTER", "blue-from-env")
	defer func() {
		os.Unsetenv("EKSBG_AWS_REGION")
		os.Unsetenv("EKSBG_BLUE_CLUSTER")
	}()

	loaded, err := config.LoadConfig("")
	require.NoError(t, err)

	assert.Equal(t, "eu-central-1", loaded.AWSRegion)
	assert.Equal(t, "blue-from-env", loaded.BlueCluster)
}
