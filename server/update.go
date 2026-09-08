package server

import (
	"changeme/project"
	"context"
	_ "embed"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	githubupdater "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
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

func (s *Server) StartUpdateCfg() error {
	provider, err := githubupdater.New(githubupdater.Config{
		Repository:    UpdateRepository,
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher:  matchUpdateAsset,
	})
	if err != nil {
		return err
	}
	return s.App.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{provider},
		CheckInterval:  s.updateInterval(),
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
	})
}

func (s *Server) CheckForUpdates() error {
	if s.App == nil {
		return errors.New("application is not ready")
	}
	if !s.updating.CompareAndSwap(false, true) {
		return errors.New("更新流程正在进行，请查看更新窗口")
	}

	go func() {
		defer s.updating.Store(false)
		if err := s.App.Updater.CheckAndInstall(context.Background()); err != nil {
			s.App.Logger.Error("update", "error", err)
		}
	}()
	return nil
}

// Select only the published runtime artifact. Installers must never replace the app binary.
func matchUpdateAsset(req updater.CheckRequest, assets []githubupdater.ReleaseAsset) int {
	suffix := "-" + req.Platform + "-" + req.Arch
	switch req.Platform {
	case "windows":
		suffix += ".exe"
	case "darwin":
		suffix += ".zip"
	case "linux":
	default:
		return -1
	}
	prefix := appConfig.BinaryName + "-v"
	for i, asset := range assets {
		if !strings.HasPrefix(asset.Name, prefix) || !strings.HasSuffix(asset.Name, suffix) {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(asset.Name, prefix), suffix)
		if _, err := project.NumericVersion(version); err == nil {
			return i
		}
	}
	return -1
}
