package desktop

import (
	"changeme/server/localapi"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type DesktopState struct {
	Name                     string          `json:"name"`
	Version                  string          `json:"version"`
	Commit                   string          `json:"commit"`
	BuildTime                string          `json:"buildTime"`
	Platform                 string          `json:"platform"`
	Architecture             string          `json:"architecture"`
	AutoStart                bool            `json:"autoStart"`
	AutoStartShowWindow      bool            `json:"autoStartShowWindow"`
	AutoStartWindowSupported bool            `json:"autoStartWindowSupported"`
	ConfigPath               string          `json:"configPath"`
	ConfigError              string          `json:"configError,omitempty"`
	AutoStartError           string          `json:"autoStartError,omitempty"`
	CloseToTray              bool            `json:"closeToTray"`
	TrayReady                bool            `json:"trayReady"`
	AutoCheckUpdates         bool            `json:"autoCheckUpdates"`
	AutoDownloadUpdates      bool            `json:"autoDownloadUpdates"`
	UpdateRepositoryURL      string          `json:"updateRepositoryURL"`
	UpdateInstallHint        string          `json:"updateInstallHint,omitempty"`
	UpdateIntervalHours      int             `json:"updateIntervalHours"`
	LocalServer              localapi.Status `json:"localServer"`
	WebSocketURL             string          `json:"websocketURL"`
}

type DesktopService struct {
	mu     sync.Mutex
	server *Server
}

func NewDesktopService(server *Server) *DesktopService {
	return &DesktopService{server: server}
}

func (s *DesktopService) State() (DesktopState, error) {
	if s.server == nil || s.server.App == nil {
		return DesktopState{}, errors.New("desktop runtime is not ready")
	}
	autoStart, autoStartErr := s.server.App.Autostart.Status()
	settings := s.server.Settings.Get()
	state := DesktopState{
		Name:                     AppName,
		Version:                  Version,
		Commit:                   Commit,
		BuildTime:                BuildTime,
		Platform:                 runtime.GOOS,
		Architecture:             runtime.GOARCH,
		AutoStart:                autoStart.Enabled,
		AutoStartShowWindow:      settings.AutoStartShowWindow,
		AutoStartWindowSupported: autoStart.Strategy != application.AutostartStrategySMAppService,
		ConfigPath:               s.server.Settings.Path(),
		ConfigError:              s.server.Settings.Error(),
		CloseToTray:              settings.CloseToTray,
		TrayReady:                s.server.Tray != nil,
		AutoCheckUpdates:         settings.AutoCheckUpdates,
		AutoDownloadUpdates:      settings.AutoDownloadUpdates,
		UpdateRepositoryURL:      UpdateRepositoryURL,
		UpdateInstallHint:        s.server.updateInstallHint(),
		UpdateIntervalHours:      settings.UpdateIntervalHours,
		LocalServer:              s.server.LocalServer.Status(),
		WebSocketURL:             s.server.LocalServer.WebSocketURL(),
	}
	if autoStartErr != nil {
		state.AutoStartError = autoStartErr.Error()
	}
	return state, nil
}

func (s *DesktopService) Echo(message string) string {
	return fmt.Sprintf("Go 服务已收到：%s", message)
}

func (s *DesktopService) SetAutoStart(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil || s.server.App == nil {
		return errors.New("desktop runtime is not ready")
	}
	if !enabled {
		return s.server.App.Autostart.Disable()
	}
	return s.server.App.Autostart.EnableWithOptions(applicationAutostartOptions(s.server.Settings.Get().AutoStartShowWindow))
}

func (s *DesktopService) SetCloseToTray(enabled bool) error {
	return s.server.Settings.SetCloseToTray(enabled)
}

func (s *DesktopService) SetAutoCheckUpdates(enabled bool) error {
	return s.server.Settings.SetAutoCheckUpdates(enabled)
}

func (s *DesktopService) SetAutoDownloadUpdates(enabled bool) error {
	return s.server.Settings.SetAutoDownloadUpdates(enabled)
}

func (s *DesktopService) CheckForUpdates() error {
	return s.server.CheckForUpdates()
}

// SetAutoStartShowWindow updates both the saved preference and an existing registration.
func (s *DesktopService) SetAutoStartShowWindow(show bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil || s.server.App == nil {
		return errors.New("desktop runtime is not ready")
	}
	status, err := s.server.App.Autostart.Status()
	if err != nil {
		return err
	}
	if status.Strategy == application.AutostartStrategySMAppService {
		return errors.New("当前 macOS 自启动方式不支持窗口启动参数")
	}
	old := s.server.Settings.Get().AutoStartShowWindow
	if err := s.server.Settings.SetAutoStartShowWindow(show); err != nil {
		return err
	}
	if status.Enabled {
		if err := s.server.App.Autostart.EnableWithOptions(applicationAutostartOptions(show)); err != nil {
			return errors.Join(err, s.server.Settings.SetAutoStartShowWindow(old))
		}
	}
	return nil
}
