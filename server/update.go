package server

import (
	"changeme/project"
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
	githubupdater "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

//go:embed updater-window.html
var updaterWindowHTML string

func (s *Server) updateInterval() time.Duration {
	cfg := s.Settings.Get()
	if !cfg.AutoCheckUpdates || updateInstallHint() != "" {
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
	if updateInstallHint() != "" {
		return s.App.Browser.OpenURL("https://github.com/" + UpdateRepository + "/releases")
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

func updateInstallHint() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return systemInstallHint(runtime.GOOS, executable, appConfig.BinaryName)
}

// System installers own these paths. Upgrade through the installer so package
// receipts, permissions and application contents stay consistent.
func systemInstallHint(platform, executable, binaryName string) string {
	executable = filepath.ToSlash(executable)
	if platform == "linux" && executable == "/usr/bin/"+binaryName {
		return "通过 DEB 安装的版本，请下载新版 .deb 安装升级。"
	}
	if platform == "darwin" && strings.HasPrefix(executable, "/Applications/") && strings.Contains(executable, ".app/Contents/MacOS/") {
		return "应用程序目录中的版本，请下载新版 .pkg 安装升级。"
	}
	return ""
}
