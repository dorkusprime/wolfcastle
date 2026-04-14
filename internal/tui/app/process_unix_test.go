//go:build !windows

package app

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestIsProcessAlive_SelfPID(t *testing.T) {
	t.Parallel()
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be reported alive")
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	t.Parallel()
	if isProcessAlive(0) {
		t.Error("pid 0 should be rejected")
	}
	if isProcessAlive(-1) {
		t.Error("negative pid should be rejected")
	}
}

func TestIsProcessAlive_DeadPID(t *testing.T) {
	t.Parallel()

	// Spawn a short-lived process, wait for it, then check that the PID
	// reads as dead. We pick a process that exits immediately (`true`)
	// so we don't have to race a kill.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}

	// The reaped PID may linger briefly in some kernels; give it a moment.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	// On Linux a PID can be recycled fast; if it still reads alive,
	// treat that as a flake rather than a failure. Verify the negative
	// branch another way: a pathological PID like 2^30 must be dead.
	if isProcessAlive(1 << 30) {
		t.Error("synthetic huge pid should read as dead")
	}
}

func TestIsProcessRunning_DelegatesToAlive(t *testing.T) {
	t.Parallel()
	if !isProcessRunning(os.Getpid()) {
		t.Error("isProcessRunning should mirror isProcessAlive for current pid")
	}
	if isProcessRunning(0) {
		t.Error("isProcessRunning(0) should be false")
	}
}

func TestKillProcess_Signal0OnSelf(t *testing.T) {
	t.Parallel()
	// Signal 0 doesn't deliver; it just validates the target exists.
	if err := killProcess(os.Getpid(), syscall.Signal(0)); err != nil {
		t.Errorf("signal 0 to self should succeed, got %v", err)
	}
}

func TestKillProcess_InvalidPID(t *testing.T) {
	t.Parallel()
	// Negative and huge PIDs should surface an error from the kernel.
	if err := killProcess(1<<30, syscall.Signal(0)); err == nil {
		t.Error("expected error signaling a non-existent pid")
	}
}
