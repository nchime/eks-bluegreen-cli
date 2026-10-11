package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents application configuration.
type Config struct {
	AWSProfile      string            `mapstructure:"aws_profile" json:"aws_profile" yaml:"aws_profile"`
	AWSRegion       string            `mapstructure:"aws_region" json:"aws_region" yaml:"aws_region"`
	BlueCluster     string            `mapstructure:"blue_cluster" json:"blue_cluster" yaml:"blue_cluster"`
	GreenCluster    string            `mapstructure:"green_cluster" json:"green_cluster" yaml:"green_cluster"`
	TargetVersion   string            `mapstructure:"target_version" json:"target_version" yaml:"target_version"`
	WorkDir         string            `mapstructure:"work_dir" json:"work_dir" yaml:"work_dir"`
	StateStore      string            `mapstructure:"state_store" json:"state_store" yaml:"state_store"`
	SQLitePath      string            `mapstructure:"sqlite_path" json:"sqlite_path" yaml:"sqlite_path"`
	S3Bucket        string            `mapstructure:"s3_bucket" json:"s3_bucket" yaml:"s3_bucket,omitempty"`
	TrafficMethod   string            `mapstructure:"traffic_method" json:"traffic_method" yaml:"traffic_method"`
	Route53ZoneID   string            `mapstructure:"route53_zone_id" json:"route53_zone_id" yaml:"route53_zone_id,omitempty"`
	RecordName      string            `mapstructure:"record_name" json:"record_name" yaml:"record_name,omitempty"`
	HealthCheckURLs []string          `mapstructure:"health_check_urls" json:"health_check_urls" yaml:"health_check_urls,omitempty"`
	HealthCheckSec  int               `mapstructure:"health_check_timeout_sec" json:"health_check_timeout_sec" yaml:"health_check_timeout_sec"`
	ExcludeResources []string         `mapstructure:"exclude_resources" json:"exclude_resources" yaml:"exclude_resources,omitempty"`
	DryRun          bool              `mapstructure:"dry_run" json:"dry_run" yaml:"dry_run"`
}

// DefaultConfig returns safe defaults.
func DefaultConfig() *Config {
	return &Config{
		AWSRegion:       "ap-northeast-2",
		WorkDir:         "./.eksbg",
		StateStore:      "sqlite",
		SQLitePath:      "./.eksbg/state.db",
		TrafficMethod:   "route53",
		HealthCheckSec:  10,
		HealthCheckURLs: []string{},
		DryRun:          false,
	}
}

// LoadConfig loads configuration from path or defaults.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("EKSBG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Bind known config keys
	keys := []string{
		"aws_profile", "aws_region", "blue_cluster", "green_cluster",
		"target_version", "work_dir", "state_store", "sqlite_path",
		"s3_bucket", "traffic_method", "route53_zone_id", "record_name",
		"health_check_timeout_sec", "dry_run",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}

	cfg := DefaultConfig()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %q: %w", configPath, err)
		}
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.SetConfigName("eksbg")
		v.SetConfigType("yaml")
		_ = v.ReadInConfig() // ignore if not present, defaults apply
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Ensure work directory exists
	if cfg.WorkDir != "" {
		if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create workdir %q: %w", cfg.WorkDir, err)
		}
		if cfg.SQLitePath == "" {
			cfg.SQLitePath = filepath.Join(cfg.WorkDir, "state.db")
		}
	}

	return cfg, nil
}

// SaveConfig writes the configuration to a YAML file.
func SaveConfig(cfg *Config, targetPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %q: %w", targetPath, err)
	}

	return nil
}
