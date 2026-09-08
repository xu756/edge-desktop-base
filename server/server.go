package server

import (
	"embed"
	"errors"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Server struct {
	App            *application.App
	MainWindow     *application.WebviewWindow
	Tray           *application.SystemTray
	Settings       *SettingsStore
	LocalServer    *LocalServer
	DesktopService *DesktopService

	quitting atomic.Bool
	updating atomic.Bool
}

func New() *Server {
	return &Server{}
}

func (s *Server) Init(assets embed.FS, icon []byte) error {
	s.Settings = NewSettingsStore()
	s.LocalServer = NewLocalServer(DefaultAPIAddress)
	s.DesktopService = NewDesktopService(s)

	s.App = application.New(application.Options{
		Name:        AppName,
		Description: AppDescription,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(s.LocalServer),
			application.NewService(s.DesktopService),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: AppIdentifier,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				s.ShowMainWindow()
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	if err := s.StartUpdateCfg(); err != nil {
		return err
	}

	s.NewMainWindow()
	s.SetTray(icon)
	return nil
}

func (s *Server) Start() error {
	if s.App == nil {
		return errors.New("application is not initialised")
	}
	return s.App.Run()
}

func (s *Server) Quit() {
	s.quitting.Store(true)
	if s.App != nil {
		s.App.Quit()
	}
}
