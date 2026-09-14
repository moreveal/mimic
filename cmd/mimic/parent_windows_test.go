//go:build windows

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestProcessExitSignalUsesParentHandle(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "exit", "0")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := processExitSignal(uint32(command.Process.Pid))
	if done == nil {
		t.Fatal("failed to monitor child process")
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("process exit was not observed")
	}
}

func TestProcessExitSignalReportsMissingProcess(t *testing.T) {
	done := processExitSignal(^uint32(0))
	if done == nil {
		t.Fatal("missing process was not recognized")
	}
	select {
	case <-done:
	default:
		t.Fatal("missing process did not report exit immediately")
	}
}
