package server

import (
	"context"
	_ "embed"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

//go:embed updater-window.html
var updaterWindowHTML string

func (s *Server) updateInterval() time.Duration {
	cfg := s.Settings.Get()
	if !cfg.AutoCheckUpdates {
		return 0
	}
	return time.Duration(cfg.UpdateIntervalHours) * time.Hour
}

func configuredCNBProvider() (*cnbUpdaterProvider, error) {
	return newCNBUpdaterProvider(UpdateRepositoryURL, UpdateBranch, appConfig.BinaryName, nil)
}

func (s *Server) StartUpdateCfg() error {
	provider, err := configuredCNBProvider()
	if err != nil {
		return err
	}
	if err := s.App.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{provider},
		// The application owns the polling loop so Linux /usr/bin installs can
		// use the same CNB runtime artifact with one privilege-escalated swap.
		CheckInterval: 0,
		Window: &updater.BuiltinWindow{
			HTML: strings.ReplaceAll(updaterWindowHTML, "{{APP_NAME}}", html.EscapeString(AppName)),
			Options: updater.WindowOptions{
				Title:         "检查更新",
				Width:         520,
				Height:        340,
				AlwaysOnTop:   false,
				DisableResize: false,
			},
		},
	}); err != nil {
		return err
	}
	if interval := s.updateInterval(); interval > 0 {
		go s.automaticUpdateLoop(interval)
	}
	return nil
}

func (s *Server) automaticUpdateLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if s.quitting.Load() {
			return
		}
		if !s.updating.CompareAndSwap(false, true) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		err := s.performAutomaticUpdate(ctx)
		cancel()
		s.updating.Store(false)
		if err != nil && s.App != nil {
			s.App.Logger.Error("automatic update", "error", err)
		}
		if s.quitting.Load() {
			return
		}
	}
}

func (s *Server) performAutomaticUpdate(ctx context.Context) error {
	if needsPrivilegedRuntimeUpdate() {
		return s.performPrivilegedRuntimeUpdate(ctx)
	}

	release, err := s.App.Updater.Check(ctx)
	if err != nil || release == nil {
		return err
	}
	if err := s.App.Updater.DownloadAndInstall(ctx); err != nil {
		return err
	}
	if s.App.Updater.State() != updater.StateReady {
		return errors.New("update downloaded but updater is not ready to restart")
	}
	return s.App.Updater.Restart(ctx)
}

func (s *Server) CheckForUpdates() error {
	if s.App == nil {
		return errors.New("application is not ready")
	}
	if !s.updating.CompareAndSwap(false, true) {
		return errors.New("更新流程正在进行")
	}

	if needsPrivilegedRuntimeUpdate() {
		defer s.updating.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		return s.performPrivilegedRuntimeUpdate(ctx)
	}

	go func() {
		defer s.updating.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := s.App.Updater.CheckAndInstall(ctx); err != nil {
			s.App.Logger.Error("update", "error", err)
			return
		}
		if s.App.Updater.State() == updater.StateReady {
			if err := s.App.Updater.Restart(ctx); err != nil {
				s.App.Logger.Error("restart after update", "error", err)
			}
		}
	}()
	return nil
}

func updateInstallHint() string {
	return privilegedRuntimeUpdateHint()
}
