//go:build linux

package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/updater"
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
	return "DEB 仅用于首次安装；后续直接更新程序二进制，替换 /usr/bin 时会请求一次系统授权并自动重启。"
}

func performPrivilegedRuntimeUpdate(ctx context.Context, provider *cnbProvider, currentVersion, appName string, quit func()) error {
	release, err := provider.Check(ctx, updater.CheckRequest{
		CurrentVersion: currentVersion,
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
	})
	if err != nil || release == nil {
		return err
	}
	if release.Verification == nil || release.Verification.DigestAlgo != "sha256" || len(release.Verification.Digest) != sha256.Size {
		return fmt.Errorf("cnb: release %s is missing SHA-256 verification", release.Version)
	}

	stagingDir, err := os.MkdirTemp("", appName+"-update-*")
	if err != nil {
		return fmt.Errorf("create update staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	staged := filepath.Join(stagingDir, release.Artifact.Filename)
	file, err := os.OpenFile(staged, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("create staged runtime: %w", err)
	}
	hash := sha256.New()
	downloadErr := provider.Download(ctx, release, io.MultiWriter(file, hash), nil)
	syncErr := file.Sync()
	closeErr := file.Close()
	if downloadErr != nil {
		return downloadErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !bytes.Equal(hash.Sum(nil), release.Verification.Digest) {
		return fmt.Errorf("cnb: SHA-256 verification failed for %s", release.Artifact.Filename)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return fmt.Errorf("chmod staged runtime: %w", err)
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

	// Write beside the target first, then atomically rename over the running
	// executable. Linux keeps the old inode alive for the current process.
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

	args := []string{"/bin/sh", "-c", script, "update-runtime", staged, target, installTool, mvTool}
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
