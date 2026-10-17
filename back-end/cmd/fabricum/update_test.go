package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fabricum/back-end/internal/updater"
)

func TestUpdateExecutableDispatch(t *testing.T) {
	name := "fabricum"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	if data, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", data, err)
	}
	data, err := exec.Command(binary, "update", "--help").CombinedOutput()
	if err != nil || !strings.Contains(string(data), "fabricum update") {
		t.Fatalf("help: %s %v", data, err)
	}
	command := exec.Command(binary, "update", "--timeout", "0")
	data, err = command.Output()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("expected exit 1: %s %v", data, err)
	}
	var result updater.Result
	if err := json.Unmarshal(data, &result); err != nil || result.Status != "failed" || result.Updated || result.Error == "" {
		t.Fatalf("failure receipt: %s %v", data, err)
	}
}
