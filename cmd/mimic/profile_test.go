package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestProfileCLIProcess(t *testing.T) {
	if os.Getenv("MIMIC_TEST_PROFILE_CLI") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("mimic", flag.ExitOnError)
	os.Args = []string{"mimic", "--profile", "unused.json", "--listen", "127.0.0.1:0"}
	main()
	os.Exit(0)
}

func TestProfileCLIRejectsRemovedConfiguration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProfileCLIProcess$")
	cmd.Env = append(os.Environ(), "MIMIC_TEST_PROFILE_CLI=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "flag provided but not defined: -profile") || strings.Contains(string(out), "Mimic listening") {
		t.Fatalf("removed option was accepted: %v; %s", err, out)
	}
}
