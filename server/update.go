package server

import (
	"context"
	_ "embed"
	"errors"
	"html"
	"strings"
	"time"

	desktopupdate "changeme/server/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

//go:embed updater-window.html
var updaterWindowHTML string

const (
	updateCompactWidth  = 348
	updateCompactHeight = 180
	updateFullWidth     = 520
	updateFullHeight    = 540
)

func (s *Server) updateInterval() time.Duration {
	return time.Duration(s.Settings.Get().UpdateIntervalHours) * time.Hour
}

func (s *Server) StartUpdateCfg() error {
	manager, err := desktopupdate.NewManager(desktopupdate.Config{
		RepositoryURL: UpdateRepositoryURL,
		Branch:        UpdateBranch,
		AppName:       appConfig.BinaryName,
	})
	if err != nil {
		return err
	}
	s.updateManager = manager

	// Keep Wails headless. The application opens the same updater HTML itself
	// so Check, DownloadAndInstall and Restart remain separate user decisions.
	if err := s.App.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{manager.Provider()},
		CheckInterval:  0,
		Window:         updater.WindowNone,
	}); err != nil {
		return err
	}

	s.bindUpdateWindowActions()
	go s.automaticUpdateLoop(s.updateInterval())
	return nil
}

func (s *Server) bindUpdateWindowActions() {
	s.App.Event.On(updater.EventWindowReady, func(*application.CustomEvent) {
		s.replayUpdateWindowState()
	})
	s.App.Event.On(updater.EventUserInstall, func(*application.CustomEvent) {
		go s.handleUpdateDownload()
	})
	s.App.Event.On(updater.EventUserRestart, func(*application.CustomEvent) {
		go s.handleUpdateRestart()
	})
	s.App.Event.On(updater.EventUserSkip, func(*application.CustomEvent) {
		s.skipPendingUpdate()
	})
	s.App.Event.On(updater.EventUserRemind, func(*application.CustomEvent) {
		s.closeUpdateWindow()
	})
	s.App.Event.On(updater.EventUserCancel, func(*application.CustomEvent) {
		s.closeUpdateWindow()
	})
}

func (s *Server) automaticUpdateLoop(interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if s.quitting.Load() {
			return
		}
		cfg := s.Settings.Get()
		if !cfg.AutoCheckUpdates || s.App.Updater.State() == updater.StateReady {
			continue
		}
		if !s.updating.CompareAndSwap(false, true) {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		err := s.performBackgroundUpdateCheck(ctx, cfg.AutoDownloadUpdates)
		cancel()
		s.updating.Store(false)
		if err != nil && s.App != nil {
			s.App.Logger.Error("automatic update", "error", err)
		}
	}
}

func (s *Server) performBackgroundUpdateCheck(ctx context.Context, autoDownload bool) error {
	release, err := s.App.Updater.Check(ctx)
	if err != nil {
		return err
	}
	s.setUpdateRelease(release)
	if release == nil || !autoDownload {
		return nil
	}
	return s.App.Updater.DownloadAndInstall(ctx)
}

// CheckForUpdates always opens the updater window. If a release is already
// Available or Ready from a background check, the existing state is shown
// immediately. Otherwise a fresh Check is performed without downloading.
func (s *Server) CheckForUpdates() error {
	if s.App == nil {
		return errors.New("application is not ready")
	}
	if s.updateManager == nil {
		return errors.New("update manager is not initialised")
	}

	state := s.App.Updater.State()
	if state == updater.StateAvailable || state == updater.StateDownloading || state == updater.StateVerifying || state == updater.StateInstalling || state == updater.StateReady {
		s.openUpdateWindow(false)
		s.replayUpdateWindowState()
		return nil
	}

	// A fresh check gets a fresh page so the template's monotonic state rank
	// starts from Checking instead of an older Up-to-Date/Error terminal state.
	s.openUpdateWindow(true)
	if !s.updating.CompareAndSwap(false, true) {
		s.replayUpdateWindowState()
		return nil
	}

	go func() {
		defer s.updating.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		release, err := s.App.Updater.Check(ctx)
		if err != nil {
			s.App.Logger.Error("update check", "error", err)
			s.resizeUpdateWindow(s.App.Updater.State())
			return
		}
		s.setUpdateRelease(release)
		s.resizeUpdateWindow(s.App.Updater.State())
		s.replayUpdateWindowState()
	}()
	return nil
}

func (s *Server) handleUpdateDownload() {
	if s.App == nil {
		return
	}
	state := s.App.Updater.State()
	if state == updater.StateError {
		s.closeUpdateWindow()
		_ = s.CheckForUpdates()
		return
	}
	if state != updater.StateAvailable {
		return
	}
	if !s.updating.CompareAndSwap(false, true) {
		return
	}
	defer s.updating.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := s.App.Updater.DownloadAndInstall(ctx); err != nil {
		s.App.Logger.Error("update download", "error", err)
		return
	}
	s.resizeUpdateWindow(s.App.Updater.State())
	s.replayUpdateWindowState()
}

func (s *Server) handleUpdateRestart() {
	if s.App == nil || s.updateManager == nil || s.App.Updater.State() != updater.StateReady {
		return
	}
	if !s.updating.CompareAndSwap(false, true) {
		return
	}
	defer s.updating.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var err error
	if s.updateManager.NeedsPrivilegedRuntimeUpdate() {
		err = s.updateManager.ApplyStagedRuntimeUpdate(ctx, s.App.Updater.DownloadedPath(), s.Quit)
	} else {
		err = s.App.Updater.Restart(ctx)
	}
	if err != nil {
		s.App.Logger.Error("restart after update", "error", err)
	}
}

func (s *Server) skipPendingUpdate() {
	s.updateMu.Lock()
	release := s.updateRelease
	s.updateRelease = nil
	s.updateMu.Unlock()
	if release != nil {
		s.App.Updater.SkipVersion(release.Version)
	}
	s.closeUpdateWindow()
}

func (s *Server) setUpdateRelease(release *updater.Release) {
	s.updateMu.Lock()
	s.updateRelease = release
	s.updateMu.Unlock()
}

func (s *Server) openUpdateWindow(reset bool) {
	if s.App == nil {
		return
	}
	if reset {
		s.closeUpdateWindow()
	}

	s.updateMu.Lock()
	window := s.updateWindow
	created := false
	if window == nil {
		window = s.App.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:                "软件更新",
			Width:                updateCompactWidth,
			Height:               updateCompactHeight,
			MinWidth:             updateCompactWidth,
			MinHeight:            updateCompactHeight,
			DisableResize:        false,
			HTML:                 strings.ReplaceAll(updaterWindowHTML, "{{APP_NAME}}", html.EscapeString(AppName)),
			AllowSimpleEventEmit: true,
		})
		s.updateWindow = window
		created = true
	}
	s.updateMu.Unlock()

	if created {
		window.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
			s.updateMu.Lock()
			if s.updateWindow == window {
				s.updateWindow = nil
			}
			s.updateMu.Unlock()
		})
	}
	window.Center()
	window.Show()
	window.Focus()
}

func (s *Server) closeUpdateWindow() {
	s.updateMu.Lock()
	window := s.updateWindow
	s.updateWindow = nil
	s.updateMu.Unlock()
	if window != nil {
		window.Close()
	}
}

func (s *Server) replayUpdateWindowState() {
	if s.App == nil {
		return
	}
	s.updateMu.Lock()
	window := s.updateWindow
	release := s.updateRelease
	s.updateMu.Unlock()
	if window == nil {
		return
	}

	window.EmitEvent(updater.EventMeta, updater.Meta{
		CurrentVersion: Version,
		SkippedVersion: s.App.Updater.SkippedVersion(),
	})
	state := s.App.Updater.State()
	s.resizeUpdateWindow(state)
	switch state {
	case updater.StateChecking:
		window.EmitEvent(updater.EventCheckStarted)
	case updater.StateAvailable:
		if release != nil {
			window.EmitEvent(updater.EventUpdateAvailable, release)
		}
	case updater.StateDownloading:
		if release != nil {
			window.EmitEvent(updater.EventDownloadStarted, release)
		}
	case updater.StateVerifying:
		window.EmitEvent(updater.EventVerifying, release)
	case updater.StateInstalling:
		window.EmitEvent(updater.EventInstalling, release)
	case updater.StateReady:
		window.EmitEvent(updater.EventUpdateReady, release)
	case updater.StateUpToDate:
		window.EmitEvent(updater.EventNoUpdate)
	}
}

func (s *Server) resizeUpdateWindow(state updater.State) {
	s.updateMu.Lock()
	window := s.updateWindow
	s.updateMu.Unlock()
	if window == nil {
		return
	}
	switch state {
	case updater.StateAvailable, updater.StateDownloading, updater.StateVerifying, updater.StateInstalling, updater.StateReady:
		window.SetSize(updateFullWidth, updateFullHeight)
	default:
		window.SetSize(updateCompactWidth, updateCompactHeight)
	}
	window.Center()
}

func (s *Server) updateInstallHint() string {
	if s.updateManager == nil {
		return ""
	}
	return s.updateManager.InstallHint()
}
