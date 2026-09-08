//go:build !linux && !darwin

package server

import "fmt"

func installSystemPackage(path string) error {
	return fmt.Errorf("system package installation is unsupported on this platform: %s", path)
}

func scheduleSystemAppRestart(executable string) error {
	return fmt.Errorf("system package restart is unsupported on this platform: %s", executable)
}
