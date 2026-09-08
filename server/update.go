package server

import (
	"context"
	_ "embed"
	"errors"
	"html"
	"os"
	"path/filepath"
	"runtime"
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
		// Periodic updates are orchestrated by this application so both portable
		// binaries and system-installed DEB/PKG builds follow the same automatic
		// install + restart policy.
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
	if updateInstallHint() != "" {
		return s.installLatestSystemPackage(ctx)
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

func (s *Server) installLatestSystemPackage(ctx context.Context) error {
	provider, err := configuredCNBProvider()
	if err != nil {
		return err
	}
	update, err := provider.checkInstallerUpdate(ctx, Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if update == nil {
		return nil
	}

	path, err := provider.downloadInstaller(ctx, update)
	if err != nil {
		return err
	}
	removeInstaller := true
	defer func() {
		if removeInstaller {
			_ = os.Remove(path)
		}
	}()

	if err := installSystemPackage(path); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err := scheduleSystemAppRestart(executable); err != nil {
		return err
	}
	_ = os.Remove(path)
	removeInstaller = false
	s.Quit()
	return nil
}

func (s *Server) CheckForUpdates() error {
	if s.App == nil {
		return errors.New("application is not ready")
	}
	if !s.updating.CompareAndSwap(false, true) {
		return errors.New("更新流程正在进行")
	}

	if updateInstallHint() != "" {
		defer s.updating.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		return s.installLatestSystemPackage(ctx)
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
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return systemInstallHint(runtime.GOOS, executable, appConfig.BinaryName)
}

func systemInstallHint(platform, executable, binaryName string) string {
	executable = filepath.ToSlash(executable)
	if platform == "linux" && executable == "/usr/bin/"+binaryName {
		return "DEB 安装版；新版本会自动下载，系统授权后完成安装并自动重启。"
	}
	if platform == "darwin" && strings.HasPrefix(executable, "/Applications/") && strings.Contains(executable, ".app/Contents/MacOS/") {
		return "PKG 安装版；新版本会自动下载，系统授权后完成安装并自动重启。"
	}
	return ""
}
