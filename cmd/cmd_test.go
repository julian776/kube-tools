package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}
}

func TestGraphCommand_Exists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "graph" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'graph' subcommand to be registered")
	}
}

func TestGraphSubcommands(t *testing.T) {
	subcommands := map[string]bool{
		"pod":        false,
		"deployment": false,
	}

	for _, cmd := range graphCmd.Commands() {
		if _, ok := subcommands[cmd.Name()]; ok {
			subcommands[cmd.Name()] = true
		}
	}

	for name, found := range subcommands {
		if !found {
			t.Errorf("expected graph subcommand %q to be registered", name)
		}
	}
}

func TestGraphPodCommand_RequiresArg(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"graph", "pod"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no pod name argument provided")
	}
}

func TestGraphDeploymentCommand_RequiresArg(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"graph", "deployment"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no deployment name argument provided")
	}
}

func TestNamespaceFlag(t *testing.T) {
	flag := graphCmd.PersistentFlags().Lookup("namespace")
	if flag == nil {
		t.Fatal("expected 'namespace' flag on graph command")
	}
	if flag.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", flag.Shorthand)
	}
	if flag.DefValue != "default" {
		t.Errorf("expected default 'default', got %q", flag.DefValue)
	}
}

func TestContextFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("context")
	if flag == nil {
		t.Fatal("expected 'context' flag on root command")
	}
	if flag.DefValue != "" {
		t.Errorf("expected empty default, got %q", flag.DefValue)
	}
}

func TestCompletionCommand_Exists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'completion' subcommand to be registered")
	}
}
