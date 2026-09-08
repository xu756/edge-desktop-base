package server

import (
	"context"
	"errors"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	githubupdater "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const updateInterval = 6 * time.Hour

func (s *Server) StartUpdateCfg() error {
	provider, err := githubupdater.New(githubupdater.Config{
		Repository:    UpdateRepository,
		ChecksumAsset: "SHA256SUMS",
	})
	if err != nil {
		return err
	}
	return s.App.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{provider},
		CheckInterval:  updateInterval,
		Window: &updater.BuiltinWindow{
			CSS: `:root { --accent: #ff6f00; --radius: 16px; }`,
			Options: updater.WindowOptions{
				Title:         "检查更新",
				Width:         400,
				Height:        200,
				AlwaysOnTop:   true,
				DisableResize: true,
			},
		},
	})
}

func (s *Server) CheckForUpdates() error {
	if s.App == nil {
		return errors.New("application is not ready")
	}
	if !s.updating.CompareAndSwap(false, true) {
		return nil
	}

	go func() {
		defer s.updating.Store(false)
		if err := s.App.Updater.CheckAndInstall(context.Background()); err != nil {
			s.App.Logger.Error("update", "error", err)
		}
	}()
	return nil
}
