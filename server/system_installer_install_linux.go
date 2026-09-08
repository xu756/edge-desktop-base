//go:build linux

package server

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func installSystemPackage(path string) error {
	apt, err := exec.LookPath("apt-get")
	if err != nil {
		return fmt.Errorf("apt-get not found: %w", err)
	}
	if os.Geteuid() == 0 {
		cmd := exec.Command(apt, "install", "-y", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("pkexec not found; install PolicyKit to allow in-app DEB upgrades: %w", err)
	}
	cmd := exec.Command(pkexec, apt, "install", "-y", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("DEB installation failed: %w", err)
	}
	return nil
}

func scheduleSystemAppRestart(executable string) error {
	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command(
		"/bin/sh", "-c",
		`pid="$1"; app="$2"; while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done; exec "$app" >/dev/null 2>&1`,
		"restart-app", pid, executable,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schedule app restart: %w", err)
	}
	return nil
}
