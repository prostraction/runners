package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Runner represents a GitHub runner configuration.
type Runner struct {
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Token       string  `json:"token"`
	Labels      string  `json:"labels"`
	ContainerID string  `json:"container_id,omitempty"`
	ErrorCount  int     `json:"error_count"`
	CPULimit    float64 `json:"cpu_limit,omitempty"`    // in cores, e.g., 0.5
	MemoryLimit int64   `json:"memory_limit,omitempty"` // in MB, e.g., 512
}

// Config represents the application configuration.
type Config struct {
	Runners map[string]*Runner `json:"runners"`
}

var (
	// ConfigDir is the directory where configuration is stored.
	ConfigDir string
	// ConfigFile is the path to the configuration file.
	ConfigFile string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	ConfigDir = filepath.Join(home, ".runners")
	ConfigFile = filepath.Join(ConfigDir, "config.json")
}

// DataDir returns the persistent data directory path for a runner.
func DataDir(name string) string {
	return filepath.Join(ConfigDir, "data", name)
}

// DataDirExists reports whether a runner's persistent data directory exists.
func DataDirExists(name string) bool {
	_, err := os.Stat(DataDir(name))
	return err == nil
}

// RemoveDataDir deletes a runner's persistent data directory.
func RemoveDataDir(name string) error {
	return os.RemoveAll(DataDir(name))
}

// LoadConfig loads the configuration from disk.
func LoadConfig() (*Config, error) {
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		return &Config{Runners: make(map[string]*Runner)}, nil
	}

	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Runners == nil {
		cfg.Runners = make(map[string]*Runner)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to disk.
func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigFile, data, 0644)
}

// AddRunner adds a new runner to the configuration.
func AddRunner(runner *Runner) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Runners[runner.Name]; exists {
		return errors.New("runner with this name already exists")
	}

	cfg.Runners[runner.Name] = runner
	return SaveConfig(cfg)
}

// RemoveRunner removes a runner from the configuration.
func RemoveRunner(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Runners[name]; !exists {
		return errors.New("runner not found")
	}

	delete(cfg.Runners, name)
	return SaveConfig(cfg)
}

// UpdateRunner updates an existing runner in the configuration.
func UpdateRunner(runner *Runner) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	cfg.Runners[runner.Name] = runner
	return SaveConfig(cfg)
}
