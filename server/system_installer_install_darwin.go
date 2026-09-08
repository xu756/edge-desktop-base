//go:build darwin

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func installSystemPackage(path string) error {
	script := `on run argv
set pkgPath to item 1 of argv
do shell script "/usr/sbin/installer -pkg " & quoted form of pkgPath & " -target /" with administrator privileges
end run`
	cmd := exec.Command("/usr/bin/osascript", "-e", script, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("PKG installation failed: %w", err)
	}
	return nil
}

func scheduleSystemAppRestart(executable string) error {
	executable = filepath.Clean(executable)
	marker := string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS" + string(filepath.Separator)
	index := strings.Index(executable, marker)
	if index < 0 {
		return fmt.Errorf("cannot resolve .app bundle from executable %q", executable)
	}
	bundle := executable[:index]
	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command(
		"/bin/sh", "-c",
		`pid="$1"; app="$2"; while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done; exec /usr/bin/open "$app" >/dev/null 2>&1`,
		"restart-app", pid, bundle,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schedule app restart: %w", err)
	}
	return nil
}
