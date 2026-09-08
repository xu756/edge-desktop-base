package desktop

import (
	"changeme/project"
	"changeme/server/localapi"
	"changeme/server/settings"
	"embed"
	"errors"
	"sync"
	"sync/atomic"

	desktopupdate "changeme/server/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

type Server struct {
	App            *application.App
	MainWindow     *application.WebviewWindow
	Tray           *application.SystemTray
	Settings       *settings.Store
	LocalServer    *localapi.Server
	DesktopService *DesktopService

	updateManager *desktopupdate.Manager
	updateWindow  *application.WebviewWindow
	updateRelease *updater.Release
	updateMu      sync.Mutex
	quitting      atomic.Bool
	updating      atomic.Bool
}

func New() *Server {
	return &Server{}
}

func (s *Server) Init(assets embed.FS, icon []byte) error {
	s.Settings = settings.New(appConfig.ConfigDirName, appConfig.LegacyConfigDirNames)
	s.LocalServer = localapi.New(DefaultAPIAddress, Version)
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

var appConfig = project.Load()

var (
	AppName             = appConfig.Name
	AppDescription      = appConfig.Description
	AppIdentifier       = appConfig.Identifier
	UpdateRepositoryURL = appConfig.UpdateRepositoryURL
	UpdateBranch        = appConfig.UpdateBranch
	DefaultAPIAddress   = appConfig.DefaultAPIAddress
)
