//go:build !linux

package server

import (
	"context"
	"errors"
)

func needsPrivilegedRuntimeUpdate() bool { return false }

func privilegedRuntimeUpdateHint() string { return "" }

func (s *Server) performPrivilegedRuntimeUpdate(context.Context) error {
	return errors.New("privileged runtime update is unsupported on this platform")
}
