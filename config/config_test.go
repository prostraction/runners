package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigFlow(t *testing.T) {
	// Setup temporary config directory
	tmpDir, err := os.MkdirTemp("", "runners-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ConfigDir = tmpDir
	ConfigFile = filepath.Join(tmpDir, "config.json")

	// 1. Test LoadConfig on empty
	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("expected no error on empty config, got %v", err)
	}
	if len(cfg.Runners) != 0 {
		t.Errorf("expected 0 runners, got %d", len(cfg.Runners))
	}

	// 2. Test AddRunner
	runner := &Runner{
		Name:  "test-runner",
		URL:   "https://github.com/test/repo",
		Token: "test-token",
	}
	err = AddRunner(runner)
	if err != nil {
		t.Errorf("expected no error adding runner, got %v", err)
	}

	// 3. Test Add duplicate
	err = AddRunner(runner)
	if err == nil {
		t.Errorf("expected error when adding duplicate runner, got nil")
	}

	// 4. Test LoadConfig after add
	cfg, err = LoadConfig()
	if err != nil {
		t.Errorf("expected no error loading config, got %v", err)
	}
	if _, exists := cfg.Runners["test-runner"]; !exists {
		t.Errorf("expected 'test-runner' to exist in config")
	}

	// 5. Test UpdateRunner
	runner.Labels = "new-label"
	err = UpdateRunner(runner)
	if err != nil {
		t.Errorf("expected no error updating runner, got %v", err)
	}

	cfg, _ = LoadConfig()
	if cfg.Runners["test-runner"].Labels != "new-label" {
		t.Errorf("expected label to be 'new-label', got '%s'", cfg.Runners["test-runner"].Labels)
	}

	// 6. Test RemoveRunner
	err = RemoveRunner("test-runner")
	if err != nil {
		t.Errorf("expected no error removing runner, got %v", err)
	}

	cfg, _ = LoadConfig()
	if _, exists := cfg.Runners["test-runner"]; exists {
		t.Errorf("expected 'test-runner' to be removed")
	}

	// 7. Test Remove non-existent
	err = RemoveRunner("none")
	if err == nil {
		t.Errorf("expected error removing non-existent runner, got nil")
	}

	// 8. Test LoadConfig with invalid JSON
	if err := os.WriteFile(ConfigFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}
	_, err = LoadConfig()
	if err == nil {
		t.Error("expected error loading invalid JSON, got nil")
	}

	// 9. Test LoadConfig with non-readable file
	if err := os.Chmod(ConfigFile, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	_, err = LoadConfig()
	if err == nil {
		t.Error("expected error loading non-readable file, got nil")
	}
	if err := os.Chmod(ConfigFile, 0644); err != nil { // restore for cleanup
		t.Fatalf("failed to restore chmod: %v", err)
	}

	// 10. Test LoadConfig with nil runners map in JSON
	if err := os.WriteFile(ConfigFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write empty json: %v", err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Errorf("expected no error with empty object, got %v", err)
	}
	if cfg.Runners == nil {
		t.Error("expected Runners map to be initialized, got nil")
	}
}

func TestSaveConfigError(t *testing.T) {
	// Try to save to a file that is actually a directory (will fail on WriteFile)
	tmpDir, _ := os.MkdirTemp("", "runners-fail")
	defer os.RemoveAll(tmpDir)
	
	ConfigDir = tmpDir
	ConfigFile = tmpDir // ConfigFile points to a directory
	
	err := SaveConfig(&Config{})
	if err == nil {
		t.Error("expected error saving when ConfigFile is a directory, got nil")
	}
}
