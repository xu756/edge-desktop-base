//go:build linux

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func needsPrivilegedRuntimeUpdate(appName string) bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	return filepath.Clean(executable) == filepath.Join("/usr/bin", appName)
}

func privilegedRuntimeUpdateHint(appName string) string {
	if !needsPrivilegedRuntimeUpdate(appName) {
		return ""
	}
	return "DEB 仅用于首次安装；更新会先下载并校验，点击“重启并应用”时才请求系统授权替换 /usr/bin 中的程序。"
}

func applyPrivilegedRuntimeUpdate(ctx context.Context, stagedPath, appName string, quit func()) error {
	info, err := os.Stat(stagedPath)
	if err != nil {
		return fmt.Errorf("stat staged runtime: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("staged runtime is not a regular file: %s", stagedPath)
	}

	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	target = filepath.Clean(target)
	if target != filepath.Join("/usr/bin", appName) {
		return fmt.Errorf("refusing privileged runtime swap for unexpected target %q", target)
	}

	installTool, err := exec.LookPath("install")
	if err != nil {
		return fmt.Errorf("install command not found: %w", err)
	}
	mvTool, err := exec.LookPath("mv")
	if err != nil {
		return fmt.Errorf("mv command not found: %w", err)
	}

	// Wails already downloaded and verified stagedPath. Copy it beside the
	// target and atomically rename it over the running executable. Linux keeps
	// the old inode alive until this process exits, so the restart sees the new
	// runtime without writing into the currently mapped binary.
	script := "set -eu\n" +
		"src=\"$1\"\n" +
		"dst=\"$2\"\n" +
		"install_tool=\"$3\"\n" +
		"mv_tool=\"$4\"\n" +
		"tmp=\"${dst}.update.$$\"\n" +
		"trap 'rm -f \"$tmp\"' EXIT\n" +
		"\"$install_tool\" -m 0755 \"$src\" \"$tmp\"\n" +
		"\"$mv_tool\" -f \"$tmp\" \"$dst\"\n" +
		"trap - EXIT"

	args := []string{"/bin/sh", "-c", script, "update-runtime", stagedPath, target, installTool, mvTool}
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	} else {
		pkexec, err := exec.LookPath("pkexec")
		if err != nil {
			return fmt.Errorf("pkexec not found; PolicyKit is required to update %s: %w", target, err)
		}
		cmd = exec.CommandContext(ctx, pkexec, args...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("replace runtime binary: %w", err)
	}

	// Clean Wails' staging directory after the privileged copy. Restrict the
	// cleanup to the updater's own temp-directory naming convention.
	stagingDir := filepath.Dir(stagedPath)
	if strings.HasPrefix(filepath.Base(stagingDir), "wails-update-") {
		_ = os.RemoveAll(stagingDir)
	}

	if err := scheduleRuntimeRestart(target); err != nil {
		return err
	}
	if quit != nil {
		quit()
	}
	return nil
}

func scheduleRuntimeRestart(executable string) error {
	pid := strconv.Itoa(os.Getpid())
	script := "pid=\"$1\"; app=\"$2\"; while kill -0 \"$pid\" 2>/dev/null; do sleep 0.2; done; exec \"$app\" >/dev/null 2>&1"
	cmd := exec.Command("/bin/sh", "-c", script, "restart-app", pid, executable)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schedule app restart: %w", err)
	}
	return nil
}
