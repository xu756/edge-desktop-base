//go:build !linux

package update

import (
	"context"
	"errors"
)

func needsPrivilegedRuntimeUpdate(string) bool { return false }

func privilegedRuntimeUpdateHint(string) string { return "" }

func applyPrivilegedRuntimeUpdate(context.Context, string, string, func()) error {
	return errors.New("update: privileged runtime update is unsupported on this platform")
}
