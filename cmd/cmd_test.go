package cmd

import (
	"bytes"
	"testing"
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
