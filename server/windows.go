package server

import "github.com/wailsapp/wails/v3/pkg/application"

func (s *Server) NewWindow(windowOptions application.WebviewWindowOptions) {
	s.App.Window.NewWithOptions(windowOptions)

}
