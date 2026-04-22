package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/pflag"
)

func TestRootCommand(t *testing.T) {
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"--help"})
	
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() failed: %v", err)
	}
	
	if !bytes.Contains(b.Bytes(), []byte("Runners is a CLI tool")) {
		t.Errorf("expected help text, got: %s", b.String())
	}
}

func TestAddCommandHelp(t *testing.T) {
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"add", "--help"})
	
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("add command help failed: %v", err)
	}
	
	if !bytes.Contains(b.Bytes(), []byte("--token")) {
		t.Errorf("expected add help flags, got: %s", b.String())
	}
}

func TestMemoryAliasesNormalize(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	var mem int64
	fs.Int64Var(&mem, "memory", 0, "")
	fs.SetNormalizeFunc(normalizeMemoryAliases)

	// --mem should map to --memory
	if err := fs.Parse([]string{"--mem", "512"}); err != nil {
		t.Fatalf("--mem failed to parse: %v", err)
	}
	if mem != 512 {
		t.Errorf("expected 512 via --mem, got %d", mem)
	}

	// --ram should map to --memory
	mem = 0
	if err := fs.Parse([]string{"--ram", "1024"}); err != nil {
		t.Fatalf("--ram failed to parse: %v", err)
	}
	if mem != 1024 {
		t.Errorf("expected 1024 via --ram, got %d", mem)
	}

	// Changed("memory") should be true even when user typed --mem.
	if !fs.Changed("memory") {
		t.Error("expected Changed(\"memory\") to be true after --ram")
	}
}

func TestLogCommandHelp(t *testing.T) {
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"log", "--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("log command help failed: %v", err)
	}
	if !bytes.Contains(b.Bytes(), []byte("--follow")) {
		t.Errorf("expected --follow flag in help, got: %s", b.String())
	}
	if !bytes.Contains(b.Bytes(), []byte("--tail")) {
		t.Errorf("expected --tail flag in help, got: %s", b.String())
	}
}

func TestListCommandOutput(t *testing.T) {
	// List command requires a mock config to not fail on file reading
	// For now we just check if it executes without panic when help is requested
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"list", "--help"})
	
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list command help failed: %v", err)
	}
}
