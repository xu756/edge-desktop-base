package server

import (
	"changeme/server/service"
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Server struct {
	App     *application.App
	Version string
}

func New(version string) *Server {
	return &Server{
		Version: version,
	}
}

func (s *Server) Init(assets embed.FS) error {

	app := application.New(application.Options{
		Name:        "edgeinfer-node-test",
		Description: "A demo of using raw HTML & CSS",
		Services: []application.Service{
			application.NewService(&service.Service{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	s.App = app
	// 设置菜单
	s.SetMenu()

	// 启动一个窗口
	s.NewWindow(application.WebviewWindowOptions{
		Title: "推理服务器测试程序",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	// 启动更新程序
	return s.StartUpdateCfg()
}

func (s *Server) Start() error {
	return s.App.Run()
}
