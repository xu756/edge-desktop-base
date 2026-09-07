package server

import (
	"errors"
	"fmt"
	"runtime"
)

type DesktopState struct {
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	Commit              string            `json:"commit"`
	BuildTime           string            `json:"buildTime"`
	Platform            string            `json:"platform"`
	Architecture        string            `json:"architecture"`
	AutoStart           bool              `json:"autoStart"`
	AutoStartError      string            `json:"autoStartError,omitempty"`
	CloseToTray         bool              `json:"closeToTray"`
	TrayReady           bool              `json:"trayReady"`
	UpdateRepository    string            `json:"updateRepository"`
	UpdateIntervalHours int               `json:"updateIntervalHours"`
	LocalServer         LocalServerStatus `json:"localServer"`
	WebSocketURL        string            `json:"websocketURL"`
}

type DesktopService struct {
	server *Server
}

func NewDesktopService(server *Server) *DesktopService {
	return &DesktopService{server: server}
}

func (s *DesktopService) State() (DesktopState, error) {
	if s.server == nil || s.server.App == nil {
		return DesktopState{}, errors.New("desktop runtime is not ready")
	}
	autoStart, autoStartErr := s.server.App.Autostart.IsEnabled()
	state := DesktopState{
		Name:                AppName,
		Version:             Version,
		Commit:              Commit,
		BuildTime:           BuildTime,
		Platform:            runtime.GOOS,
		Architecture:        runtime.GOARCH,
		AutoStart:           autoStart,
		CloseToTray:         s.server.Settings.Get().CloseToTray,
		TrayReady:           s.server.Tray != nil,
		UpdateRepository:    UpdateRepository,
		UpdateIntervalHours: int(updateInterval.Hours()),
		LocalServer:         s.server.LocalServer.Status(),
		WebSocketURL:        s.server.LocalServer.WebSocketURL(),
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
	if s.server == nil || s.server.App == nil {
		return errors.New("desktop runtime is not ready")
	}
	if !enabled {
		return s.server.App.Autostart.Disable()
	}
	return s.server.App.Autostart.EnableWithOptions(applicationAutostartOptions())
}

func (s *DesktopService) SetCloseToTray(enabled bool) error {
	return s.server.Settings.SetCloseToTray(enabled)
}

func (s *DesktopService) CheckForUpdates() error {
	return s.server.CheckForUpdates()
}
