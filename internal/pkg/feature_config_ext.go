package pkg

import (
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

type Config struct {
	ConfigFile         string
	CommandTimeout     time.Duration
	RebootRequiredPath string
	StatusDatabasePath string
	APTListsPath       string
}

const (
	DefaultRefreshInterval    = time.Minute
	DefaultCommandTimeout     = 15 * time.Second
	DefaultRebootRequiredPath = "/run/reboot-required"
	DefaultStatusDatabasePath = "/var/lib/dpkg/status"
	DefaultAPTListsPath       = "/var/lib/apt/lists"
)

var DefaultFeatureConfigFileName = "prometheus-pkg-exporter.yml"

type featureConfigFile struct {
	CommandTimeout     string `yaml:"command_timeout"`
	RebootRequiredPath string `yaml:"reboot_required_path"`
	StatusDatabasePath string `yaml:"status_database_path"`
	APTListsPath       string `yaml:"apt_lists_path"`
}

var featureConfigFlagSpecs = []featurekit.FeatureConfigFlagSpec[Config]{
	{
		Name:    "command-timeout",
		Help:    "Timeout for reading local package state",
		Default: DefaultCommandTimeout.String(),
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.DurationVar(&config.CommandTimeout)
		},
	},
	{
		Name:    "reboot-required-path",
		Help:    "Path to the reboot-required marker file",
		Default: DefaultRebootRequiredPath,
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringVar(&config.RebootRequiredPath)
		},
	},
	{
		Name:    "status-database-path",
		Help:    "Path to the dpkg status database",
		Default: DefaultStatusDatabasePath,
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringVar(&config.StatusDatabasePath)
		},
	},
	{
		Name:    "apt-lists-path",
		Help:    "Path to the apt package index directory",
		Default: DefaultAPTListsPath,
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringVar(&config.APTListsPath)
		},
	},
}

func NewDefaultConfig() Config {
	return Config{
		CommandTimeout:     DefaultCommandTimeout,
		RebootRequiredPath: DefaultRebootRequiredPath,
		StatusDatabasePath: DefaultStatusDatabasePath,
		APTListsPath:       DefaultAPTListsPath,
	}
}

func FeatureConfigFile(config *Config) *string {
	return &config.ConfigFile
}

func ValidateFeatureConfig(_ Config) error {
	return nil
}

func FeatureRuntimeConfigEntries(_ featurekit.RuntimeConfigContext[Config], config Config) []any {
	return []any{
		"command_timeout", framework.NormalizeDuration(config.CommandTimeout, DefaultCommandTimeout),
		"reboot_required_path", normalizeString(config.RebootRequiredPath, DefaultRebootRequiredPath),
		"status_database_path", normalizeString(config.StatusDatabasePath, DefaultStatusDatabasePath),
		"apt_lists_path", normalizeString(config.APTListsPath, DefaultAPTListsPath),
	}
}

func normalizeString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func ResolveFeatureConfig(featureName string, config Config) (Config, string, bool, error) {
	var fileConfig featureConfigFile
	cfgFile, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	if err != nil {
		return config, cfgFile, false, err
	}
	if !loaded {
		return config, cfgFile, false, nil
	}

	if fileConfig.CommandTimeout != "" && config.CommandTimeout == DefaultCommandTimeout {
		commandTimeout, err := time.ParseDuration(fileConfig.CommandTimeout)
		if err != nil {
			return config, cfgFile, true, fmt.Errorf("parse command_timeout from %q: %w", cfgFile, err)
		}
		config.CommandTimeout = commandTimeout
	}
	if fileConfig.RebootRequiredPath != "" && config.RebootRequiredPath == DefaultRebootRequiredPath {
		config.RebootRequiredPath = fileConfig.RebootRequiredPath
	}
	if fileConfig.StatusDatabasePath != "" && config.StatusDatabasePath == DefaultStatusDatabasePath {
		config.StatusDatabasePath = fileConfig.StatusDatabasePath
	}
	if fileConfig.APTListsPath != "" && config.APTListsPath == DefaultAPTListsPath {
		config.APTListsPath = fileConfig.APTListsPath
	}
	return config, cfgFile, true, nil
}
