package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Runner struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Token       string `json:"token"`
	Labels      string `json:"labels"`
	ContainerID string `json:"container_id,omitempty"`
	ErrorCount  int    `json:"error_count"`
	CPULimit    float64 `json:"cpu_limit,omitempty"`    // in cores, e.g., 0.5
	MemoryLimit int64   `json:"memory_limit,omitempty"` // in MB, e.g., 512
}

type Config struct {
	Runners map[string]*Runner `json:"runners"`
}

var (
	ConfigDir  string
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

func UpdateRunner(runner *Runner) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	cfg.Runners[runner.Name] = runner
	return SaveConfig(cfg)
}
